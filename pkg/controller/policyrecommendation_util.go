package controller

import (
	v1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (

	//Reason for RecoTaskProgress Condition
	RecoTaskExecutionDone        = "RecoTaskExecutionDone"
	RecoTaskExecutionDoneMessage = "The Recommendation Workflow execution has been completed"

	RecoTaskRecommendationGenerated = "RecoTaskRecommendationGenerated"
	RecommendationGeneratedMessage  = "HPA Recommendation is generated"

	RecoTaskInProgress        = "RecoTaskInProgress"
	RecoTaskInProgressMessage = "Recommendation Workflow execution is in progress"

	RecoTaskErrored        = "RecoTaskErrored"
	EmptyRecoConfigMessage = "Empty recommendation config could be due to lack of utilization data points or non availability of pod ready time"
	EmptyHPAConfigMessage  = "HPA config to be applied is empty"

	//Reason for Initialized Condition
	PolicyRecommendationCreated = "PolicyRecommendationCreated"
	InitializedMessage          = "PolicyRecommendation has been created"

	//Reason for TargetRecoAchieved Condition
	PolicyRecommendationAtTargetReco    = "PolicyRecommendationAtTargetReco"
	PolicyRecommendationNotAtTargetReco = "PolicyRecommendationNotAtTargetReco"
	TargetRecoAchievedSuccessMessage    = "Target Recommendation has been achieved"
	TargetRecoAchievedFailureMessage    = "Target Recommendation has not been achieved yet"
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

func CreatePolicyPatch(policyreco v1alpha1.PolicyRecommendation, conditions []metav1.Condition, condType v1alpha1.PolicyRecommendationConditionType, status metav1.ConditionStatus, reason, message string) (*v1alpha1.PolicyRecommendation, []metav1.Condition) {
	newCondition := NewPolicyRecommendationCondition(condType, status, reason, message)
	updatedConditions := SetConditions(conditions, *newCondition)
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
			Conditions: updatedConditions,
		},
	}
	return statusPatch, updatedConditions
}

func SetConditions(conditions []metav1.Condition, newCondition metav1.Condition) []metav1.Condition {
	var newConditions []metav1.Condition
	for _, c := range conditions {
		if c.Type != newCondition.Type {
			newConditions = append(newConditions, c)
		}
	}
	newConditions = append(newConditions, newCondition)
	return newConditions
}
