package reco

import (
	"context"
	"errors"
	"fmt"
	rolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	"github.com/flipkart-incubator/ottoscalr/pkg/metrics"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"math"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"time"
)

var unableToRecommendError = errors.New("Unable to generate recommendation without any breaches.")

const OTTOSCALR_MAX_POD_ANNOTATION = "ottoscalr.io/max-pods"

type CpuUtilizationBasedRecommender struct {
	k8sClient          client.Client
	redLineUtil        float64
	metricWindow       time.Duration
	scraper            metrics.Scraper
	metricsTransformer []metrics.MetricsTransformer
	metricStep         time.Duration
	minTarget          int
	maxTarget          int
	logger             logr.Logger
}

func NewCpuUtilizationBasedRecommender(k8sClient client.Client,
	redLineUtil float64,
	metricWindow time.Duration,
	scraper metrics.Scraper,
	metricsTransformer []metrics.MetricsTransformer,
	metricStep time.Duration,
	minTarget int,
	maxTarget int,
	logger logr.Logger) *CpuUtilizationBasedRecommender {
	return &CpuUtilizationBasedRecommender{
		k8sClient:          k8sClient,
		redLineUtil:        redLineUtil,
		metricWindow:       metricWindow,
		scraper:            scraper,
		metricsTransformer: metricsTransformer,
		metricStep:         metricStep,
		minTarget:          minTarget,
		maxTarget:          maxTarget,
		logger:             logger,
	}
}

func (c *CpuUtilizationBasedRecommender) Recommend(ctx context.Context, workloadMeta WorkloadMeta) (*v1alpha1.HPAConfiguration,
	error) {

	end := time.Now()
	start := end.Add(-c.metricWindow)

	dataPoints, err := c.scraper.GetAverageCPUUtilizationByWorkload(workloadMeta.Namespace,
		workloadMeta.Name,
		start,
		end,
		c.metricStep)
	if err != nil {
		c.logger.Error(err, "Error while scraping GetAverageCPUUtilizationByWorkload.")
		return nil, err
	}
	if c.metricsTransformer != nil {
		for _, transformers := range c.metricsTransformer {
			dataPoints, err = transformers.Transform(start, end, dataPoints)
			if err != nil {
				c.logger.Error(err, "Error while getting outlier interval from event api")
				return nil, err
			}
		}
	}

	acl, err := c.scraper.GetACLByWorkload(workloadMeta.Namespace, workloadMeta.Name)
	if err != nil {
		c.logger.Error(err, "Error while getting GetACL.")
		return nil, err
	}

	perPodResources, err := c.getContainerCPULimitsSum(workloadMeta.Namespace, workloadMeta.Kind, workloadMeta.Name)
	if err != nil {
		c.logger.Error(err, "Error while getting getContainerCPULimitsSum")
		return nil, err
	}

	maxReplicas, err := c.getMaxPodFromAnnotations(workloadMeta.Namespace, workloadMeta.Kind, workloadMeta.Name)

	if err != nil {
		c.logger.Error(err, "Error while getting getMaxPodFromAnnotations")
		return nil, err
	}

	optimalTargetUtil, minReplicas, maxReplicas, err := c.findOptimalTargetUtilization(dataPoints,
		acl,
		c.minTarget,
		c.maxTarget,
		perPodResources, maxReplicas)
	if err != nil {
		c.logger.Error(err, "Error while executing findOptimalTargetUtilization")
		return nil, err
	}

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
	perPodResources float64, maxReplicas int) ([]metrics.DataPoint, int, int, error) {

	targetUtilization = int(math.Floor(float64(targetUtilization) * 1.1))

	if len(dataPoints) == 0 {
		return []metrics.DataPoint{}, 0, 0, nil
	}
	if targetUtilization < 1 || targetUtilization > 100 {
		return []metrics.DataPoint{}, 0, 0, errors.New(fmt.Sprintf("Invalid value of target utilization: %v."+
			" Value should be between 1 and 100", targetUtilization))
	}

	simulatedDataPoints := make([]metrics.DataPoint, len(dataPoints))

	currentReplicas := math.Min(float64(maxReplicas), math.Ceil((dataPoints[0].Value*100)/float64(targetUtilization)/perPodResources))
	minReplicas := currentReplicas
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
		newReplicas := math.Min(float64(maxReplicas), math.Ceil((100*dp.Value)/float64(targetUtilization)/perPodResources))
		minReplicas = math.Min(newReplicas, minReplicas)

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

	return simulatedDataPoints, int(minReplicas), maxReplicas, nil
}

func (c *CpuUtilizationBasedRecommender) hasNoBreachOccurred(original, simulated []metrics.DataPoint) bool {
	for i := range original {
		if original[i].Value > simulated[i].Value {
			return false
		}
	}
	return true
}

func (c *CpuUtilizationBasedRecommender) findOptimalTargetUtilization(dataPoints []metrics.DataPoint,
	acl time.Duration,
	minTarget,
	maxTarget int,
	perPodResources float64, maxReplicas int) (int, int, int, error) {
	low := minTarget
	high := maxTarget
	minReplicas := 0

	for low <= high {
		mid := low + (high-low)/2
		target := mid
		var simulatedHPAList []metrics.DataPoint
		var err error
		simulatedHPAList, minReplicas, maxReplicas, err = c.simulateHPA(dataPoints, acl, target, perPodResources, maxReplicas)
		if err != nil {
			c.logger.Error(err, "Error while simulating HPA")
			return -1, minReplicas, maxReplicas, err
		}

		if c.hasNoBreachOccurred(dataPoints, simulatedHPAList) {
			low = mid + 1
		} else {
			high = mid - 1
		}
	}

	if high < minTarget {
		return 0, 0, 0, unableToRecommendError
	}
	return high, minReplicas, maxReplicas, nil
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

	cpuLimitsSum := int64(0)
	for _, container := range podTemplateSpec.Spec.Containers {
		if limit, ok := container.Resources.Limits[corev1.ResourceCPU]; ok {
			cpuLimitsSum += limit.MilliValue()
		}
	}
	return float64(cpuLimitsSum) / 1000, nil
}

func (c *CpuUtilizationBasedRecommender) getMaxPodFromAnnotations(namespace string, objectKind string, objectName string) (int, error) {
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

	maxPodAnnotation, ok := obj.GetAnnotations()[OTTOSCALR_MAX_POD_ANNOTATION]

	var maxPods int

	if ok {
		var err error
		maxPods, err = strconv.Atoi(maxPodAnnotation)
		if err != nil {
			return 0, fmt.Errorf("unable to convert maxPods from string to int")
		}

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

	return maxPods, nil
}
