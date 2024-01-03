package autoscaler

import (
	"context"
	"fmt"

	kedaapi "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type ScaledobjectClient struct {
	k8sClient             client.Client
	enableEventAutoscaler *bool
}

func NewScaledobjectClient(k8sClient client.Client, enableEventAutoscaler *bool) *ScaledobjectClient {

	return &ScaledobjectClient{
		k8sClient:             k8sClient,
		enableEventAutoscaler: enableEventAutoscaler,
	}
}

func (soc *ScaledobjectClient) GetMaxReplicaCount(obj client.Object) int32 {
	maxPods := int32(0)
	scaledObject := obj.(*kedaapi.ScaledObject)
	if scaledObject.Spec.MaxReplicaCount != nil {
		maxPods = *scaledObject.Spec.MaxReplicaCount
	}

	return maxPods
}

func (soc *ScaledobjectClient) GetName() string {
	return "ScaledObject"
}

func (soc *ScaledobjectClient) GetScaleTargetName(obj client.Object) string {
	scaledObject := obj.(*kedaapi.ScaledObject)
	return scaledObject.Spec.ScaleTargetRef.Name
}

func (soc *ScaledobjectClient) GetList(ctx context.Context, labelSelector labels.Selector, namespace string, fieldSelector fields.Selector) ([]client.Object, error) {
	scaledObjects := &kedaapi.ScaledObjectList{}
	if err := soc.k8sClient.List(ctx, scaledObjects, &client.ListOptions{
		FieldSelector: fieldSelector,
		LabelSelector: labelSelector,
		Namespace:     namespace,
	}); err != nil && client.IgnoreNotFound(err) != nil {
		return nil, err
	}

	var result []client.Object

	for _, scaledObject := range scaledObjects.Items {
		result = append(result, &scaledObject)
	}

	return result, nil

}

func (soc *ScaledobjectClient) GetType() client.Object {
	return &kedaapi.ScaledObject{}
}

func (soc *ScaledobjectClient) DeleteAutoscaler(ctx context.Context, obj client.Object) error {
	deletePropagationPolicy := metav1.DeletePropagationForeground
	err := soc.k8sClient.Delete(ctx, obj, &client.DeleteOptions{
		PropagationPolicy: &deletePropagationPolicy,
	})
	if err != nil {
		return err
	}
	return nil
}

func (soc *ScaledobjectClient) CreateOrUpdateAutoscaler(ctx context.Context, workload client.Object, labels map[string]string,
	max int32, min int32, targetCPUUtilization int32) (string, error) {
	scaledObj := kedaapi.ScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workload.GetName(),
			Namespace: workload.GetNamespace(),
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         workload.GetObjectKind().GroupVersionKind().GroupVersion().String(),
				Kind:               workload.GetObjectKind().GroupVersionKind().Kind,
				Name:               workload.GetName(),
				UID:                workload.GetUID(),
				Controller:         &trueBool,
				BlockOwnerDeletion: &trueBool,
			}},
			Labels: labels,
		},
		Spec: kedaapi.ScaledObjectSpec{
			ScaleTargetRef: &kedaapi.ScaleTarget{
				Name:       workload.GetName(),
				APIVersion: workload.GetObjectKind().GroupVersionKind().GroupVersion().String(),
				Kind:       workload.GetObjectKind().GroupVersionKind().Kind,
			},
			MinReplicaCount: &min,
			MaxReplicaCount: &max,
			Triggers:        soc.setScaleTriggers(targetCPUUtilization),
		},
	}

	result, err := controllerutil.CreateOrUpdate(ctx, soc.k8sClient, &scaledObj, func() error {

		scaledObj.Spec = kedaapi.ScaledObjectSpec{
			ScaleTargetRef: &kedaapi.ScaleTarget{
				Name:       workload.GetName(),
				APIVersion: workload.GetObjectKind().GroupVersionKind().GroupVersion().String(),
				Kind:       workload.GetObjectKind().GroupVersionKind().Kind,
			},
			MinReplicaCount: &min,
			MaxReplicaCount: &max,
			Triggers:        soc.setScaleTriggers(targetCPUUtilization),
		}

		return nil
	})
	if err != nil {
		return "", err
	}
	return string(result), nil
}

func (soc *ScaledobjectClient) setScaleTriggers(targetCPUUtilization int32) []kedaapi.ScaleTriggers {
	scaleTriggers := []kedaapi.ScaleTriggers{
		{
			Type: "cpu",
			Metadata: map[string]string{
				"type":  "Utilization",
				"value": fmt.Sprint(targetCPUUtilization),
			},
		},
	}
	if isEventScalerEnabled(soc.enableEventAutoscaler) {
		scaleTriggers = append(scaleTriggers, kedaapi.ScaleTriggers{
			Type: "scheduled-event",
			Metadata: map[string]string{
				"scalingStrategy": "scaleToMax",
			},
		})
	}
	return scaleTriggers
}

func isEventScalerEnabled(enableEventAutoscaler *bool) bool {
	if *enableEventAutoscaler {
		//TODO: define based on annotation
		return true
	}
	return false
}
