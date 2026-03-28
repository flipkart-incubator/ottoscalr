package reco

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	rolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	"github.com/flipkart-incubator/ottoscalr/pkg/metrics"
	"github.com/go-logr/logr"
	kedaapi "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	p8smetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	getAverageCPUUtilizationQueryLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "get_avg_cpu_utilization_query_latency_seconds",
			Help:    "Time to execute utilization datapoint query in seconds",
			Buckets: append(prometheus.DefBuckets, 15, 20, 50, 100),
		}, []string{"namespace", "policyreco", "workloadKind", "workload"},
	)

	minPercentageOfDataPointsPresent = promauto.NewGaugeVec(
		prometheus.GaugeOpts{Name: "minimum_percentage_of_datapoints_present",
			Help: "Boolean to show if min percentage of datapoints is present to generate recommendation"},
		[]string{"namespace", "workload"},
	)

	potentialSavingsPercentage = promauto.NewGaugeVec(
		prometheus.GaugeOpts{Name: "workload_potential_savings_percentage",
			Help: "Potential savings percentage achieved by applying the recommendation"},
		[]string{"namespace", "workload"},
	)
)

const (
	HPAScaleTolerationFactor = 1.1
)

func init() {
	p8smetrics.Registry.MustRegister(getAverageCPUUtilizationQueryLatency, minPercentageOfDataPointsPresent, potentialSavingsPercentage)
}

var unableToRecommendError = errors.New("Unable to generate recommendation without any breaches.")

const (
	ScaledObjectField         = "spec.scaleTargetRef.name"
	OttoscalrMaxPodAnnotation = "ottoscalr.io/max-pods"
	OttoscalrMinPodAnnotation = "ottoscalr.io/min-pods"
)

type timestampedScaleEvent struct {
	timestamp     time.Time
	replicaChange float64
}

type replicaRecommendation struct {
	Timestamp time.Time
	Replicas  int
}

type CpuUtilizationBasedRecommender struct {
	k8sClient                       client.Client
	redLineUtil                     float64
	metricWindow                    time.Duration
	scraper                         metrics.Scraper
	metricsTransformer              []metrics.MetricsTransformer
	metricStep                      time.Duration
	minTarget                       int
	maxTarget                       int
	metricsPercentageThreshold      int
	downScaleStabilizationWindow    time.Duration
	defaultScaleUpPeriodSeconds     int
	defaultMaxScaleUpPodsPercentage int
	enableScaleUpBehaviourForAll    *bool
	enableTimeWindows               bool
	numberOfWindows                 int
	logger                          logr.Logger
}

func NewCpuUtilizationBasedRecommender(k8sClient client.Client,
	redLineUtil float64,
	metricWindow time.Duration,
	scraper metrics.Scraper,
	metricsTransformer []metrics.MetricsTransformer,
	metricStep time.Duration,
	minTarget int,
	maxTarget int,
	metricsPercentageThreshold int, downScaleStabilizationWindow time.Duration,
	defaultScaleUpPeriodSeconds int, defaultMaxScaleUpPodsPercentage int,
	enableScaleUpBehaviourForAll *bool,
	enableTimeWindows bool,
	numberOfWindows int,
	logger logr.Logger) *CpuUtilizationBasedRecommender {
	return &CpuUtilizationBasedRecommender{
		k8sClient:                       k8sClient,
		redLineUtil:                     redLineUtil,
		metricWindow:                    metricWindow,
		scraper:                         scraper,
		metricsTransformer:              metricsTransformer,
		metricStep:                      metricStep,
		minTarget:                       minTarget,
		maxTarget:                       maxTarget,
		metricsPercentageThreshold:      metricsPercentageThreshold,
		downScaleStabilizationWindow:    downScaleStabilizationWindow,
		defaultScaleUpPeriodSeconds:     defaultScaleUpPeriodSeconds,
		defaultMaxScaleUpPodsPercentage: defaultMaxScaleUpPodsPercentage,
		enableScaleUpBehaviourForAll:    enableScaleUpBehaviourForAll,
		enableTimeWindows:               enableTimeWindows,
		numberOfWindows:                 numberOfWindows,
		logger:                          logger,
	}
}

// WindowedRecommender extends Recommender with time-window awareness.
type WindowedRecommender interface {
	Recommender
	RecommendWindowed(ctx context.Context, wm WorkloadMeta) (*v1alpha1.HPAConfiguration, []v1alpha1.TimeWindowedHPAConfiguration, error)
}

// Recommend implements the Recommender interface. It delegates to RecommendWindowed,
// which handles both feature-on and feature-off cases.
func (c *CpuUtilizationBasedRecommender) Recommend(ctx context.Context, workloadMeta WorkloadMeta) (*v1alpha1.HPAConfiguration, error) {
	cfg, _, err := c.RecommendWindowed(ctx, workloadMeta)
	return cfg, err
}

