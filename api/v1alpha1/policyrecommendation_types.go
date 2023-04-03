/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type Policy struct {
	ID        string `json:"id"`
	RiskIndex string `json:"riskIndex"`
}

// PolicyRecommendationSpec defines the desired state of PolicyRecommendation
type PolicyRecommendationSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of PolicyRecommendation. Edit policyrecommendation_types.go to remove/update

	WorkloadSpec           WorkloadSpec     `json:"workload"`
	TargetHPAConfiguration HPAConfiguration `json:"targetHPAConfig"`
	Policy                 Policy           `json:"policy"`
	GeneratedAt            metav1.Time      `json:"generatedAt"`
	QueuedForExecution     bool             `json:"queuedForExecution"`
	QueuedForExecutionAt   metav1.Time      `json:"queuedForExecutionAt,omitempty"`
}

type WorkloadSpec struct {
	metav1.TypeMeta `json:","`
	Name            string `json:"name"`
}

type HPAConfiguration struct {
	Min               int `json:"min"`
	Max               int `json:"max"`
	TargetMetricValue int `json:"targetMetricValue"`
}

// PolicyRecommendationStatus defines the observed state of PolicyRecommendation
type PolicyRecommendationStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// PolicyRecommendation is the Schema for the policyrecommendations API
type PolicyRecommendation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PolicyRecommendationSpec   `json:"spec,omitempty"`
	Status PolicyRecommendationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PolicyRecommendationList contains a list of PolicyRecommendation
type PolicyRecommendationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PolicyRecommendation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PolicyRecommendation{}, &PolicyRecommendationList{})
}
