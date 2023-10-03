package registry

import (
	"context"
	"fmt"
	argov1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
)

var RolloutGVK = schema.GroupVersionKind{
	Group:   "argoproj.io",
	Version: "v1alpha1",
	Kind:    "Rollout",
}

type RolloutClient struct {
	k8sClient client.Client
	gvk       schema.GroupVersionKind
}

func (rc *RolloutClient) GetKind() string {
	return rc.gvk.Kind
}

func (rc *RolloutClient) GetObjectType() client.Object {
	return &argov1alpha1.Rollout{}
}

func (rc *RolloutClient) GetObject(namespace string, name string) (client.Object, error) {
	rolloutObject := &argov1alpha1.Rollout{}
	err := rc.k8sClient.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: name}, rolloutObject)
	if err != nil {
		return nil, err
	}
	return rolloutObject, nil

}

func (rc *RolloutClient) GetMaxReplicaFromAnnotation(namespace string, name string) (int, error) {
	rolloutObject := &argov1alpha1.Rollout{}
	if err := rc.k8sClient.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: name}, rolloutObject); err != nil {
		return 0, err
	}
	maxPodsAnnotation, ok := rolloutObject.GetAnnotations()["ottoscalr.io/max-pods"]
	if ok {
		var err error
		maxPods, err := strconv.Atoi(maxPodsAnnotation)
		if err != nil {
			return 0, fmt.Errorf("unable to convert maxPods from string to int: %s", err)
		}
		return maxPods, nil
	}
	return 0, fmt.Errorf("annotation not present")
}

func (rc *RolloutClient) GetContainerResourceLimits(namespace string, name string) (float64, error) {
	rolloutObject := &argov1alpha1.Rollout{}
	if err := rc.k8sClient.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: name}, rolloutObject); err != nil {
		return 0, err
	}
	podTemplateSpec := rolloutObject.Spec.Template

	podList := &corev1.PodList{}

	if podTemplateSpec.Labels == nil {
		return 0, fmt.Errorf("no labels present on the workload to fetch pod")
	}

	labelSet := labels.Set(podTemplateSpec.Labels)
	selector := labels.SelectorFromSet(labelSet)

	if err := rc.k8sClient.List(context.Background(), podList, client.InNamespace(namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
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

func (rc *RolloutClient) GetReplicaCount(namespace string, name string) (int, error) {
	rolloutObject := &argov1alpha1.Rollout{}
	if err := rc.k8sClient.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: name}, rolloutObject); err != nil {
		return 0, err
	}
	return int(*rolloutObject.Spec.Replicas), nil
}

func (rc *RolloutClient) Scale(namespace string, name string, replicas int32) error {
	var workloadPatch client.Object

	workloadPatch = &argov1alpha1.Rollout{
		TypeMeta: metav1.TypeMeta{
			Kind:       rc.GetKind(),
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	scale := &autoscalingv1.Scale{Spec: autoscalingv1.ScaleSpec{Replicas: replicas}}
	if err := rc.k8sClient.SubResource("scale").Update(context.Background(), workloadPatch, client.WithSubResourceBody(scale)); err != nil {
		return client.IgnoreNotFound(err)
	}
	return nil
}