// recommendGlobal runs the original (non-windowed) recommendation algorithm across
// all 21-28 days of data points uniformly and returns a single HPAConfiguration.
func (c *CpuUtilizationBasedRecommender) recommendGlobal(ctx context.Context, workloadMeta WorkloadMeta) (*v1alpha1.HPAConfiguration, error) {
	end := time.Now()
	start := end.Add(-c.metricWindow)

	utilizationQueryStartTime := time.Now()
	dataPoints, err := c.scraper.GetAverageCPUUtilizationByWorkload(workloadMeta.Namespace,
		workloadMeta.Name,
		workloadMeta.Kind,
		start,
		end,
		c.metricStep)
	if err != nil {
		c.logger.Error(err, "Error while scraping GetAverageCPUUtilizationByWorkload.")
		return nil, err
	}
	cpuUtilizationQueryLatency := time.Since(utilizationQueryStartTime).Seconds()
	getAverageCPUUtilizationQueryLatency.WithLabelValues(workloadMeta.Namespace, workloadMeta.Name, workloadMeta.Kind, workloadMeta.Name).Observe(cpuUtilizationQueryLatency)

	workloadMaxReplicas, err := c.getMaxPods(workloadMeta.Namespace, workloadMeta.Kind, workloadMeta.Name)
	if err != nil {
		c.logger.Error(err, "Error while getting getMaxPods")
		return nil, err
	}

	workloadMinReplicas, err := getMinPods(c.k8sClient, workloadMeta.Namespace, workloadMeta.Kind, workloadMeta.Name)
	if err != nil {
		c.logger.Error(err, "Error while getting getMinPods")
		return nil, err
	}

	if !c.isMetricsAboveThreshold(dataPoints) {
		minPercentageOfDataPointsPresent.WithLabelValues(workloadMeta.Namespace, workloadMeta.Name).Set(float64(0))
		err = fmt.Errorf("metric Source doesn't has required number of metrics to generate recommendation")
		c.logger.Error(err, "Setting the recommendation to no operation policy")
		return &v1alpha1.HPAConfiguration{Min: workloadMaxReplicas, Max: workloadMaxReplicas, TargetMetricValue: c.minTarget}, nil
	}
	minPercentageOfDataPointsPresent.WithLabelValues(workloadMeta.Namespace, workloadMeta.Name).Set(float64(1))

	if c.metricsTransformer != nil {
		for _, transformers := range c.metricsTransformer {
			dataPoints, err = transformers.Transform(start, end, dataPoints)
			if err != nil {
				c.logger.Error(err, "Error while getting outlier interval from event api")
				return nil, err
			}
		}
	}

	acl, err := c.scraper.GetACLByWorkload(workloadMeta.Namespace, workloadMeta.Name, workloadMeta.Kind)
	if err != nil {
		c.logger.Error(err, "Error while getting GetACL.")
		return nil, err
	}

	perPodResources, err := c.getContainerCPULimitsSum(workloadMeta.Namespace, workloadMeta.Kind, workloadMeta.Name)
	if err != nil {
		c.logger.Error(err, "Error while getting getContainerCPULimitsSum")
		return nil, err
	}
	enableScaleUpBehaviour := EnableScaleUpBehaviourForHPA(c.k8sClient, workloadMeta.Namespace, workloadMeta.Kind, workloadMeta.Name, c.enableScaleUpBehaviourForAll)
	var maxScaleUpPods int
	var scaleUpPeriodSeconds int
	if enableScaleUpBehaviour {
		maxScaleUpPods, err = GetMaxScaleUpPods(c.k8sClient, workloadMeta.Namespace, workloadMeta.Kind, workloadMeta.Name, workloadMaxReplicas, c.defaultMaxScaleUpPodsPercentage)
		if err != nil {
			c.logger.Error(err, "Error while getting maxScaleUpPods")
			return nil, err
		}
		scaleUpPeriodSeconds = int(math.Min((math.Ceil(acl.Minutes()) * 60), float64(c.defaultScaleUpPeriodSeconds)))
	}

	optimalTargetUtil, minReplicas, maxReplicas, savings, err := c.findOptimalHPAConfigurations(dataPoints,
		acl,
		c.minTarget,
		c.maxTarget,
		perPodResources, workloadMaxReplicas, workloadMinReplicas, enableScaleUpBehaviour, maxScaleUpPods, scaleUpPeriodSeconds)
	if err != nil {
		if errors.Is(err, unableToRecommendError) {
			potentialSavingsPercentage.WithLabelValues(workloadMeta.Namespace, workloadMeta.Name).Set(savings)
			return &v1alpha1.HPAConfiguration{Min: workloadMaxReplicas, Max: workloadMaxReplicas, TargetMetricValue: c.minTarget}, nil
		}
		c.logger.Error(err, "Error while executing findOptimalTargetUtilization")
		return nil, err
	}

	potentialSavingsPercentage.WithLabelValues(workloadMeta.Namespace, workloadMeta.Name).Set(savings)
	return &v1alpha1.HPAConfiguration{Min: minReplicas, Max: maxReplicas, TargetMetricValue: optimalTargetUtil}, nil
}

