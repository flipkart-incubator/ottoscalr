package reco

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	rolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	OttoscalrMaxScaleUpPodsAnnotation             = "ottoscalr.io/max-scale-up-pods"
	OttoscalrMaxScaleUpPodsDefaultValuePercentage = 15
	OttoscalrEnableScaleUpBehaviourAnnotation     = "ottoscalr.io/enable-scale-up-behaviour"
)

func GetMaxScaleUpPods(k8sClient client.Client, namespace string, objectKind string, objectName string, maxReplicas int) (int, error) {
	var obj client.Object
	switch objectKind {
	case "Deployment":
		obj = &appsv1.Deployment{}
	case "Rollout":
		obj = &rolloutv1alpha1.Rollout{}
	default:
		return 0, fmt.Errorf("unsupported objectKind: %s", objectKind)
	}

	if err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: objectName}, obj); err != nil {
		return 0, err
	}
	var maxScaleUpPods int
	maxScaleUpPodAnnotation, ok := obj.GetAnnotations()[OttoscalrMaxScaleUpPodsAnnotation]
	if ok {
		var err error
		maxScaleUpPods, err = strconv.Atoi(maxScaleUpPodAnnotation)
		if err != nil {
			return 0, fmt.Errorf("unable to convert maxScaleUpPods from string to int: %s", err)
		}
		return maxScaleUpPods, nil
	}
	defaultMaxScaleUpPods := int(math.Ceil(float64(maxReplicas) * (float64(OttoscalrMaxScaleUpPodsDefaultValuePercentage) / 100)))

	var maxSurge int
	var maxUnavailable int
	switch v := obj.(type) {
	case *appsv1.Deployment:
		if v.Spec.Strategy.RollingUpdate != nil && v.Spec.Strategy.RollingUpdate.MaxUnavailable != nil && v.Spec.Strategy.RollingUpdate.MaxSurge != nil {

			if v.Spec.Strategy.RollingUpdate.MaxUnavailable.StrVal != "" && strings.HasSuffix(v.Spec.Strategy.RollingUpdate.MaxUnavailable.StrVal, "%") {
				percentageValue, err := strconv.Atoi(strings.TrimSuffix(v.Spec.Strategy.RollingUpdate.MaxUnavailable.StrVal, "%"))
				if err == nil {
					maxUnavailable = maxReplicas * percentageValue / 100
				}
			} else {
				maxUnavailable = int(v.Spec.Strategy.RollingUpdate.MaxUnavailable.IntVal)
			}

			if v.Spec.Strategy.RollingUpdate.MaxSurge.StrVal != "" && strings.HasSuffix(v.Spec.Strategy.RollingUpdate.MaxSurge.StrVal, "%") {
				percentageValue, err := strconv.Atoi(strings.TrimSuffix(v.Spec.Strategy.RollingUpdate.MaxSurge.StrVal, "%"))
				if err == nil {
					maxSurge = maxReplicas * percentageValue / 100
				}
			} else {
				maxSurge = int(v.Spec.Strategy.RollingUpdate.MaxSurge.IntVal)
			}
		}
	case *rolloutv1alpha1.Rollout:
		if v.Spec.Strategy.Canary != nil && v.Spec.Strategy.Canary.MaxUnavailable != nil && v.Spec.Strategy.Canary.MaxSurge != nil {

			if v.Spec.Strategy.Canary.MaxUnavailable.StrVal != "" && strings.HasSuffix(v.Spec.Strategy.Canary.MaxUnavailable.StrVal, "%") {
				percentageValue, err := strconv.Atoi(strings.TrimSuffix(v.Spec.Strategy.Canary.MaxUnavailable.StrVal, "%"))
				if err == nil {
					maxUnavailable = maxReplicas * percentageValue / 100
				}
			} else {
				maxUnavailable = int(v.Spec.Strategy.Canary.MaxUnavailable.IntVal)
			}

			if v.Spec.Strategy.Canary.MaxSurge.StrVal != "" && strings.HasSuffix(v.Spec.Strategy.Canary.MaxSurge.StrVal, "%") {
				percentageValue, err := strconv.Atoi(strings.TrimSuffix(v.Spec.Strategy.Canary.MaxSurge.StrVal, "%"))
				if err == nil {
					maxSurge = maxReplicas * percentageValue / 100
				}
			} else {
				maxSurge = int(v.Spec.Strategy.Canary.MaxSurge.IntVal)
			}
		}
	default:
		return 0, fmt.Errorf("unsupported object type")
	}
	maxScaleUpPods = int(math.Max(float64(maxSurge), float64(maxUnavailable)))

	return int(math.Max(float64(maxScaleUpPods), float64(defaultMaxScaleUpPods))), nil
}

func EnableScaleUpBehaviourForHPA(k8sClient client.Client, namespace string, objectKind string, objectName string) bool {
	var obj client.Object
	switch objectKind {
	case "Deployment":
		obj = &appsv1.Deployment{}
	case "Rollout":
		obj = &rolloutv1alpha1.Rollout{}
	default:
		return false
	}

	if err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: objectName}, obj); err != nil {
		return false
	}
	if v, ok := obj.GetAnnotations()[OttoscalrEnableScaleUpBehaviourAnnotation]; ok {
		if allow, _ := strconv.ParseBool(v); allow {
			return true
		}
	}

	return false
}
