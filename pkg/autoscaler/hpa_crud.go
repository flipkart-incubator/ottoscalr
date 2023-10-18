package autoscaler

import (
	"context"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type HPACRUD struct {
	k8sClient client.Client
}

func NewHPACRUD(k8sClient client.Client) *HPACRUD {
	return &HPACRUD{
		k8sClient: k8sClient,
	}
}

func (hc *HPACRUD) GetMaxReplicaCount(obj client.Object) int32 {
	hpa := obj.(*autoscalingv1.HorizontalPodAutoscaler)
	maxPods := hpa.Spec.MaxReplicas
	return maxPods
}

func (hc *HPACRUD) GetName() string {
	return "HPA"
}

func (hc *HPACRUD) GetList(ctx context.Context, labelSelector labels.Selector, namespace string, fieldSelector fields.Selector) ([]client.Object, error) {
	hpas := &autoscalingv1.HorizontalPodAutoscalerList{}
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

func (hc *HPACRUD) GetType() client.Object {
	return &autoscalingv1.HorizontalPodAutoscaler{}
}

func (hc *HPACRUD) DeleteAutoscaler(ctx context.Context, obj client.Object) error {
	deletePropagationPolicy := metav1.DeletePropagationForeground
	err := hc.k8sClient.Delete(ctx, obj, &client.DeleteOptions{
		PropagationPolicy: &deletePropagationPolicy,
	})
	if err != nil {
		return err
	}
	return nil
}

func (hc *HPACRUD) GetScaleTargetName(obj client.Object) string {
	hpa := obj.(*autoscalingv1.HorizontalPodAutoscaler)
	return hpa.Spec.ScaleTargetRef.Name
}

func (hc *HPACRUD) CreateOrUpdateAutoscaler(ctx context.Context, workload client.Object, labels map[string]string,
	max int32, min int32, targetCPUUtilization int32) (string, error) {
	hpa := autoscalingv1.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workload.GetName(),
			Namespace: workload.GetNamespace(),
			Labels:    labels,
		},
		Spec: autoscalingv1.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv1.CrossVersionObjectReference{
				Name:       workload.GetName(),
				APIVersion: workload.GetObjectKind().GroupVersionKind().GroupVersion().String(),
				Kind:       workload.GetObjectKind().GroupVersionKind().Kind,
			},
			MinReplicas:                    &min,
			MaxReplicas:                    max,
			TargetCPUUtilizationPercentage: &targetCPUUtilization,
		},
	}

	result, err := controllerutil.CreateOrUpdate(ctx, hc.k8sClient, &hpa, func() error {
		hpa.Spec = autoscalingv1.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv1.CrossVersionObjectReference{
				Name:       workload.GetName(),
				APIVersion: workload.GetObjectKind().GroupVersionKind().GroupVersion().String(),
				Kind:       workload.GetObjectKind().GroupVersionKind().Kind,
			},
			MinReplicas:                    &min,
			MaxReplicas:                    max,
			TargetCPUUtilizationPercentage: &targetCPUUtilization,
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	return string(result), nil
}