type TimerEvent struct {
	Timestamp time.Time
	Delta     float64
}

// simulateHPA simulates the operation of HPA by adding a delay of amount Autoscaling Cycle Lag (ACL)
// to all upscale events. It takes as input
// dataPoints - sum of cpu utilization data points for a workload.
// acl - Autoscaling Cycle Lag for the workload
// perPodResources - these are required ot more accurately mimic the working of HPA by making the available resources
// multiples of perPodResources.

func (c *CpuUtilizationBasedRecommender) simulateHPA(dataPoints []metrics.DataPoint,
	acl time.Duration,
	targetUtilization int,
	perPodResources float64, maxReplicas int, minReplicas int) ([]metrics.DataPoint, int, error) {

	targetUtilization = int(math.Floor(float64(targetUtilization) * HPAScaleTolerationFactor))

	if len(dataPoints) == 0 {
		return []metrics.DataPoint{}, 0, nil
	}
	if targetUtilization < 1 || targetUtilization > 100 {
		return []metrics.DataPoint{}, 0, errors.New(fmt.Sprintf("Invalid value of target utilization: %v."+
			" Value should be between 1 and 100", targetUtilization))
	}

	simulatedDataPoints := make([]metrics.DataPoint, len(dataPoints))

	currentReplicas := math.Min(float64(maxReplicas), math.Max(float64(minReplicas), math.Ceil((dataPoints[0].Value*100)/float64(targetUtilization)/perPodResources)))
	calculatedMinReplicas := math.Ceil((dataPoints[0].Value * 100) / float64(targetUtilization) / perPodResources)
	currentResources := currentReplicas * perPodResources
	readyResources := currentResources

	simulatedDataPoints[0] = metrics.DataPoint{Timestamp: dataPoints[0].Timestamp,
		Value: currentResources * c.redLineUtil}

	//stores the list of all upscale events with a time delay of acl added.
	readyResourcesTimerList := []TimerEvent{}

	for i, dp := range dataPoints[1:] {

		// Consume timers for all upscale events before the current time.
		for len(readyResourcesTimerList) > 0 && !dp.Timestamp.Before(readyResourcesTimerList[0].Timestamp) {
			readyResources += readyResourcesTimerList[0].Delta
			readyResourcesTimerList = readyResourcesTimerList[1:]
		}
		newReplicas := math.Min(float64(maxReplicas), math.Max(float64(minReplicas), math.Ceil((100*dp.Value)/float64(targetUtilization)/perPodResources)))
		calculatedMinReplicas = math.Min(calculatedMinReplicas, math.Ceil((100*dp.Value)/float64(targetUtilization)/perPodResources))

		newResources := newReplicas * perPodResources
		currentResources = newResources

		if newResources > readyResources {
			delta := newResources - readyResources

			//subtract delta that is already in queue.
			for _, timer := range readyResourcesTimerList {
				delta -= timer.Delta
			}

			if delta > 0 {
				readyReplicasTimer := TimerEvent{Timestamp: dp.Timestamp.Add(acl), Delta: delta}
				readyResourcesTimerList = append(readyResourcesTimerList, readyReplicasTimer)
			}

		} else {
			readyResources = newResources
			readyResourcesTimerList = []TimerEvent{}
		}

		availableResources := readyResources * c.redLineUtil
		simulatedDataPoints[i+1] = metrics.DataPoint{Timestamp: dp.Timestamp, Value: availableResources}
	}

	return simulatedDataPoints, int(calculatedMinReplicas), nil
}

