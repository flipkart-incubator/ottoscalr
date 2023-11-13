package autoscaler

import (
	"context"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type HPAClientV2 struct {
	k8sClient client.Client
}

func NewHPAClientV2(k8sClient client.Client) *HPAClientV2 {
	return &HPAClientV2{
		k8sClient: k8sClient,
	}
}

func (hc *HPAClientV2) GetMaxReplicaCount(obj client.Object) int32 {
	hpa := obj.(*autoscalingv2.HorizontalPodAutoscaler)
	maxPods := hpa.Spec.MaxReplicas
	return maxPods
}

func (hc *HPAClientV2) GetName() string {
	return "HPA"
}

func (hc *HPAClientV2) GetList(ctx context.Context, labelSelector labels.Selector, namespace string, fieldSelector fields.Selector) ([]client.Object, error) {
	hpas := &autoscalingv2.HorizontalPodAutoscalerList{}
	if err := hc.k8sClient.List(ctx, hpas, &client.ListOptions{
		FieldSelector: fieldSelector,
		LabelSelector: labelSelector,
		Namespace:     namespace,
	}); err != nil {
		return nil, err
	}

	var result []client.Object

	for _, hpa := range hpas.Items {
		result = append(result, &hpa)
	}

	return result, nil

}

func (hc *HPAClientV2) GetType() client.Object {
	return &autoscalingv2.HorizontalPodAutoscaler{}
}

func (hc *HPAClientV2) DeleteAutoscaler(ctx context.Context, obj client.Object) error {
	err := hc.k8sClient.Delete(ctx, obj)
	if err != nil {
		return err
	}
	return nil
}

func (hc *HPAClientV2) GetScaleTargetName(obj client.Object) string {
	hpa := obj.(*autoscalingv2.HorizontalPodAutoscaler)
	return hpa.Spec.ScaleTargetRef.Name
}

func (hc *HPAClientV2) CreateOrUpdateAutoscaler(ctx context.Context, workload client.Object, labels map[string]string,
	max int32, min int32, targetCPUUtilization int32) (string, error) {
	hpa := autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workload.GetName(),
			Namespace: workload.GetNamespace(),
			Labels:    labels,
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				Name:       workload.GetName(),
				APIVersion: workload.GetObjectKind().GroupVersionKind().GroupVersion().String(),
				Kind:       workload.GetObjectKind().GroupVersionKind().Kind,
			},
			MinReplicas: &min,
			MaxReplicas: max,
			Metrics: []autoscalingv2.MetricSpec{
				{
					Type: "Resource",
					Resource: &autoscalingv2.ResourceMetricSource{
						Name: "cpu",
						Target: autoscalingv2.MetricTarget{
							Type:               "Utilization",
							AverageUtilization: &targetCPUUtilization, // Target CPU utilization percentage
						},
					},
				},
			},
		},
	}

	result, err := controllerutil.CreateOrUpdate(ctx, hc.k8sClient, &hpa, func() error {
		hpa.Spec = autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				Name:       workload.GetName(),
				APIVersion: workload.GetObjectKind().GroupVersionKind().GroupVersion().String(),
				Kind:       workload.GetObjectKind().GroupVersionKind().Kind,
			},
			MinReplicas: &min,
			MaxReplicas: max,
			Metrics: []autoscalingv2.MetricSpec{
				{
					Type: "Resource",
					Resource: &autoscalingv2.ResourceMetricSource{
						Name: "cpu",
						Target: autoscalingv2.MetricTarget{
							Type:               "Utilization",
							AverageUtilization: &targetCPUUtilization, // Target CPU utilization percentage
						},
					},
				},
			},
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	return string(result), nil
}
