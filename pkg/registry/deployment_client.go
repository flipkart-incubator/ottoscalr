package registry

import (
	"context"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
)

var DeploymentGVK = schema.GroupVersionKind{
	Group:   "apps",
	Version: "v1",
	Kind:    "Deployment",
}

type DeploymentClient struct {
	k8sClient client.Client
	gvk       schema.GroupVersionKind
}

func NewDeploymentClient(k8sClient client.Client) ObjectClient {
	return &DeploymentClient{
		k8sClient: k8sClient,
		gvk:       DeploymentGVK,
	}
}

func (dc *DeploymentClient) GetKind() string {
	return dc.gvk.Kind
}

func (dc *DeploymentClient) GetObjectType() client.Object {
	return &appsv1.Deployment{}
}

func (dc *DeploymentClient) GetObject(namespace string, name string) (client.Object, error) {
	deploymentObject := &appsv1.Deployment{}
	err := dc.k8sClient.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: name}, deploymentObject)
	if err != nil {
		return nil, err
	}
	return deploymentObject, nil

}

func (dc *DeploymentClient) GetMaxReplicaFromAnnotation(namespace string, name string) (int, error) {
	deploymentObject := &appsv1.Deployment{}
	if err := dc.k8sClient.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: name}, deploymentObject); err != nil {
		return 0, err
	}
	maxPodsAnnotation, ok := deploymentObject.GetAnnotations()["ottoscalr.io/max-pods"]
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

func (dc *DeploymentClient) GetContainerResourceLimits(namespace string, name string) (float64, error) {
	deploymentObject := &appsv1.Deployment{}
	if err := dc.k8sClient.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: name}, deploymentObject); err != nil {
		return 0, err
	}
	podTemplateSpec := deploymentObject.Spec.Template

	podList := &corev1.PodList{}

	if podTemplateSpec.Labels == nil {
		return 0, fmt.Errorf("no labels present on the workload to fetch pod")
	}

	labelSet := labels.Set(podTemplateSpec.Labels)
	selector := labels.SelectorFromSet(labelSet)

	if err := dc.k8sClient.List(context.Background(), podList, client.InNamespace(namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
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

func (dc *DeploymentClient) GetReplicaCount(namespace string, name string) (int, error) {
	deploymentObject := &appsv1.Deployment{}
	if err := dc.k8sClient.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: name}, deploymentObject); err != nil {
		return 0, err
	}
	if deploymentObject.Spec.Replicas != nil {
		return int(*deploymentObject.Spec.Replicas), nil
	}
	return 0, fmt.Errorf("replica count not present")
}

func (dc *DeploymentClient) Scale(namespace string, name string, replicas int32) error {
	var workloadPatch client.Object

	workloadPatch = &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       dc.GetKind(),
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	scale := &autoscalingv1.Scale{Spec: autoscalingv1.ScaleSpec{Replicas: replicas}}
	if err := dc.k8sClient.SubResource("scale").Update(context.Background(), workloadPatch, client.WithSubResourceBody(scale)); err != nil {
		return client.IgnoreNotFound(err)
	}
	return nil
}