func (c *CpuUtilizationBasedRecommender) simulateHPAWithScaleBehaviours(dataPoints []metrics.DataPoint,
	acl time.Duration,
	targetUtilization int,
	perPodResources float64, maxReplicas int, minReplicas int, maxScaleUpPods int, scaleUpPeriodSeconds int) ([]metrics.DataPoint, int, error) {

	targetUtilization = int(math.Floor(float64(targetUtilization) * HPAScaleTolerationFactor))
	scaleUpPeriodSecondsDuration := time.Duration(scaleUpPeriodSeconds) * time.Second

	var scaleUpEvents []timestampedScaleEvent
	var recentRecommendations []replicaRecommendation

	if len(dataPoints) == 0 {
		return []metrics.DataPoint{}, 0, nil
	}
	if targetUtilization < 1 || targetUtilization > 100 {
		return []metrics.DataPoint{}, 0, errors.New(fmt.Sprintf("Invalid value of target utilization: %v."+
			" Value should be between 1 and 100", targetUtilization))
	}

	simulatedDataPoints := make([]metrics.DataPoint, len(dataPoints))

	currentReplicas := math.Min(float64(maxReplicas), math.Max(float64(minReplicas), math.Ceil((dataPoints[0].Value*100)/float64(targetUtilization)/perPodResources)))
	calculatedMinReplicas := math.Ceil((dataPoints[0].Value * 100) / float64(targetUtilization) / perPodResources)
	currentResources := currentReplicas * perPodResources
	readyResources := currentResources

	recentRecommendations = append(recentRecommendations, replicaRecommendation{Timestamp: dataPoints[0].Timestamp, Replicas: int(currentReplicas)})

	simulatedDataPoints[0] = metrics.DataPoint{Timestamp: dataPoints[0].Timestamp,
		Value: currentResources * c.redLineUtil}

	//stores the list of all upscale events with a time delay of acl added.
	readyResourcesTimerList := []TimerEvent{}

	for i, dp := range dataPoints[1:] {

		// Consume timers for all upscale events before the current time.
		for len(readyResourcesTimerList) > 0 && !dp.Timestamp.Before(readyResourcesTimerList[0].Timestamp) {
			readyResources += readyResourcesTimerList[0].Delta
			readyResourcesTimerList = readyResourcesTimerList[1:]
		}

		//Trim the scale events outside of the periodSeconds
		cutoff := dp.Timestamp.Add(-scaleUpPeriodSecondsDuration)
		for _, event := range scaleUpEvents {
			if event.timestamp.Before(cutoff) {
				scaleUpEvents = scaleUpEvents[1:]
			}
		}

		//Trim the recent recommendations outside of the stabilization window
		stabilizationWindowCutoff := dp.Timestamp.Add(-c.downScaleStabilizationWindow)
		for _, rec := range recentRecommendations {
			if rec.Timestamp.Before(stabilizationWindowCutoff) {
				recentRecommendations = recentRecommendations[1:]
			}
		}

		newReplicas := math.Min(float64(maxReplicas), math.Max(float64(minReplicas), math.Ceil((100*dp.Value)/float64(targetUtilization)/perPodResources)))

		if newReplicas < currentReplicas {
			recentRecommendations = append(recentRecommendations, replicaRecommendation{Timestamp: dp.Timestamp, Replicas: int(newReplicas)})
			newReplicas = math.Max(newReplicas, c.getMaxRecommendationInStabilizationWindowDuration(dp, recentRecommendations))
		} else {
			normalizedNewReplicas := normalizeReplicasWithScaleUpBehaviour(currentReplicas, scaleUpEvents, dp, maxScaleUpPods, scaleUpPeriodSecondsDuration)

			newReplicas = math.Min(newReplicas, normalizedNewReplicas)
			if newReplicas > currentReplicas {
				scaleUpEvents = append(scaleUpEvents,
					timestampedScaleEvent{
						timestamp:     dp.Timestamp,
						replicaChange: newReplicas - currentReplicas,
					})
			}
			recentRecommendations = append(recentRecommendations, replicaRecommendation{Timestamp: dp.Timestamp, Replicas: int(newReplicas)})
		}
		currentReplicas = newReplicas
		calculatedMinReplicas = math.Min(calculatedMinReplicas, math.Ceil((100*dp.Value)/float64(targetUtilization)/perPodResources))

		newResources := newReplicas * perPodResources
		currentResources = newResources

		if newResources > readyResources {
			delta := newResources - readyResources

			//subtract delta that is already in queue.
			for _, timer := range readyResourcesTimerList {
				delta -= timer.Delta
			}

			if delta > 0 {
				readyReplicasTimer := TimerEvent{Timestamp: dp.Timestamp.Add(acl), Delta: delta}
				readyResourcesTimerList = append(readyResourcesTimerList, readyReplicasTimer)
			}

		} else {
			readyResources = newResources
			readyResourcesTimerList = []TimerEvent{}
		}

		availableResources := readyResources * c.redLineUtil
		simulatedDataPoints[i+1] = metrics.DataPoint{Timestamp: dp.Timestamp, Value: availableResources}
	}

	return simulatedDataPoints, int(calculatedMinReplicas), nil
}

func (c *CpuUtilizationBasedRecommender) hasNoBreachOccurred(original, simulated []metrics.DataPoint) bool {
	for i := range original {
		if original[i].Value > simulated[i].Value {
			return false
		}
	}
	return true
}

