package reco

import (
	"context"
	"fmt"
	rolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	"github.com/flipkart-incubator/ottoscalr/pkg/metrics"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type Recommender interface {
	Recommend(workloadSpec v1alpha1.WorkloadSpec) (*v1alpha1.HPAConfiguration, error)
}

type CpuUtilizationBasedRecommender struct {
	k8sClient       client.Client
	ctx             context.Context
	currentReplicas int
	redLineUtil     float32
	metricWindow    time.Duration
	scraper         metrics.Scraper
	metricStep      time.Duration
	minTarget       int
	maxTarget       int
	logger          logr.Logger
}

func NewCpuUtilizationBasedRecommender(k8sClient client.Client,
	ctx context.Context,
	currentReplicas int, redLineUtil float32,
	metricWindow time.Duration,
	scraper metrics.Scraper,
	metricStep time.Duration,
	minTarget int,
	maxTarget int,
	logger logr.Logger) *CpuUtilizationBasedRecommender {
	return &CpuUtilizationBasedRecommender{
		k8sClient:       k8sClient,
		ctx:             ctx,
		currentReplicas: currentReplicas,
		redLineUtil:     redLineUtil,
		metricWindow:    metricWindow,
		scraper:         scraper,
		metricStep:      metricStep,
		minTarget:       minTarget,
		maxTarget:       maxTarget,
		logger:          logger,
	}
}

func (c *CpuUtilizationBasedRecommender) Recommend(workloadSpec v1alpha1.WorkloadSpec) (*v1alpha1.HPAConfiguration,
	error) {

	end := time.Now()
	start := end.Add(-24 * c.metricWindow * time.Hour)

	dataPoints, err := c.scraper.GetCPUUtilizationBreachDataPoints(workloadSpec.Namespace,
		workloadSpec.Kind,
		workloadSpec.Name,
		c.redLineUtil,
		start,
		end,
		c.metricStep)
	if err != nil {
		c.logger.Error(err, "Error while scraping GetCPUUtilizationBreachDataPoints.")
		return nil, nil
	}

	acl, err := c.scraper.GetACL(workloadSpec.Namespace, workloadSpec.Kind, workloadSpec.Name)
	if err != nil {
		c.logger.Error(err, "Error while getting GetACL.")
		return nil, nil
	}

	optimalTargetUtil := c.findOptimalTargetUtilization(dataPoints, acl, c.minTarget, c.maxTarget)

	return &v1alpha1.HPAConfiguration{Min: 0, Max: 0, TargetMetricValue: optimalTargetUtil}, nil
}

func (c *CpuUtilizationBasedRecommender) simulateHPA(dataPoints []metrics.DataPoint,
	acl time.Duration,
	targetUtilization int) []metrics.DataPoint {
	simulatedDataPoints := make([]metrics.DataPoint, len(dataPoints))
	readyReplicas := float64(c.currentReplicas)
	readyReplicasTimer := time.Time{}

	for i, dp := range dataPoints {
		newReplicas := float64(c.currentReplicas) * float64(dp.Value) / float64(targetUtilization)
		c.currentReplicas = int(newReplicas)

		if readyReplicas < newReplicas {
			if readyReplicasTimer.Before(dp.Timestamp) {
				readyReplicasTimer = dp.Timestamp.Add(acl)
			}
			if readyReplicasTimer.Before(dp.Timestamp) {
				readyReplicas = newReplicas
			}
		} else {
			readyReplicas = newReplicas
		}

		availableResources := readyReplicas * float64(c.redLineUtil)
		simulatedDataPoints[i] = metrics.DataPoint{Timestamp: dp.Timestamp, Value: float64(availableResources)}
	}

	return simulatedDataPoints
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
	maxTarget int) int {
	low := minTarget / 5
	high := maxTarget / 5

	for low <= high {
		mid := low + (high-low)/2
		target := mid * 5

		simulatedHPAList := c.simulateHPA(dataPoints, acl, target)
		noBreaches := c.hasNoBreachOccurred(dataPoints, simulatedHPAList)

		if noBreaches {
			low = mid + 1
		} else {
			high = mid - 1
		}
	}
	return high * 5
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

	if err := c.k8sClient.Get(c.ctx, types.NamespacedName{Namespace: namespace, Name: objectName}, obj); err != nil {
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
