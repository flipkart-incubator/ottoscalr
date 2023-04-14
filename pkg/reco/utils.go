package reco

import (
	v1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewPolicyRecoCondition(conditionType v1alpha1.PolicyRecommendationConditionType, status metav1.ConditionStatus) *metav1.Condition {
	return &metav1.Condition{
		Type:               string(conditionType),
		Status:             status,
		ObservedGeneration: 0,
		LastTransitionTime: metav1.Now(),
		Reason:             "",
		Message:            "",
	}
}