func (c *CpuUtilizationBasedRecommender) findOptimalHPAConfigurations(dataPoints []metrics.DataPoint,
	acl time.Duration,
	minTarget,
	maxTarget int,
	perPodResources float64, maxReplicas int, minReplicas int, enableScaleUpBehaviour bool, maxScaleUpPods int, scaleUpPeriodSeconds int) (int, int, int, float64, error) {

	optimalTargetThreshold := 0
	optimalMin := 0
	savings := 0.0

	for ; minReplicas <= maxReplicas; minReplicas++ {
		calculatedMin := 0
		low := minTarget
		high := maxTarget
		var simulatedHPAList []metrics.DataPoint
		for low <= high {
			mid := low + (high-low)/2
			target := mid
			var err error
			if !enableScaleUpBehaviour {
				simulatedHPAList, calculatedMin, err = c.simulateHPA(dataPoints, acl, target, perPodResources, maxReplicas, minReplicas)
			} else {
				simulatedHPAList, calculatedMin, err = c.simulateHPAWithScaleBehaviours(dataPoints, acl, target, perPodResources, maxReplicas, minReplicas, maxScaleUpPods, scaleUpPeriodSeconds)
			}
			if err != nil {
				c.logger.Error(err, "Error while simulating HPA")
				return -1, minReplicas, maxReplicas, 0, err
			}

			if c.hasNoBreachOccurred(dataPoints, simulatedHPAList) {
				low = mid + 1
			} else {
				high = mid - 1
			}
		}
		if high >= minTarget && calculatedMin <= minReplicas {
			if len(simulatedHPAList) > 0 {
				newSavings := c.calculateSavings(maxReplicas, simulatedHPAList, perPodResources)
				if newSavings >= savings {
					optimalMin = minReplicas
					optimalTargetThreshold = high
					savings = newSavings
				}
			}
		}
	}

	if optimalTargetThreshold < minTarget || savings == 0.0 {
		return 0, 0, 0, 0, unableToRecommendError
	}
	return optimalTargetThreshold, optimalMin, maxReplicas, savings, nil
}

func (c *CpuUtilizationBasedRecommender) calculateSavings(maxReplicas int, simulated []metrics.DataPoint, perPodResources float64) float64 {
	savings := 0.0
	for _, dp := range simulated {
		sm := dp.Value / c.redLineUtil
		savings += (float64(maxReplicas) * perPodResources) - sm
	}

	savings = savings / (float64(maxReplicas) * perPodResources)
	savings = savings / float64(len(simulated))
	return savings * 100.0
}

func (c *CpuUtilizationBasedRecommender) getContainerCPULimitsSum(namespace, objectKind, objectName string) (float64,
	error) {
	var obj client.Object
	switch objectKind {
	case "Deployment":
		obj = &appsv1.Deployment{}
	case "Rollout":
		obj = &rolloutv1alpha1.Rollout{}
	default:
		return 0, fmt.Errorf("unsupported objectKind: %s", objectKind)
	}

	if err := c.k8sClient.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: objectName}, obj); err != nil {
		return 0, err
	}

	var podTemplateSpec *corev1.PodTemplateSpec
	switch v := obj.(type) {
	case *appsv1.Deployment:
		podTemplateSpec = &v.Spec.Template
	case *rolloutv1alpha1.Rollout:
		podTemplateSpec = &v.Spec.Template
	default:
		return 0, fmt.Errorf("unsupported object type")
	}

	podList := &corev1.PodList{}

	if podTemplateSpec.Labels == nil {
		return 0, fmt.Errorf("no labels present on the workload to fetch pod")
	}

	labelSet := labels.Set(podTemplateSpec.Labels)
	selector := labels.SelectorFromSet(labelSet)

	if err := c.k8sClient.List(context.Background(), podList, client.InNamespace(namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return 0, err
	}

	cpuLimitsSum := int64(0)

	if len(podList.Items) == 0 {
		return 0, fmt.Errorf("no pod found for the workload")
	}

	for _, container := range podList.Items[0].Spec.Containers {
		if limit, ok := container.Resources.Limits[corev1.ResourceCPU]; ok {
			cpuLimitsSum += limit.MilliValue()
		}
	}

	return float64(cpuLimitsSum) / 1000, nil
}

func (c *CpuUtilizationBasedRecommender) getMaxPods(namespace string, objectKind string, objectName string) (int, error) {
	var obj client.Object
	switch objectKind {
	case "Deployment":
		obj = &appsv1.Deployment{}
	case "Rollout":
		obj = &rolloutv1alpha1.Rollout{}
	default:
		return 0, fmt.Errorf("unsupported objectKind: %s", objectKind)
	}

	if err := c.k8sClient.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: objectName}, obj); err != nil {
		return 0, err
	}

	maxPodAnnotation, ok := obj.GetAnnotations()[OttoscalrMaxPodAnnotation]

	var maxPods int

	if ok {
		var err error
		maxPods, err = strconv.Atoi(maxPodAnnotation)
		if err != nil {
			return 0, fmt.Errorf("unable to convert maxPods from string to int: %s", err)
		}

	} else {
		scaledObjects := &kedaapi.ScaledObjectList{}
		if err := c.k8sClient.List(context.Background(), scaledObjects, &client.ListOptions{
			FieldSelector: fields.OneTermEqualSelector(ScaledObjectField, objectName),
			Namespace:     namespace,
		}); err != nil && client.IgnoreNotFound(err) != nil {
			return 0, fmt.Errorf("unable to fetch scaledobjects: %s", err)
		}

		if len(scaledObjects.Items) > 0 && scaledObjects.Items[0].Spec.MaxReplicaCount != nil {
			maxPods = int(*scaledObjects.Items[0].Spec.MaxReplicaCount)
		} else {
			switch v := obj.(type) {
			case *appsv1.Deployment:
				maxPods = int(*v.Spec.Replicas)
			case *rolloutv1alpha1.Rollout:
				maxPods = int(*v.Spec.Replicas)
			default:
				return 0, fmt.Errorf("unsupported object type")
			}
		}

	}

	return maxPods, nil
}

