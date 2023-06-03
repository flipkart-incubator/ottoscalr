package controller

import (
	v1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (

	//Reason for RecoTaskProgress Condition
	RecoTaskQueued        = "RecoTaskQueued"
	RecoTaskQueuedMessage = "Workload Queued for Fresh HPA Recommendation"

	RecoTaskInProgress = "RecoTaskInProgress"

	RecoTaskRecommendationGenerated = "RecoTaskRecommendationGenerated"
	RecommendationGeneratedMessage  = "HPA Recommendation is generated"

	RecoTaskErrored = "RecoTaskErrored"

	//Reason for Initialized Condition
	PolicyRecommendationCRDCreated = "PolicyRecommendationCreated"
	InitializedMessage             = "PolicyRecommendation has been created"
)

func NewPolicyRecommendationCondition(condType v1alpha1.PolicyRecommendationConditionType, status metav1.ConditionStatus, reason, message string) *metav1.Condition {
	return &metav1.Condition{
		Type:               string(condType),
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

func CreatePolicyPatch(policyreco v1alpha1.PolicyRecommendation, condType v1alpha1.PolicyRecommendationConditionType, status metav1.ConditionStatus, reason, message string) *v1alpha1.PolicyRecommendation {
	recoTaskProgressCondition := NewPolicyRecommendationCondition(condType, status, reason, message)
	var conditions1 []metav1.Condition
	conditions1 = append(conditions1, *recoTaskProgressCondition)
	statusPatch := &v1alpha1.PolicyRecommendation{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.GroupVersion.String(),
			Kind:       "PolicyRecommendation",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      policyreco.Name,
			Namespace: policyreco.Namespace,
		},
		Status: v1alpha1.PolicyRecommendationStatus{
			Conditions: conditions1,
		},
	}
	return statusPatch
}