func (c *CpuUtilizationBasedRecommender) isMetricsAboveThreshold(dataPoints []metrics.DataPoint) bool {
	totalDataPoints := int(c.metricWindow.Seconds()) / int(c.metricStep.Seconds())
	percentageOfDataPointsFetched := (float64(len(dataPoints)) / float64(totalDataPoints)) * 100
	if int(percentageOfDataPointsFetched) < c.metricsPercentageThreshold {
		return false
	}
	return true
}

func normalizeReplicasWithScaleUpBehaviour(currentReplicas float64, scaleUpEvents []timestampedScaleEvent, dp metrics.DataPoint, maxScaleUpPods int, scaleUpPeriodSecondsDuration time.Duration) float64 {
	cutoff := dp.Timestamp.Add(-scaleUpPeriodSecondsDuration)
	var replicasAddedInCurrentPeriod float64
	for _, events := range scaleUpEvents {
		if events.timestamp.After(cutoff) {
			replicasAddedInCurrentPeriod += events.replicaChange
		}
	}

	periodStartReplicas := currentReplicas - replicasAddedInCurrentPeriod
	maximumAllowedReplicas := periodStartReplicas + float64(maxScaleUpPods)

	return maximumAllowedReplicas

}

func (c *CpuUtilizationBasedRecommender) getMaxRecommendationInStabilizationWindowDuration(dp metrics.DataPoint, recentRecommendations []replicaRecommendation) float64 {
	stabilizationWindowCutoff := dp.Timestamp.Add(-c.downScaleStabilizationWindow)
	max := 0.0
	for _, rec := range recentRecommendations[0:] {
		if rec.Timestamp.After(stabilizationWindowCutoff) {
			if float64(rec.Replicas) > max {
				max = float64(rec.Replicas)
			}
		}
	}

	return max
}

// daySlice holds the data points for a single calendar-day slice of a time window,
// with a warmup prefix to prime HPA simulation state before the actual window starts.
type daySlice struct {
	points       []metrics.DataPoint
	warmupCutoff int // index of first point >= startHour (actual window portion starts here)
}

// filterDataPointsByHourRange groups all datapoints by calendar day (in IST) and
// returns a map from day → daySlice. Each slice covers [startHour-warmup, endHour).
// The warmupCutoff field marks where the actual window portion begins within the slice.
func filterDataPointsByHourRange(
	dps []metrics.DataPoint,
	startHour, endHour int,
	warmupDuration time.Duration,
	loc *time.Location,
) map[time.Time]daySlice {
	result := make(map[time.Time]daySlice)

	warmupStart := func(t time.Time) time.Time {
		windowStart := time.Date(t.Year(), t.Month(), t.Day(), startHour, 0, 0, 0, loc)
		return windowStart.Add(-warmupDuration)
	}
	windowEnd := func(t time.Time) time.Time {
		return time.Date(t.Year(), t.Month(), t.Day(), endHour, 0, 0, 0, loc)
	}

	dayMap := make(map[time.Time][]metrics.DataPoint)
	for _, dp := range dps {
		dpLocal := dp.Timestamp.In(loc)
		dayKey := time.Date(dpLocal.Year(), dpLocal.Month(), dpLocal.Day(), 0, 0, 0, 0, loc)
		ws := warmupStart(dpLocal)
		we := windowEnd(dpLocal)
		if (dp.Timestamp.Equal(ws) || dp.Timestamp.After(ws)) && dp.Timestamp.Before(we) {
			dayMap[dayKey] = append(dayMap[dayKey], dp)
		}
	}

	for dayKey, pts := range dayMap {
		if len(pts) == 0 {
			continue
		}
		actualWindowStart := time.Date(dayKey.Year(), dayKey.Month(), dayKey.Day(), startHour, 0, 0, 0, loc)
		cutoff := 0
		for cutoff < len(pts) && pts[cutoff].Timestamp.Before(actualWindowStart) {
			cutoff++
		}
		result[dayKey] = daySlice{
			points:       pts,
			warmupCutoff: cutoff,
		}
	}
	return result
}

// recommendForWindow runs the HPA recommendation algorithm for a single time window
// across all historical days, and returns the most conservative config that avoids
// breaches in the actual window portion (excluding the warmup prefix).
// Returns nil if no valid recommendation can be derived (no-op fallback expected by caller).
func (c *CpuUtilizationBasedRecommender) recommendForWindow(
	allDataPoints []metrics.DataPoint,
	startHour, endHour int,
	acl time.Duration,
	perPodResources float64,
	workloadMaxReplicas, workloadMinReplicas int,
	enableScaleUpBehaviour bool,
	maxScaleUpPods, scaleUpPeriodSeconds int,
	loc *time.Location,
) *v1alpha1.HPAConfiguration {

	daySlices := filterDataPointsByHourRange(allDataPoints, startHour, endHour, acl, loc)
	if len(daySlices) == 0 {
		return nil
	}

	// Track the most conservative result across all days.
	// Most conservative = lowest targetUtil AND highest minReplicas.
	bestTargetUtil := c.maxTarget
	bestMinReplicas := workloadMinReplicas
	foundValid := false

	for _, ds := range daySlices {
		if len(ds.points) == 0 {
			continue
		}

		// Binary search for best target utilization for this day's window.
		low := c.minTarget
		high := c.maxTarget
		var lastGoodSimulated []metrics.DataPoint
		var lastGoodCalculatedMin int

		for low <= high {
			mid := low + (high-low)/2
			var simulated []metrics.DataPoint
			var calculatedMin int
			var err error
			if !enableScaleUpBehaviour {
				simulated, calculatedMin, err = c.simulateHPA(ds.points, acl, mid, perPodResources, workloadMaxReplicas, workloadMinReplicas)
			} else {
				simulated, calculatedMin, err = c.simulateHPAWithScaleBehaviours(ds.points, acl, mid, perPodResources, workloadMaxReplicas, workloadMinReplicas, maxScaleUpPods, scaleUpPeriodSeconds)
			}
			if err != nil {
				break
			}

			// Check breach only on the actual window portion (skip warmup).
			windowOriginal := ds.points[ds.warmupCutoff:]
			windowSimulated := simulated[ds.warmupCutoff:]
			if c.hasNoBreachOccurred(windowOriginal, windowSimulated) {
				lastGoodSimulated = simulated
				lastGoodCalculatedMin = calculatedMin
				low = mid + 1
			} else {
				high = mid - 1
			}
		}

		if high < c.minTarget || lastGoodSimulated == nil {
			continue
		}
		if lastGoodCalculatedMin > workloadMinReplicas {
			continue
		}

		// This day yields a valid config. Apply most-conservative selection.
		if !foundValid {
			bestTargetUtil = high
			bestMinReplicas = workloadMinReplicas
			foundValid = true
		} else {
			if high < bestTargetUtil {
				bestTargetUtil = high
			}
		}
	}

	if !foundValid {
		return nil
	}

	return &v1alpha1.HPAConfiguration{
		Min:               bestMinReplicas,
		Max:               workloadMaxReplicas,
		TargetMetricValue: bestTargetUtil,
	}
}

// RecommendWindowed runs the recommendation algorithm independently per time window
// and returns both per-window configs and a global (most conservative) fallback config.
// When enableTimeWindows is false or numberOfWindows <= 0, it falls back to the global
// (non-windowed) algorithm and returns nil windowed configs for backward compatibility.
func (c *CpuUtilizationBasedRecommender) RecommendWindowed(ctx context.Context, workloadMeta WorkloadMeta) (*v1alpha1.HPAConfiguration, []v1alpha1.TimeWindowedHPAConfiguration, error) {
	if !c.enableTimeWindows || c.numberOfWindows <= 0 {
		cfg, err := c.recommendGlobal(ctx, workloadMeta)
		return cfg, nil, err
	}

	end := time.Now()
	start := end.Add(-c.metricWindow)

	utilizationQueryStartTime := time.Now()
	dataPoints, err := c.scraper.GetAverageCPUUtilizationByWorkload(workloadMeta.Namespace,
		workloadMeta.Name,
		workloadMeta.Kind,
		start,
		end,
		c.metricStep)
	if err != nil {
		c.logger.Error(err, "Error while scraping GetAverageCPUUtilizationByWorkload.")
		return nil, nil, err
	}
	getAverageCPUUtilizationQueryLatency.WithLabelValues(workloadMeta.Namespace, workloadMeta.Name, workloadMeta.Kind, workloadMeta.Name).Observe(time.Since(utilizationQueryStartTime).Seconds())

	workloadMaxReplicas, err := c.getMaxPods(workloadMeta.Namespace, workloadMeta.Kind, workloadMeta.Name)
	if err != nil {
		c.logger.Error(err, "Error while getting getMaxPods")
		return nil, nil, err
	}

	workloadMinReplicas, err := getMinPods(c.k8sClient, workloadMeta.Namespace, workloadMeta.Kind, workloadMeta.Name)
	if err != nil {
		c.logger.Error(err, "Error while getting getMinPods")
		return nil, nil, err
	}

	if !c.isMetricsAboveThreshold(dataPoints) {
		minPercentageOfDataPointsPresent.WithLabelValues(workloadMeta.Namespace, workloadMeta.Name).Set(float64(0))
		c.logger.Error(fmt.Errorf("insufficient metrics"), "Setting the recommendation to no operation policy")
		noOp := &v1alpha1.HPAConfiguration{Min: workloadMaxReplicas, Max: workloadMaxReplicas, TargetMetricValue: c.minTarget}
		return noOp, nil, nil
	}
	minPercentageOfDataPointsPresent.WithLabelValues(workloadMeta.Namespace, workloadMeta.Name).Set(float64(1))

	if c.metricsTransformer != nil {
		for _, t := range c.metricsTransformer {
			dataPoints, err = t.Transform(start, end, dataPoints)
			if err != nil {
				c.logger.Error(err, "Error while transforming metrics")
				return nil, nil, err
			}
		}
	}

	acl, err := c.scraper.GetACLByWorkload(workloadMeta.Namespace, workloadMeta.Name, workloadMeta.Kind)
	if err != nil {
		c.logger.Error(err, "Error while getting GetACL.")
		return nil, nil, err
	}

	perPodResources, err := c.getContainerCPULimitsSum(workloadMeta.Namespace, workloadMeta.Kind, workloadMeta.Name)
	if err != nil {
		c.logger.Error(err, "Error while getting getContainerCPULimitsSum")
		return nil, nil, err
	}

	enableScaleUpBehaviour := EnableScaleUpBehaviourForHPA(c.k8sClient, workloadMeta.Namespace, workloadMeta.Kind, workloadMeta.Name, c.enableScaleUpBehaviourForAll)
	var maxScaleUpPods int
	var scaleUpPeriodSeconds int
	if enableScaleUpBehaviour {
		maxScaleUpPods, err = GetMaxScaleUpPods(c.k8sClient, workloadMeta.Namespace, workloadMeta.Kind, workloadMeta.Name, workloadMaxReplicas, c.defaultMaxScaleUpPodsPercentage)
		if err != nil {
			c.logger.Error(err, "Error while getting maxScaleUpPods")
			return nil, nil, err
		}
		scaleUpPeriodSeconds = int(math.Min((math.Ceil(acl.Minutes()) * 60), float64(c.defaultScaleUpPeriodSeconds)))
	}

	istLocation := time.FixedZone("IST", 5*60*60+30*60)
	hoursPerWindow := 24 / c.numberOfWindows

	var windowedConfigs []v1alpha1.TimeWindowedHPAConfiguration

	// noOp config used as fallback when a window has no valid recommendation
	noOpConfig := v1alpha1.HPAConfiguration{Min: workloadMaxReplicas, Max: workloadMaxReplicas, TargetMetricValue: c.minTarget}

	// Global (most conservative) config across all windows
	globalTargetUtil := c.maxTarget
	globalMinReplicas := workloadMinReplicas
	globalMaxReplicas := workloadMaxReplicas
	foundAnyValid := false

	for i := 0; i < c.numberOfWindows; i++ {
		wStartHour := i * hoursPerWindow
		wEndHour := wStartHour + hoursPerWindow

		winCfg := c.recommendForWindow(
			dataPoints,
			wStartHour, wEndHour,
			acl,
			perPodResources,
			workloadMaxReplicas, workloadMinReplicas,
			enableScaleUpBehaviour,
			maxScaleUpPods, scaleUpPeriodSeconds,
			istLocation,
		)

		var hpaCfg v1alpha1.HPAConfiguration
		if winCfg != nil {
			hpaCfg = *winCfg
			if !foundAnyValid {
				globalTargetUtil = winCfg.TargetMetricValue
				globalMinReplicas = winCfg.Min
				foundAnyValid = true
			} else {
				if winCfg.TargetMetricValue < globalTargetUtil {
					globalTargetUtil = winCfg.TargetMetricValue
				}
				if winCfg.Min > globalMinReplicas {
					globalMinReplicas = winCfg.Min
				}
			}
		} else {
			hpaCfg = noOpConfig
		}

		windowedConfigs = append(windowedConfigs, v1alpha1.TimeWindowedHPAConfiguration{
			WindowIndex:      i,
			StartHour:        wStartHour,
			EndHour:          wEndHour,
			HPAConfiguration: hpaCfg,
		})
	}

	var globalConfig *v1alpha1.HPAConfiguration
	if foundAnyValid {
		globalConfig = &v1alpha1.HPAConfiguration{
			Min:               globalMinReplicas,
			Max:               globalMaxReplicas,
			TargetMetricValue: globalTargetUtil,
		}
	} else {
		globalConfig = &noOpConfig
	}

	potentialSavingsPercentage.WithLabelValues(workloadMeta.Namespace, workloadMeta.Name).Set(0)
	return globalConfig, windowedConfigs, nil
}
