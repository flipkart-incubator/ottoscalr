package controller

import (
	"context"
	"encoding/json"
	"fmt"
	argov1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	v1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	kedaapi "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"strconv"
	"time"
)

var _ = Describe("Test ScaledObject enforcer", func() {

	const (
		HPAEnforcerPolicyRecoName      = "test-deployment-asoif"
		HPAEnforcerPolicyRecoNamespace = "default"

		timeout            = time.Second * 10
		interval           = time.Millisecond * 250
		policyAge          = 1 * time.Second
		argoRolloutVersion = "argoproj.io/v1alpha1"
		argoRolloutsKind   = "Rollout"
	)

	BeforeEach(func() {})
	AfterEach(func() {})
	/*
		When("A policy reco is generated in blacklist mode", func() {
			var scaledObject *kedaapi.ScaledObject
			var policyReco *v1alpha1.PolicyRecommendation
			var deployment *appsv1.Deployment
			var rollout *argov1alpha1.Rollout
			BeforeEach(func() {
				*hpaEnforcerIsDryRun = false
				*hpaEnforcerExcludedNamespaces = nil
				*hpaEnforcerIncludedNamespaces = nil
				*whitelistMode = falseBool
				policyReco = nil
				scaledObject = nil
				deployment = nil
				rollout = nil
			})
			AfterEach(func() {
				if deployment != nil && len(deployment.Name) != 0 {
					Expect(k8sClient.Delete(context.TODO(), deployment)).Should(Succeed())
				}
				if rollout != nil && len(rollout.Name) != 0 {
					Expect(k8sClient.Delete(context.TODO(), rollout)).Should(Succeed())
				}
				Eventually(func() bool {
					err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, policyReco)
					if err != nil {
						return true
					}
					Expect(k8sClient.Delete(context.TODO(), policyReco)).Should(Succeed())
					return true
				}, timeout, interval).Should(BeTrue())
				Eventually(func() bool {
					err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject)
					if err != nil {
						return true
					}
					Expect(k8sClient.Delete(context.TODO(), scaledObject)).Should(Succeed())
					return true
				}, timeout, interval).Should(BeTrue())
			})

			It("Should create a ScaledObject for a Deployment", func() {
				policyReco = &v1alpha1.PolicyRecommendation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: v1alpha1.PolicyRecommendationSpec{
						WorkloadMeta: v1alpha1.WorkloadMeta{
							TypeMeta: metav1.TypeMeta{
								Kind:       "Deployment",
								APIVersion: "apps/v1",
							},
							Name: HPAEnforcerPolicyRecoName,
						},
						TargetHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               60,
							Max:               100,
							TargetMetricValue: 50,
						},
						CurrentHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               20,
							Max:               100,
							TargetMetricValue: 40,
						},
						Policy:             "random",
						QueuedForExecution: &falseBool,
					},
				}
				Expect(k8sClient.Create(context.TODO(), policyReco)).To(Succeed())
				updatedPolicyReco := &v1alpha1.PolicyRecommendation{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)
				replicas := int32(10)
				deployment = &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: &replicas,
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test-app",
							},
						},
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "test-app",
								},
							},
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  "test-container",
										Image: "nginx:1.17.5",
									},
								},
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), deployment)).To(Succeed())
				updatedPolicyReco.Status = v1alpha1.PolicyRecommendationStatus{
					Conditions: []metav1.Condition{
						{
							Type:               string(v1alpha1.Initialized),
							Status:             metav1.ConditionTrue,
							LastTransitionTime: metav1.Now(),
							Reason:             PolicyRecommendationCreated,
							Message:            InitializedMessage,
						},
						{
							Type:               string(v1alpha1.RecoTaskProgress),
							Status:             metav1.ConditionFalse,
							Reason:             RecoTaskRecommendationGenerated,
							Message:            RecommendationGeneratedMessage,
							LastTransitionTime: metav1.Now(),
						},
					}}
				Expect(k8sClient.Status().Update(context.TODO(), updatedPolicyReco)).To(Succeed())
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ = json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)

				scaledObject = &kedaapi.ScaledObject{}
				Eventually(k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject), timeout, interval).Should(Succeed())
				scaledObjectString, _ := json.MarshalIndent(scaledObject, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", scaledObjectString)
				fmt.Println("step 20")
				Expect(scaledObject.OwnerReferences[0].Name).Should(Equal(deployment.Name))
				Expect(scaledObject.OwnerReferences[0].Kind).Should(Equal("Deployment"))
				Expect(scaledObject.OwnerReferences[0].APIVersion).Should(Equal("apps/v1"))
				Expect(scaledObject.Spec.ScaleTargetRef.Name).Should(Equal(deployment.Name))
				Expect(scaledObject.Spec.ScaleTargetRef.Kind).Should(Equal("Deployment"))
				Expect(scaledObject.Spec.ScaleTargetRef.APIVersion).Should(Equal("apps/v1"))
				Expect(int(*scaledObject.Spec.MaxReplicaCount)).Should(Equal(policyReco.Spec.CurrentHPAConfiguration.Max))
				Expect(int(*scaledObject.Spec.MinReplicaCount)).Should(Equal(policyReco.Spec.CurrentHPAConfiguration.Min))
				Expect(len(scaledObject.Spec.Triggers)).Should(Equal(2))
				Expect(scaledObject.Spec.Triggers[0].Type).Should(Equal("cpu"))
				n, _ := strconv.Atoi(scaledObject.Spec.Triggers[0].Metadata["value"])
				Expect(n).Should(Equal((policyReco.Spec.CurrentHPAConfiguration.TargetMetricValue)))

			})

			It("Should not create a ScaledObject for a Deployment", func() {
				policyReco = &v1alpha1.PolicyRecommendation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: v1alpha1.PolicyRecommendationSpec{
						WorkloadMeta: v1alpha1.WorkloadMeta{
							TypeMeta: metav1.TypeMeta{
								Kind:       "Deployment",
								APIVersion: "apps/v1",
							},
							Name: HPAEnforcerPolicyRecoName,
						},
						TargetHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               60,
							Max:               100,
							TargetMetricValue: 50,
						},
						CurrentHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               20,
							Max:               100,
							TargetMetricValue: 40,
						},
						Policy:             "random",
						QueuedForExecution: &falseBool,
					},
				}
				Expect(k8sClient.Create(context.TODO(), policyReco)).To(Succeed())
				updatedPolicyReco := &v1alpha1.PolicyRecommendation{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)
				replicas := int32(10)
				deployment = &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:        HPAEnforcerPolicyRecoName,
						Namespace:   HPAEnforcerPolicyRecoNamespace,
						Annotations: map[string]string{"ottoscalr.io/skip-hpa-enforcement": "true"},
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: &replicas,
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test-app",
							},
						},
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "test-app",
								},
							},
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  "test-container",
										Image: "nginx:1.17.5",
									},
								},
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), deployment)).To(Succeed())
				updatedPolicyReco.Status = v1alpha1.PolicyRecommendationStatus{
					Conditions: []metav1.Condition{
						{
							Type:               string(v1alpha1.Initialized),
							Status:             metav1.ConditionTrue,
							LastTransitionTime: metav1.Now(),
							Reason:             PolicyRecommendationCreated,
							Message:            InitializedMessage,
						},
						{
							Type:               string(v1alpha1.RecoTaskProgress),
							Status:             metav1.ConditionFalse,
							Reason:             RecoTaskRecommendationGenerated,
							Message:            RecommendationGeneratedMessage,
							LastTransitionTime: metav1.Now(),
						},
					}}
				Expect(k8sClient.Status().Update(context.TODO(), updatedPolicyReco)).To(Succeed())
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ = json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)

				scaledObject = &kedaapi.ScaledObject{}
				Eventually(func() bool {
					err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject)
					if err != nil {
						return true
					}
					return false
				}, timeout, interval).Should(BeTrue())
			})

			It("Should create a ScaledObject for a Rollout", func() {
				fmt.Fprintf(GinkgoWriter, "Step test pr %v\n", policyReco)
				fmt.Fprintf(GinkgoWriter, "Step test d %v\n", deployment)
				fmt.Fprintf(GinkgoWriter, "Step test r %v\n", rollout)
				fmt.Fprintf(GinkgoWriter, "Step test so %v\n", scaledObject)
				policyReco = &v1alpha1.PolicyRecommendation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: v1alpha1.PolicyRecommendationSpec{
						WorkloadMeta: v1alpha1.WorkloadMeta{
							TypeMeta: metav1.TypeMeta{
								Kind:       argoRolloutsKind,
								APIVersion: argoRolloutVersion,
							},
							Name: HPAEnforcerPolicyRecoName,
						},
						TargetHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               60,
							Max:               100,
							TargetMetricValue: 50,
						},
						CurrentHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               20,
							Max:               100,
							TargetMetricValue: 40,
						},
						Policy:             "random",
						QueuedForExecution: &falseBool,
					},
				}
				Expect(k8sClient.Create(context.TODO(), policyReco)).To(Succeed())
				updatedPolicyReco := &v1alpha1.PolicyRecommendation{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)
				fmt.Println("step 1")
				replicas := int32(10)
				rollout = &argov1alpha1.Rollout{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: argov1alpha1.RolloutSpec{
						Replicas: &replicas,
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test-app",
							},
						},
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "test-app",
								},
							},
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  "test-container",
										Image: "nginx:1.17.5",
									},
								},
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), rollout)).To(Succeed())
				updatedPolicyReco.Status = v1alpha1.PolicyRecommendationStatus{
					Conditions: []metav1.Condition{
						{
							Type:               string(v1alpha1.Initialized),
							Status:             metav1.ConditionTrue,
							LastTransitionTime: metav1.Now(),
							Reason:             PolicyRecommendationCreated,
							Message:            InitializedMessage,
						},
						{
							Type:               string(v1alpha1.RecoTaskProgress),
							Status:             metav1.ConditionFalse,
							Reason:             RecoTaskRecommendationGenerated,
							Message:            RecommendationGeneratedMessage,
							LastTransitionTime: metav1.Now(),
						},
					}}
				Expect(k8sClient.Status().Update(context.TODO(), updatedPolicyReco)).To(Succeed())
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ = json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)

				scaledObject = &kedaapi.ScaledObject{}
				Eventually(k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject), timeout, interval).Should(Succeed())
				scaledObjectString, _ := json.MarshalIndent(scaledObject, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", scaledObjectString)
				fmt.Println("step 22")
				Expect(scaledObject.OwnerReferences[0].Name).Should(Equal(rollout.Name))
				Expect(scaledObject.OwnerReferences[0].Kind).Should(Equal("Rollout"))
				Expect(scaledObject.OwnerReferences[0].APIVersion).Should(Equal("argoproj.io/v1alpha1"))
				Expect(scaledObject.Spec.ScaleTargetRef.Name).Should(Equal(rollout.Name))
				Expect(scaledObject.Spec.ScaleTargetRef.Kind).Should(Equal("Rollout"))
				Expect(scaledObject.Spec.ScaleTargetRef.APIVersion).Should(Equal("argoproj.io/v1alpha1"))
				Expect(int(*scaledObject.Spec.MaxReplicaCount)).Should(Equal(policyReco.Spec.CurrentHPAConfiguration.Max))
				Expect(int(*scaledObject.Spec.MinReplicaCount)).Should(Equal(policyReco.Spec.CurrentHPAConfiguration.Min))
				Expect(len(scaledObject.Spec.Triggers)).Should(Equal(2))
				Expect(scaledObject.Spec.Triggers[0].Type).Should(Equal("cpu"))
				n, _ := strconv.Atoi(scaledObject.Spec.Triggers[0].Metadata["value"])
				Expect(n).Should(Equal((policyReco.Spec.CurrentHPAConfiguration.TargetMetricValue)))

			})

			It("Should not create a ScaledObject for a Rollout", func() {

				policyReco = &v1alpha1.PolicyRecommendation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: v1alpha1.PolicyRecommendationSpec{
						WorkloadMeta: v1alpha1.WorkloadMeta{
							TypeMeta: metav1.TypeMeta{
								Kind:       argoRolloutsKind,
								APIVersion: argoRolloutVersion,
							},
							Name: HPAEnforcerPolicyRecoName,
						},
						TargetHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               60,
							Max:               100,
							TargetMetricValue: 50,
						},
						CurrentHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               20,
							Max:               100,
							TargetMetricValue: 40,
						},
						Policy:             "random",
						QueuedForExecution: &falseBool,
					},
					Status: v1alpha1.PolicyRecommendationStatus{
						Conditions: []metav1.Condition{
							{
								Type:    string(v1alpha1.Initialized),
								Status:  metav1.ConditionTrue,
								Reason:  PolicyRecommendationCreated,
								Message: InitializedMessage,
							},
							{
								Type:    string(v1alpha1.RecoTaskProgress),
								Status:  metav1.ConditionFalse,
								Reason:  RecoTaskRecommendationGenerated,
								Message: RecommendationGeneratedMessage,
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), policyReco)).To(Succeed())
				updatedPolicyReco := &v1alpha1.PolicyRecommendation{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)
				replicas := int32(10)
				rollout = &argov1alpha1.Rollout{
					ObjectMeta: metav1.ObjectMeta{
						Name:        HPAEnforcerPolicyRecoName,
						Namespace:   HPAEnforcerPolicyRecoNamespace,
						Annotations: map[string]string{"ottoscalr.io/skip-hpa-enforcement": "true"},
					},
					Spec: argov1alpha1.RolloutSpec{
						Replicas: &replicas,
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test-app",
							},
						},
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "test-app",
								},
							},
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  "test-container",
										Image: "nginx:1.17.5",
									},
								},
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), rollout)).To(Succeed())
				updatedPolicyReco.Status = v1alpha1.PolicyRecommendationStatus{
					Conditions: []metav1.Condition{
						{
							Type:               string(v1alpha1.Initialized),
							Status:             metav1.ConditionTrue,
							LastTransitionTime: metav1.Now(),
							Reason:             PolicyRecommendationCreated,
							Message:            InitializedMessage,
						},
						{
							Type:               string(v1alpha1.RecoTaskProgress),
							Status:             metav1.ConditionFalse,
							Reason:             RecoTaskRecommendationGenerated,
							Message:            RecommendationGeneratedMessage,
							LastTransitionTime: metav1.Now(),
						},
					}}
				Expect(k8sClient.Status().Update(context.TODO(), updatedPolicyReco)).To(Succeed())
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ = json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)

				scaledObject = &kedaapi.ScaledObject{}
				Eventually(k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject), timeout, interval).ShouldNot(Succeed())
			})
		})

		When("A policy reco is generated in dry run mode in blacklist mode", func() {
			var scaledObject *kedaapi.ScaledObject
			var policyReco *v1alpha1.PolicyRecommendation
			var deployment *appsv1.Deployment
			var rollout *argov1alpha1.Rollout
			BeforeEach(func() {
				*hpaEnforcerIsDryRun = true
				*hpaEnforcerExcludedNamespaces = nil
				*hpaEnforcerIncludedNamespaces = nil
				*whitelistMode = false
				policyReco = nil
				scaledObject = nil
				deployment = nil
				rollout = nil
			})
			AfterEach(func() {
				if deployment != nil && len(deployment.Name) != 0 {
					Expect(k8sClient.Delete(context.TODO(), deployment)).Should(Succeed())
				}
				if rollout != nil && len(rollout.Name) != 0 {
					Expect(k8sClient.Delete(context.TODO(), rollout)).Should(Succeed())
				}
				Eventually(func() bool {
					err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, policyReco)
					if err != nil {
						return true
					}
					Expect(k8sClient.Delete(context.TODO(), policyReco)).Should(Succeed())
					return true
				}, timeout, interval).Should(BeTrue())
				Eventually(func() bool {
					err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject)
					if err != nil {
						return true
					}
					Expect(k8sClient.Delete(context.TODO(), scaledObject)).Should(Succeed())
					return true
				}, timeout, interval).Should(BeTrue())
			})

			It("Should not create a ScaledObject for a Deployment in dryrun mode", func() {
				policyReco = &v1alpha1.PolicyRecommendation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: v1alpha1.PolicyRecommendationSpec{
						WorkloadMeta: v1alpha1.WorkloadMeta{
							TypeMeta: metav1.TypeMeta{
								Kind:       "Deployment",
								APIVersion: "apps/v1",
							},
							Name: HPAEnforcerPolicyRecoName,
						},
						TargetHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               60,
							Max:               100,
							TargetMetricValue: 50,
						},
						CurrentHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               20,
							Max:               100,
							TargetMetricValue: 40,
						},
						Policy:             "random",
						QueuedForExecution: &falseBool,
					},
				}
				Expect(k8sClient.Create(context.TODO(), policyReco)).Should(Succeed())
				updatedPolicyReco := &v1alpha1.PolicyRecommendation{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "Policy reco after creation: %s\n", updatedPolicyRecoString)
				replicas := int32(10)
				deployment = &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: &replicas,
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test-app",
							},
						},
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "test-app",
								},
							},
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  "test-container",
										Image: "nginx:1.17.5",
									},
								},
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), deployment)).To(Succeed())
				updatedPolicyReco.Status = v1alpha1.PolicyRecommendationStatus{
					Conditions: []metav1.Condition{
						{
							Type:               string(v1alpha1.Initialized),
							Status:             metav1.ConditionTrue,
							LastTransitionTime: metav1.Now(),
							Reason:             PolicyRecommendationCreated,
							Message:            InitializedMessage,
						},
						{
							Type:               string(v1alpha1.RecoTaskProgress),
							Status:             metav1.ConditionFalse,
							Reason:             RecoTaskRecommendationGenerated,
							Message:            RecommendationGeneratedMessage,
							LastTransitionTime: metav1.Now(),
						},
					}}
				Expect(k8sClient.Status().Update(context.TODO(), updatedPolicyReco)).To(Succeed())
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ = json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)

				scaledObject = &kedaapi.ScaledObject{}
				Eventually(k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject), timeout, interval).ShouldNot(Succeed())
			})

			It("Should not create a ScaledObject for a Deploymentin dryrun mode", func() {
				policyReco = &v1alpha1.PolicyRecommendation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: v1alpha1.PolicyRecommendationSpec{
						WorkloadMeta: v1alpha1.WorkloadMeta{
							TypeMeta: metav1.TypeMeta{
								Kind:       "Deployment",
								APIVersion: "apps/v1",
							},
							Name: HPAEnforcerPolicyRecoName,
						},
						TargetHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               60,
							Max:               100,
							TargetMetricValue: 50,
						},
						CurrentHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               20,
							Max:               100,
							TargetMetricValue: 40,
						},
						Policy:             "random",
						QueuedForExecution: &falseBool,
					},
					Status: v1alpha1.PolicyRecommendationStatus{
						Conditions: []metav1.Condition{
							{
								Type:    string(v1alpha1.Initialized),
								Status:  metav1.ConditionTrue,
								Reason:  PolicyRecommendationCreated,
								Message: InitializedMessage,
							},
							{
								Type:    string(v1alpha1.RecoTaskProgress),
								Status:  metav1.ConditionFalse,
								Reason:  RecoTaskRecommendationGenerated,
								Message: RecommendationGeneratedMessage,
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), policyReco)).To(Succeed())
				updatedPolicyReco := &v1alpha1.PolicyRecommendation{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)
				replicas := int32(10)
				deployment = &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:        HPAEnforcerPolicyRecoName,
						Namespace:   HPAEnforcerPolicyRecoNamespace,
						Annotations: map[string]string{"ottoscalr.io/skip-hpa-enforcement": "true"},
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: &replicas,
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test-app",
							},
						},
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "test-app",
								},
							},
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  "test-container",
										Image: "nginx:1.17.5",
									},
								},
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), deployment)).To(Succeed())
				updatedPolicyReco.Status = v1alpha1.PolicyRecommendationStatus{
					Conditions: []metav1.Condition{
						{
							Type:               string(v1alpha1.Initialized),
							Status:             metav1.ConditionTrue,
							LastTransitionTime: metav1.Now(),
							Reason:             PolicyRecommendationCreated,
							Message:            InitializedMessage,
						},
						{
							Type:               string(v1alpha1.RecoTaskProgress),
							Status:             metav1.ConditionFalse,
							Reason:             RecoTaskRecommendationGenerated,
							Message:            RecommendationGeneratedMessage,
							LastTransitionTime: metav1.Now(),
						},
					}}
				Expect(k8sClient.Status().Update(context.TODO(), updatedPolicyReco)).To(Succeed())
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ = json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)

				scaledObject = &kedaapi.ScaledObject{}
				Eventually(k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject), timeout, interval).ShouldNot(Succeed())
				scaledObjectString, _ := json.MarshalIndent(scaledObject, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", scaledObjectString)
				fmt.Println("Step 21")

			})

			It("Should not create a ScaledObject for a Rollout in dryrun mode", func() {

				policyReco = &v1alpha1.PolicyRecommendation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: v1alpha1.PolicyRecommendationSpec{
						WorkloadMeta: v1alpha1.WorkloadMeta{
							TypeMeta: metav1.TypeMeta{
								Kind:       argoRolloutsKind,
								APIVersion: argoRolloutVersion,
							},
							Name: HPAEnforcerPolicyRecoName,
						},
						TargetHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               60,
							Max:               100,
							TargetMetricValue: 50,
						},
						CurrentHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               20,
							Max:               100,
							TargetMetricValue: 40,
						},
						Policy:             "random",
						QueuedForExecution: &falseBool,
					},
					Status: v1alpha1.PolicyRecommendationStatus{
						Conditions: []metav1.Condition{
							{
								Type:    string(v1alpha1.Initialized),
								Status:  metav1.ConditionTrue,
								Reason:  PolicyRecommendationCreated,
								Message: InitializedMessage,
							},
							{
								Type:    string(v1alpha1.RecoTaskProgress),
								Status:  metav1.ConditionFalse,
								Reason:  RecoTaskRecommendationGenerated,
								Message: RecommendationGeneratedMessage,
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), policyReco)).To(Succeed())
				updatedPolicyReco := &v1alpha1.PolicyRecommendation{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)
				fmt.Println("step 1")
				replicas := int32(10)
				rollout = &argov1alpha1.Rollout{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: argov1alpha1.RolloutSpec{
						Replicas: &replicas,
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test-app",
							},
						},
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "test-app",
								},
							},
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  "test-container",
										Image: "nginx:1.17.5",
									},
								},
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), rollout)).To(Succeed())
				updatedPolicyReco.Status = v1alpha1.PolicyRecommendationStatus{
					Conditions: []metav1.Condition{
						{
							Type:               string(v1alpha1.Initialized),
							Status:             metav1.ConditionTrue,
							LastTransitionTime: metav1.Now(),
							Reason:             PolicyRecommendationCreated,
							Message:            InitializedMessage,
						},
						{
							Type:               string(v1alpha1.RecoTaskProgress),
							Status:             metav1.ConditionFalse,
							Reason:             RecoTaskRecommendationGenerated,
							Message:            RecommendationGeneratedMessage,
							LastTransitionTime: metav1.Now(),
						},
					}}
				Expect(k8sClient.Status().Update(context.TODO(), updatedPolicyReco)).To(Succeed())
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ = json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)

				scaledObject = &kedaapi.ScaledObject{}
				Eventually(k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject), timeout, interval).ShouldNot(Succeed())
				scaledObjectString, _ := json.MarshalIndent(scaledObject, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", scaledObjectString)

			})

			It("Should not create a ScaledObject for a Rollout in dryrun mode", func() {

				policyReco = &v1alpha1.PolicyRecommendation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: v1alpha1.PolicyRecommendationSpec{
						WorkloadMeta: v1alpha1.WorkloadMeta{
							TypeMeta: metav1.TypeMeta{
								Kind:       argoRolloutsKind,
								APIVersion: argoRolloutVersion,
							},
							Name: HPAEnforcerPolicyRecoName,
						},
						TargetHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               60,
							Max:               100,
							TargetMetricValue: 50,
						},
						CurrentHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               20,
							Max:               100,
							TargetMetricValue: 40,
						},
						Policy:             "random",
						QueuedForExecution: &falseBool,
					},
					Status: v1alpha1.PolicyRecommendationStatus{
						Conditions: []metav1.Condition{
							{
								Type:    string(v1alpha1.Initialized),
								Status:  metav1.ConditionTrue,
								Reason:  PolicyRecommendationCreated,
								Message: InitializedMessage,
							},
							{
								Type:    string(v1alpha1.RecoTaskProgress),
								Status:  metav1.ConditionFalse,
								Reason:  RecoTaskRecommendationGenerated,
								Message: RecommendationGeneratedMessage,
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), policyReco)).To(Succeed())
				updatedPolicyReco := &v1alpha1.PolicyRecommendation{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)
				replicas := int32(10)
				rollout = &argov1alpha1.Rollout{
					ObjectMeta: metav1.ObjectMeta{
						Name:        HPAEnforcerPolicyRecoName,
						Namespace:   HPAEnforcerPolicyRecoNamespace,
						Annotations: map[string]string{"ottoscalr.io/skip-hpa-enforcement": "true"},
					},
					Spec: argov1alpha1.RolloutSpec{
						Replicas: &replicas,
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test-app",
							},
						},
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "test-app",
								},
							},
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  "test-container",
										Image: "nginx:1.17.5",
									},
								},
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), rollout)).To(Succeed())
				updatedPolicyReco.Status = v1alpha1.PolicyRecommendationStatus{
					Conditions: []metav1.Condition{
						{
							Type:               string(v1alpha1.Initialized),
							Status:             metav1.ConditionTrue,
							LastTransitionTime: metav1.Now(),
							Reason:             PolicyRecommendationCreated,
							Message:            InitializedMessage,
						},
						{
							Type:               string(v1alpha1.RecoTaskProgress),
							Status:             metav1.ConditionFalse,
							Reason:             RecoTaskRecommendationGenerated,
							Message:            RecommendationGeneratedMessage,
							LastTransitionTime: metav1.Now(),
						},
					}}
				Expect(k8sClient.Status().Update(context.TODO(), updatedPolicyReco)).To(Succeed())
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ = json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)

				scaledObject = &kedaapi.ScaledObject{}
				Eventually(k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject), timeout, interval).ShouldNot(Succeed())
			})

		})

		When("A policy reco is generated in whitelist mode", func() {
			var scaledObject *kedaapi.ScaledObject
			var policyReco *v1alpha1.PolicyRecommendation
			var deployment *appsv1.Deployment
			var rollout *argov1alpha1.Rollout
			BeforeEach(func() {
				*hpaEnforcerIsDryRun = false
				*hpaEnforcerExcludedNamespaces = nil
				*hpaEnforcerIncludedNamespaces = nil
				*whitelistMode = true
				policyReco = nil
				scaledObject = nil
				deployment = nil
				rollout = nil
			})
			AfterEach(func() {
				if deployment != nil && len(deployment.Name) != 0 {
					Expect(k8sClient.Delete(context.TODO(), deployment)).Should(Succeed())
				}
				if rollout != nil && len(rollout.Name) != 0 {
					Expect(k8sClient.Delete(context.TODO(), rollout)).Should(Succeed())
				}
				Eventually(func() bool {
					err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, policyReco)
					if err != nil {
						return true
					}
					Expect(k8sClient.Delete(context.TODO(), policyReco)).Should(Succeed())
					return true
				}, timeout, interval).Should(BeTrue())
				Eventually(func() bool {
					err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject)
					if err != nil {
						return true
					}
					Expect(k8sClient.Delete(context.TODO(), scaledObject)).Should(Succeed())
					return true
				}, timeout, interval).Should(BeTrue())
			})

			It("Should create a ScaledObject for a Deployment", func() {
				policyReco = &v1alpha1.PolicyRecommendation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: v1alpha1.PolicyRecommendationSpec{
						WorkloadMeta: v1alpha1.WorkloadMeta{
							TypeMeta: metav1.TypeMeta{
								Kind:       "Deployment",
								APIVersion: "apps/v1",
							},
							Name: HPAEnforcerPolicyRecoName,
						},
						TargetHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               60,
							Max:               100,
							TargetMetricValue: 50,
						},
						CurrentHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               20,
							Max:               100,
							TargetMetricValue: 40,
						},
						Policy:             "random",
						QueuedForExecution: &falseBool,
					},
				}
				Expect(k8sClient.Create(context.TODO(), policyReco)).To(Succeed())
				updatedPolicyReco := &v1alpha1.PolicyRecommendation{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)
				replicas := int32(10)
				deployment = &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:        HPAEnforcerPolicyRecoName,
						Namespace:   HPAEnforcerPolicyRecoNamespace,
						Annotations: map[string]string{hpaEnforcementEnabledAnnotation: "true"},
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: &replicas,
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test-app",
							},
						},
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "test-app",
								},
							},
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  "test-container",
										Image: "nginx:1.17.5",
									},
								},
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), deployment)).To(Succeed())
				updatedPolicyReco.Status = v1alpha1.PolicyRecommendationStatus{
					Conditions: []metav1.Condition{
						{
							Type:               string(v1alpha1.Initialized),
							Status:             metav1.ConditionTrue,
							LastTransitionTime: metav1.Now(),
							Reason:             PolicyRecommendationCreated,
							Message:            InitializedMessage,
						},
						{
							Type:               string(v1alpha1.RecoTaskProgress),
							Status:             metav1.ConditionFalse,
							Reason:             RecoTaskRecommendationGenerated,
							Message:            RecommendationGeneratedMessage,
							LastTransitionTime: metav1.Now(),
						},
					}}
				Expect(k8sClient.Status().Update(context.TODO(), updatedPolicyReco)).To(Succeed())
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ = json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)

				scaledObject = &kedaapi.ScaledObject{}
				Eventually(k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject), timeout, interval).Should(Succeed())
				scaledObjectString, _ := json.MarshalIndent(scaledObject, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", scaledObjectString)
				fmt.Println("step 20")
				Expect(scaledObject.OwnerReferences[0].Name).Should(Equal(deployment.Name))
				Expect(scaledObject.OwnerReferences[0].Kind).Should(Equal("Deployment"))
				Expect(scaledObject.OwnerReferences[0].APIVersion).Should(Equal("apps/v1"))
				Expect(scaledObject.Spec.ScaleTargetRef.Name).Should(Equal(deployment.Name))
				Expect(scaledObject.Spec.ScaleTargetRef.Kind).Should(Equal("Deployment"))
				Expect(scaledObject.Spec.ScaleTargetRef.APIVersion).Should(Equal("apps/v1"))
				Expect(int(*scaledObject.Spec.MaxReplicaCount)).Should(Equal(policyReco.Spec.CurrentHPAConfiguration.Max))
				Expect(int(*scaledObject.Spec.MinReplicaCount)).Should(Equal(policyReco.Spec.CurrentHPAConfiguration.Min))
				Expect(len(scaledObject.Spec.Triggers)).Should(Equal(2))
				Expect(scaledObject.Spec.Triggers[0].Type).Should(Equal("cpu"))
				n, _ := strconv.Atoi(scaledObject.Spec.Triggers[0].Metadata["value"])
				Expect(n).Should(Equal((policyReco.Spec.CurrentHPAConfiguration.TargetMetricValue)))

			})

			It("Should not create a ScaledObject for a Deployment", func() {
				policyReco = &v1alpha1.PolicyRecommendation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: v1alpha1.PolicyRecommendationSpec{
						WorkloadMeta: v1alpha1.WorkloadMeta{
							TypeMeta: metav1.TypeMeta{
								Kind:       "Deployment",
								APIVersion: "apps/v1",
							},
							Name: HPAEnforcerPolicyRecoName,
						},
						TargetHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               60,
							Max:               100,
							TargetMetricValue: 50,
						},
						CurrentHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               20,
							Max:               100,
							TargetMetricValue: 40,
						},
						Policy:             "random",
						QueuedForExecution: &falseBool,
					},
				}
				Expect(k8sClient.Create(context.TODO(), policyReco)).To(Succeed())
				updatedPolicyReco := &v1alpha1.PolicyRecommendation{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)
				replicas := int32(10)
				deployment = &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: &replicas,
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test-app",
							},
						},
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "test-app",
								},
							},
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  "test-container",
										Image: "nginx:1.17.5",
									},
								},
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), deployment)).To(Succeed())
				updatedPolicyReco.Status = v1alpha1.PolicyRecommendationStatus{
					Conditions: []metav1.Condition{
						{
							Type:               string(v1alpha1.Initialized),
							Status:             metav1.ConditionTrue,
							LastTransitionTime: metav1.Now(),
							Reason:             PolicyRecommendationCreated,
							Message:            InitializedMessage,
						},
						{
							Type:               string(v1alpha1.RecoTaskProgress),
							Status:             metav1.ConditionFalse,
							Reason:             RecoTaskRecommendationGenerated,
							Message:            RecommendationGeneratedMessage,
							LastTransitionTime: metav1.Now(),
						},
					}}
				Expect(k8sClient.Status().Update(context.TODO(), updatedPolicyReco)).To(Succeed())
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ = json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)

				scaledObject = &kedaapi.ScaledObject{}
				Eventually(func() bool {
					err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject)
					if err != nil {
						return true
					}
					return false
				}, timeout, interval).Should(BeTrue())

			})

			It("Should create a ScaledObject for a Rollout", func() {
				fmt.Fprintf(GinkgoWriter, "Step test pr %v\n", policyReco)
				fmt.Fprintf(GinkgoWriter, "Step test d %v\n", deployment)
				fmt.Fprintf(GinkgoWriter, "Step test r %v\n", rollout)
				fmt.Fprintf(GinkgoWriter, "Step test so %v\n", scaledObject)
				policyReco = &v1alpha1.PolicyRecommendation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: v1alpha1.PolicyRecommendationSpec{
						WorkloadMeta: v1alpha1.WorkloadMeta{
							TypeMeta: metav1.TypeMeta{
								Kind:       argoRolloutsKind,
								APIVersion: argoRolloutVersion,
							},
							Name: HPAEnforcerPolicyRecoName,
						},
						TargetHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               60,
							Max:               100,
							TargetMetricValue: 50,
						},
						CurrentHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               20,
							Max:               100,
							TargetMetricValue: 40,
						},
						Policy:             "random",
						QueuedForExecution: &falseBool,
					},
				}
				Expect(k8sClient.Create(context.TODO(), policyReco)).To(Succeed())
				updatedPolicyReco := &v1alpha1.PolicyRecommendation{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)
				fmt.Println("step 1")
				replicas := int32(10)
				rollout = &argov1alpha1.Rollout{
					ObjectMeta: metav1.ObjectMeta{
						Name:        HPAEnforcerPolicyRecoName,
						Namespace:   HPAEnforcerPolicyRecoNamespace,
						Annotations: map[string]string{hpaEnforcementEnabledAnnotation: "true"},
					},
					Spec: argov1alpha1.RolloutSpec{
						Replicas: &replicas,
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test-app",
							},
						},
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "test-app",
								},
							},
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  "test-container",
										Image: "nginx:1.17.5",
									},
								},
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), rollout)).To(Succeed())
				updatedPolicyReco.Status = v1alpha1.PolicyRecommendationStatus{
					Conditions: []metav1.Condition{
						{
							Type:               string(v1alpha1.Initialized),
							Status:             metav1.ConditionTrue,
							LastTransitionTime: metav1.Now(),
							Reason:             PolicyRecommendationCreated,
							Message:            InitializedMessage,
						},
						{
							Type:               string(v1alpha1.RecoTaskProgress),
							Status:             metav1.ConditionFalse,
							Reason:             RecoTaskRecommendationGenerated,
							Message:            RecommendationGeneratedMessage,
							LastTransitionTime: metav1.Now(),
						},
					}}
				Expect(k8sClient.Status().Update(context.TODO(), updatedPolicyReco)).To(Succeed())
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ = json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)

				scaledObject = &kedaapi.ScaledObject{}
				Eventually(k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject), timeout, interval).Should(Succeed())
				scaledObjectString, _ := json.MarshalIndent(scaledObject, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", scaledObjectString)
				fmt.Println("step 22")
				Expect(scaledObject.OwnerReferences[0].Name).Should(Equal(rollout.Name))
				Expect(scaledObject.OwnerReferences[0].Kind).Should(Equal("Rollout"))
				Expect(scaledObject.OwnerReferences[0].APIVersion).Should(Equal("argoproj.io/v1alpha1"))
				Expect(scaledObject.Spec.ScaleTargetRef.Name).Should(Equal(rollout.Name))
				Expect(scaledObject.Spec.ScaleTargetRef.Kind).Should(Equal("Rollout"))
				Expect(scaledObject.Spec.ScaleTargetRef.APIVersion).Should(Equal("argoproj.io/v1alpha1"))
				Expect(int(*scaledObject.Spec.MaxReplicaCount)).Should(Equal(policyReco.Spec.CurrentHPAConfiguration.Max))
				Expect(int(*scaledObject.Spec.MinReplicaCount)).Should(Equal(policyReco.Spec.CurrentHPAConfiguration.Min))
				Expect(len(scaledObject.Spec.Triggers)).Should(Equal(2))
				Expect(scaledObject.Spec.Triggers[0].Type).Should(Equal("cpu"))
				n, _ := strconv.Atoi(scaledObject.Spec.Triggers[0].Metadata["value"])
				Expect(n).Should(Equal((policyReco.Spec.CurrentHPAConfiguration.TargetMetricValue)))

			})

			It("Should not create a ScaledObject for a Rollout", func() {

				policyReco = &v1alpha1.PolicyRecommendation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: v1alpha1.PolicyRecommendationSpec{
						WorkloadMeta: v1alpha1.WorkloadMeta{
							TypeMeta: metav1.TypeMeta{
								Kind:       argoRolloutsKind,
								APIVersion: argoRolloutVersion,
							},
							Name: HPAEnforcerPolicyRecoName,
						},
						TargetHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               60,
							Max:               100,
							TargetMetricValue: 50,
						},
						CurrentHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               20,
							Max:               100,
							TargetMetricValue: 40,
						},
						Policy:             "random",
						QueuedForExecution: &falseBool,
					},
					Status: v1alpha1.PolicyRecommendationStatus{
						Conditions: []metav1.Condition{
							{
								Type:    string(v1alpha1.Initialized),
								Status:  metav1.ConditionTrue,
								Reason:  PolicyRecommendationCreated,
								Message: InitializedMessage,
							},
							{
								Type:    string(v1alpha1.RecoTaskProgress),
								Status:  metav1.ConditionFalse,
								Reason:  RecoTaskRecommendationGenerated,
								Message: RecommendationGeneratedMessage,
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), policyReco)).To(Succeed())
				updatedPolicyReco := &v1alpha1.PolicyRecommendation{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)
				replicas := int32(10)
				rollout = &argov1alpha1.Rollout{
					ObjectMeta: metav1.ObjectMeta{
						Name:        HPAEnforcerPolicyRecoName,
						Namespace:   HPAEnforcerPolicyRecoNamespace,
						Annotations: map[string]string{"ottoscalr.io/skip-hpa-enforcement": "true"},
					},
					Spec: argov1alpha1.RolloutSpec{
						Replicas: &replicas,
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test-app",
							},
						},
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "test-app",
								},
							},
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  "test-container",
										Image: "nginx:1.17.5",
									},
								},
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), rollout)).To(Succeed())
				updatedPolicyReco.Status = v1alpha1.PolicyRecommendationStatus{
					Conditions: []metav1.Condition{
						{
							Type:               string(v1alpha1.Initialized),
							Status:             metav1.ConditionTrue,
							LastTransitionTime: metav1.Now(),
							Reason:             PolicyRecommendationCreated,
							Message:            InitializedMessage,
						},
						{
							Type:               string(v1alpha1.RecoTaskProgress),
							Status:             metav1.ConditionFalse,
							Reason:             RecoTaskRecommendationGenerated,
							Message:            RecommendationGeneratedMessage,
							LastTransitionTime: metav1.Now(),
						},
					}}
				Expect(k8sClient.Status().Update(context.TODO(), updatedPolicyReco)).To(Succeed())
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ = json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)

				scaledObject = &kedaapi.ScaledObject{}
				Eventually(k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject), timeout, interval).ShouldNot(Succeed())
			})
		})

		When("A policy reco is generated in dry run mode in whitelist mode", func() {
			var scaledObject *kedaapi.ScaledObject
			var policyReco *v1alpha1.PolicyRecommendation
			var deployment *appsv1.Deployment
			var rollout *argov1alpha1.Rollout
			BeforeEach(func() {
				*hpaEnforcerIsDryRun = true
				*hpaEnforcerExcludedNamespaces = nil
				*hpaEnforcerIncludedNamespaces = nil
				*whitelistMode = true
				policyReco = nil
				scaledObject = nil
				deployment = nil
				rollout = nil
			})
			AfterEach(func() {
				if deployment != nil && len(deployment.Name) != 0 {
					Expect(k8sClient.Delete(context.TODO(), deployment)).Should(Succeed())
				}
				if rollout != nil && len(rollout.Name) != 0 {
					Expect(k8sClient.Delete(context.TODO(), rollout)).Should(Succeed())
				}
				Eventually(func() bool {
					err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, policyReco)
					if err != nil {
						return true
					}
					Expect(k8sClient.Delete(context.TODO(), policyReco)).Should(Succeed())
					return true
				}, timeout, interval).Should(BeTrue())
				Eventually(func() bool {
					err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject)
					if err != nil {
						return true
					}
					Expect(k8sClient.Delete(context.TODO(), scaledObject)).Should(Succeed())
					return true
				}, timeout, interval).Should(BeTrue())
			})

			It("Should create a ScaledObject for a Deployment", func() {
				policyReco = &v1alpha1.PolicyRecommendation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: v1alpha1.PolicyRecommendationSpec{
						WorkloadMeta: v1alpha1.WorkloadMeta{
							TypeMeta: metav1.TypeMeta{
								Kind:       "Deployment",
								APIVersion: "apps/v1",
							},
							Name: HPAEnforcerPolicyRecoName,
						},
						TargetHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               60,
							Max:               100,
							TargetMetricValue: 50,
						},
						CurrentHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               20,
							Max:               100,
							TargetMetricValue: 40,
						},
						Policy:             "random",
						QueuedForExecution: &falseBool,
					},
				}
				Expect(k8sClient.Create(context.TODO(), policyReco)).To(Succeed())
				updatedPolicyReco := &v1alpha1.PolicyRecommendation{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)
				replicas := int32(10)
				deployment = &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:        HPAEnforcerPolicyRecoName,
						Namespace:   HPAEnforcerPolicyRecoNamespace,
						Annotations: map[string]string{hpaEnforcementEnabledAnnotation: "true"},
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: &replicas,
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test-app",
							},
						},
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "test-app",
								},
							},
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  "test-container",
										Image: "nginx:1.17.5",
									},
								},
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), deployment)).To(Succeed())
				updatedPolicyReco.Status = v1alpha1.PolicyRecommendationStatus{
					Conditions: []metav1.Condition{
						{
							Type:               string(v1alpha1.Initialized),
							Status:             metav1.ConditionTrue,
							LastTransitionTime: metav1.Now(),
							Reason:             PolicyRecommendationCreated,
							Message:            InitializedMessage,
						},
						{
							Type:               string(v1alpha1.RecoTaskProgress),
							Status:             metav1.ConditionFalse,
							Reason:             RecoTaskRecommendationGenerated,
							Message:            RecommendationGeneratedMessage,
							LastTransitionTime: metav1.Now(),
						},
					}}
				Expect(k8sClient.Status().Update(context.TODO(), updatedPolicyReco)).To(Succeed())
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ = json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)

				scaledObject = &kedaapi.ScaledObject{}
				Eventually(func() bool {
					err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject)
					if err != nil {
						return true
					}
					return false
				}, timeout, interval).Should(BeTrue())

			})

			It("Should not create a ScaledObject for a Deployment", func() {
				policyReco = &v1alpha1.PolicyRecommendation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: v1alpha1.PolicyRecommendationSpec{
						WorkloadMeta: v1alpha1.WorkloadMeta{
							TypeMeta: metav1.TypeMeta{
								Kind:       "Deployment",
								APIVersion: "apps/v1",
							},
							Name: HPAEnforcerPolicyRecoName,
						},
						TargetHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               60,
							Max:               100,
							TargetMetricValue: 50,
						},
						CurrentHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               20,
							Max:               100,
							TargetMetricValue: 40,
						},
						Policy:             "random",
						QueuedForExecution: &falseBool,
					},
				}
				Expect(k8sClient.Create(context.TODO(), policyReco)).To(Succeed())
				updatedPolicyReco := &v1alpha1.PolicyRecommendation{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)
				replicas := int32(10)
				deployment = &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: &replicas,
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test-app",
							},
						},
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "test-app",
								},
							},
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  "test-container",
										Image: "nginx:1.17.5",
									},
								},
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), deployment)).To(Succeed())
				updatedPolicyReco.Status = v1alpha1.PolicyRecommendationStatus{
					Conditions: []metav1.Condition{
						{
							Type:               string(v1alpha1.Initialized),
							Status:             metav1.ConditionTrue,
							LastTransitionTime: metav1.Now(),
							Reason:             PolicyRecommendationCreated,
							Message:            InitializedMessage,
						},
						{
							Type:               string(v1alpha1.RecoTaskProgress),
							Status:             metav1.ConditionFalse,
							Reason:             RecoTaskRecommendationGenerated,
							Message:            RecommendationGeneratedMessage,
							LastTransitionTime: metav1.Now(),
						},
					}}
				Expect(k8sClient.Status().Update(context.TODO(), updatedPolicyReco)).To(Succeed())
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ = json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)

				scaledObject = &kedaapi.ScaledObject{}
				Eventually(func() bool {
					err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject)
					if err != nil {
						return true
					}
					return false
				}, timeout, interval).Should(BeTrue())

			})

			It("Should create a ScaledObject for a Rollout", func() {
				fmt.Fprintf(GinkgoWriter, "Step test pr %v\n", policyReco)
				fmt.Fprintf(GinkgoWriter, "Step test d %v\n", deployment)
				fmt.Fprintf(GinkgoWriter, "Step test r %v\n", rollout)
				fmt.Fprintf(GinkgoWriter, "Step test so %v\n", scaledObject)
				policyReco = &v1alpha1.PolicyRecommendation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: v1alpha1.PolicyRecommendationSpec{
						WorkloadMeta: v1alpha1.WorkloadMeta{
							TypeMeta: metav1.TypeMeta{
								Kind:       argoRolloutsKind,
								APIVersion: argoRolloutVersion,
							},
							Name: HPAEnforcerPolicyRecoName,
						},
						TargetHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               60,
							Max:               100,
							TargetMetricValue: 50,
						},
						CurrentHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               20,
							Max:               100,
							TargetMetricValue: 40,
						},
						Policy:             "random",
						QueuedForExecution: &falseBool,
					},
				}
				Expect(k8sClient.Create(context.TODO(), policyReco)).To(Succeed())
				updatedPolicyReco := &v1alpha1.PolicyRecommendation{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)
				fmt.Println("step 1")
				replicas := int32(10)
				rollout = &argov1alpha1.Rollout{
					ObjectMeta: metav1.ObjectMeta{
						Name:        HPAEnforcerPolicyRecoName,
						Namespace:   HPAEnforcerPolicyRecoNamespace,
						Annotations: map[string]string{hpaEnforcementEnabledAnnotation: "true"},
					},
					Spec: argov1alpha1.RolloutSpec{
						Replicas: &replicas,
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test-app",
							},
						},
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "test-app",
								},
							},
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  "test-container",
										Image: "nginx:1.17.5",
									},
								},
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), rollout)).To(Succeed())
				updatedPolicyReco.Status = v1alpha1.PolicyRecommendationStatus{
					Conditions: []metav1.Condition{
						{
							Type:               string(v1alpha1.Initialized),
							Status:             metav1.ConditionTrue,
							LastTransitionTime: metav1.Now(),
							Reason:             PolicyRecommendationCreated,
							Message:            InitializedMessage,
						},
						{
							Type:               string(v1alpha1.RecoTaskProgress),
							Status:             metav1.ConditionFalse,
							Reason:             RecoTaskRecommendationGenerated,
							Message:            RecommendationGeneratedMessage,
							LastTransitionTime: metav1.Now(),
						},
					}}
				Expect(k8sClient.Status().Update(context.TODO(), updatedPolicyReco)).To(Succeed())
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ = json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)

				scaledObject = &kedaapi.ScaledObject{}
				Eventually(func() bool {
					err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject)
					if err != nil {
						return true
					}
					return false
				}, timeout, interval).Should(BeTrue())

			})

			It("Should not create a ScaledObject for a Rollout", func() {

				policyReco = &v1alpha1.PolicyRecommendation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: v1alpha1.PolicyRecommendationSpec{
						WorkloadMeta: v1alpha1.WorkloadMeta{
							TypeMeta: metav1.TypeMeta{
								Kind:       argoRolloutsKind,
								APIVersion: argoRolloutVersion,
							},
							Name: HPAEnforcerPolicyRecoName,
						},
						TargetHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               60,
							Max:               100,
							TargetMetricValue: 50,
						},
						CurrentHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               20,
							Max:               100,
							TargetMetricValue: 40,
						},
						Policy:             "random",
						QueuedForExecution: &falseBool,
					},
					Status: v1alpha1.PolicyRecommendationStatus{
						Conditions: []metav1.Condition{
							{
								Type:    string(v1alpha1.Initialized),
								Status:  metav1.ConditionTrue,
								Reason:  PolicyRecommendationCreated,
								Message: InitializedMessage,
							},
							{
								Type:    string(v1alpha1.RecoTaskProgress),
								Status:  metav1.ConditionFalse,
								Reason:  RecoTaskRecommendationGenerated,
								Message: RecommendationGeneratedMessage,
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), policyReco)).To(Succeed())
				updatedPolicyReco := &v1alpha1.PolicyRecommendation{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)
				replicas := int32(10)
				rollout = &argov1alpha1.Rollout{
					ObjectMeta: metav1.ObjectMeta{
						Name:        HPAEnforcerPolicyRecoName,
						Namespace:   HPAEnforcerPolicyRecoNamespace,
						Annotations: map[string]string{"ottoscalr.io/skip-hpa-enforcement": "true"},
					},
					Spec: argov1alpha1.RolloutSpec{
						Replicas: &replicas,
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test-app",
							},
						},
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "test-app",
								},
							},
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  "test-container",
										Image: "nginx:1.17.5",
									},
								},
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), rollout)).To(Succeed())
				updatedPolicyReco.Status = v1alpha1.PolicyRecommendationStatus{
					Conditions: []metav1.Condition{
						{
							Type:               string(v1alpha1.Initialized),
							Status:             metav1.ConditionTrue,
							LastTransitionTime: metav1.Now(),
							Reason:             PolicyRecommendationCreated,
							Message:            InitializedMessage,
						},
						{
							Type:               string(v1alpha1.RecoTaskProgress),
							Status:             metav1.ConditionFalse,
							Reason:             RecoTaskRecommendationGenerated,
							Message:            RecommendationGeneratedMessage,
							LastTransitionTime: metav1.Now(),
						},
					}}
				Expect(k8sClient.Status().Update(context.TODO(), updatedPolicyReco)).To(Succeed())
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ = json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)

				scaledObject = &kedaapi.ScaledObject{}
				Eventually(k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject), timeout, interval).ShouldNot(Succeed())
			})
		})

		When("A policy reco is generated in blacklist mode with namespace blacklisted", func() {
			var scaledObject *kedaapi.ScaledObject
			var policyReco *v1alpha1.PolicyRecommendation
			var deployment *appsv1.Deployment
			var rollout *argov1alpha1.Rollout
			BeforeEach(func() {
				*hpaEnforcerIsDryRun = false
				*whitelistMode = falseBool
				*hpaEnforcerIncludedNamespaces = nil
				*hpaEnforcerExcludedNamespaces = nil
				*hpaEnforcerExcludedNamespaces = append(*hpaEnforcerExcludedNamespaces, HPAEnforcerPolicyRecoNamespace)
				policyReco = nil
				scaledObject = nil
				deployment = nil
				rollout = nil
				fmt.Printf("Excluded ns :  %v\n", *hpaEnforcerExcludedNamespaces)

			})
			AfterEach(func() {
				if deployment != nil && len(deployment.Name) != 0 {
					Expect(k8sClient.Delete(context.TODO(), deployment)).Should(Succeed())
				}
				if rollout != nil && len(rollout.Name) != 0 {
					Expect(k8sClient.Delete(context.TODO(), rollout)).Should(Succeed())
				}
				Eventually(func() bool {
					err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, policyReco)
					if err != nil {
						return true
					}
					Expect(k8sClient.Delete(context.TODO(), policyReco)).Should(Succeed())
					return true
				}, timeout, interval).Should(BeTrue())
				Eventually(func() bool {
					err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject)
					if err != nil {
						return true
					}
					Expect(k8sClient.Delete(context.TODO(), scaledObject)).Should(Succeed())
					return true
				}, timeout, interval).Should(BeTrue())
			})

			It("Should create a ScaledObject for a Deployment", func() {
				policyReco = &v1alpha1.PolicyRecommendation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: v1alpha1.PolicyRecommendationSpec{
						WorkloadMeta: v1alpha1.WorkloadMeta{
							TypeMeta: metav1.TypeMeta{
								Kind:       "Deployment",
								APIVersion: "apps/v1",
							},
							Name: HPAEnforcerPolicyRecoName,
						},
						TargetHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               60,
							Max:               100,
							TargetMetricValue: 50,
						},
						CurrentHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               20,
							Max:               100,
							TargetMetricValue: 40,
						},
						Policy:             "random",
						QueuedForExecution: &falseBool,
					},
				}
				Expect(k8sClient.Create(context.TODO(), policyReco)).To(Succeed())
				updatedPolicyReco := &v1alpha1.PolicyRecommendation{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)
				replicas := int32(10)
				deployment = &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: &replicas,
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test-app",
							},
						},
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "test-app",
								},
							},
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  "test-container",
										Image: "nginx:1.17.5",
									},
								},
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), deployment)).To(Succeed())
				updatedPolicyReco.Status = v1alpha1.PolicyRecommendationStatus{
					Conditions: []metav1.Condition{
						{
							Type:               string(v1alpha1.Initialized),
							Status:             metav1.ConditionTrue,
							LastTransitionTime: metav1.Now(),
							Reason:             PolicyRecommendationCreated,
							Message:            InitializedMessage,
						},
						{
							Type:               string(v1alpha1.RecoTaskProgress),
							Status:             metav1.ConditionFalse,
							Reason:             RecoTaskRecommendationGenerated,
							Message:            RecommendationGeneratedMessage,
							LastTransitionTime: metav1.Now(),
						},
					}}
				Expect(k8sClient.Status().Update(context.TODO(), updatedPolicyReco)).To(Succeed())
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ = json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)

				scaledObject = &kedaapi.ScaledObject{}
				Eventually(func() bool {
					err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject)
					if err != nil {
						return true
					}
					return false
				}, timeout, interval).Should(BeTrue())

			})

			It("Should not create a ScaledObject for a Deployment", func() {
				policyReco = &v1alpha1.PolicyRecommendation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: v1alpha1.PolicyRecommendationSpec{
						WorkloadMeta: v1alpha1.WorkloadMeta{
							TypeMeta: metav1.TypeMeta{
								Kind:       "Deployment",
								APIVersion: "apps/v1",
							},
							Name: HPAEnforcerPolicyRecoName,
						},
						TargetHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               60,
							Max:               100,
							TargetMetricValue: 50,
						},
						CurrentHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               20,
							Max:               100,
							TargetMetricValue: 40,
						},
						Policy:             "random",
						QueuedForExecution: &falseBool,
					},
				}
				Expect(k8sClient.Create(context.TODO(), policyReco)).To(Succeed())
				updatedPolicyReco := &v1alpha1.PolicyRecommendation{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)
				replicas := int32(10)
				deployment = &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:        HPAEnforcerPolicyRecoName,
						Namespace:   HPAEnforcerPolicyRecoNamespace,
						Annotations: map[string]string{"ottoscalr.io/skip-hpa-enforcement": "true"},
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: &replicas,
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test-app",
							},
						},
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "test-app",
								},
							},
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  "test-container",
										Image: "nginx:1.17.5",
									},
								},
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), deployment)).To(Succeed())
				updatedPolicyReco.Status = v1alpha1.PolicyRecommendationStatus{
					Conditions: []metav1.Condition{
						{
							Type:               string(v1alpha1.Initialized),
							Status:             metav1.ConditionTrue,
							LastTransitionTime: metav1.Now(),
							Reason:             PolicyRecommendationCreated,
							Message:            InitializedMessage,
						},
						{
							Type:               string(v1alpha1.RecoTaskProgress),
							Status:             metav1.ConditionFalse,
							Reason:             RecoTaskRecommendationGenerated,
							Message:            RecommendationGeneratedMessage,
							LastTransitionTime: metav1.Now(),
						},
					}}
				Expect(k8sClient.Status().Update(context.TODO(), updatedPolicyReco)).To(Succeed())
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ = json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)

				scaledObject = &kedaapi.ScaledObject{}
				Eventually(func() bool {
					err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject)
					if err != nil {
						return true
					}
					return false
				}, timeout, interval).Should(BeTrue())
			})

			It("Should create a ScaledObject for a Rollout", func() {
				fmt.Fprintf(GinkgoWriter, "Step test pr %v\n", policyReco)
				fmt.Fprintf(GinkgoWriter, "Step test d %v\n", deployment)
				fmt.Fprintf(GinkgoWriter, "Step test r %v\n", rollout)
				fmt.Fprintf(GinkgoWriter, "Step test so %v\n", scaledObject)
				policyReco = &v1alpha1.PolicyRecommendation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: v1alpha1.PolicyRecommendationSpec{
						WorkloadMeta: v1alpha1.WorkloadMeta{
							TypeMeta: metav1.TypeMeta{
								Kind:       argoRolloutsKind,
								APIVersion: argoRolloutVersion,
							},
							Name: HPAEnforcerPolicyRecoName,
						},
						TargetHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               60,
							Max:               100,
							TargetMetricValue: 50,
						},
						CurrentHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               20,
							Max:               100,
							TargetMetricValue: 40,
						},
						Policy:             "random",
						QueuedForExecution: &falseBool,
					},
				}
				Expect(k8sClient.Create(context.TODO(), policyReco)).To(Succeed())
				updatedPolicyReco := &v1alpha1.PolicyRecommendation{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)
				fmt.Println("step 1")
				replicas := int32(10)
				rollout = &argov1alpha1.Rollout{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: argov1alpha1.RolloutSpec{
						Replicas: &replicas,
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test-app",
							},
						},
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "test-app",
								},
							},
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  "test-container",
										Image: "nginx:1.17.5",
									},
								},
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), rollout)).To(Succeed())
				updatedPolicyReco.Status = v1alpha1.PolicyRecommendationStatus{
					Conditions: []metav1.Condition{
						{
							Type:               string(v1alpha1.Initialized),
							Status:             metav1.ConditionTrue,
							LastTransitionTime: metav1.Now(),
							Reason:             PolicyRecommendationCreated,
							Message:            InitializedMessage,
						},
						{
							Type:               string(v1alpha1.RecoTaskProgress),
							Status:             metav1.ConditionFalse,
							Reason:             RecoTaskRecommendationGenerated,
							Message:            RecommendationGeneratedMessage,
							LastTransitionTime: metav1.Now(),
						},
					}}
				Expect(k8sClient.Status().Update(context.TODO(), updatedPolicyReco)).To(Succeed())
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ = json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)

				scaledObject = &kedaapi.ScaledObject{}
				Eventually(func() bool {
					err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject)
					if err != nil {
						return true
					}
					return false
				}, timeout, interval).Should(BeTrue())

			})

			It("Should not create a ScaledObject for a Rollout", func() {

				policyReco = &v1alpha1.PolicyRecommendation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: v1alpha1.PolicyRecommendationSpec{
						WorkloadMeta: v1alpha1.WorkloadMeta{
							TypeMeta: metav1.TypeMeta{
								Kind:       argoRolloutsKind,
								APIVersion: argoRolloutVersion,
							},
							Name: HPAEnforcerPolicyRecoName,
						},
						TargetHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               60,
							Max:               100,
							TargetMetricValue: 50,
						},
						CurrentHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               20,
							Max:               100,
							TargetMetricValue: 40,
						},
						Policy:             "random",
						QueuedForExecution: &falseBool,
					},
					Status: v1alpha1.PolicyRecommendationStatus{
						Conditions: []metav1.Condition{
							{
								Type:    string(v1alpha1.Initialized),
								Status:  metav1.ConditionTrue,
								Reason:  PolicyRecommendationCreated,
								Message: InitializedMessage,
							},
							{
								Type:    string(v1alpha1.RecoTaskProgress),
								Status:  metav1.ConditionFalse,
								Reason:  RecoTaskRecommendationGenerated,
								Message: RecommendationGeneratedMessage,
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), policyReco)).To(Succeed())
				updatedPolicyReco := &v1alpha1.PolicyRecommendation{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)
				replicas := int32(10)
				rollout = &argov1alpha1.Rollout{
					ObjectMeta: metav1.ObjectMeta{
						Name:        HPAEnforcerPolicyRecoName,
						Namespace:   HPAEnforcerPolicyRecoNamespace,
						Annotations: map[string]string{"ottoscalr.io/skip-hpa-enforcement": "true"},
					},
					Spec: argov1alpha1.RolloutSpec{
						Replicas: &replicas,
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test-app",
							},
						},
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "test-app",
								},
							},
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  "test-container",
										Image: "nginx:1.17.5",
									},
								},
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), rollout)).To(Succeed())
				updatedPolicyReco.Status = v1alpha1.PolicyRecommendationStatus{
					Conditions: []metav1.Condition{
						{
							Type:               string(v1alpha1.Initialized),
							Status:             metav1.ConditionTrue,
							LastTransitionTime: metav1.Now(),
							Reason:             PolicyRecommendationCreated,
							Message:            InitializedMessage,
						},
						{
							Type:               string(v1alpha1.RecoTaskProgress),
							Status:             metav1.ConditionFalse,
							Reason:             RecoTaskRecommendationGenerated,
							Message:            RecommendationGeneratedMessage,
							LastTransitionTime: metav1.Now(),
						},
					}}
				Expect(k8sClient.Status().Update(context.TODO(), updatedPolicyReco)).To(Succeed())
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ = json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)

				scaledObject = &kedaapi.ScaledObject{}
				Eventually(func() bool {
					err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject)
					if err != nil {
						return true
					}
					return false
				}, timeout, interval).Should(BeTrue())
			})
		})

		When("A policy reco is generated in blacklist mode with namespace whitelisted", func() {
			var scaledObject *kedaapi.ScaledObject
			var policyReco *v1alpha1.PolicyRecommendation
			var deployment *appsv1.Deployment
			var rollout *argov1alpha1.Rollout
			BeforeEach(func() {
				*hpaEnforcerIsDryRun = false
				*whitelistMode = falseBool
				*hpaEnforcerIncludedNamespaces = nil
				*hpaEnforcerExcludedNamespaces = nil
				*hpaEnforcerIncludedNamespaces = append(*hpaEnforcerExcludedNamespaces, "default-0")
				policyReco = nil
				scaledObject = nil
				deployment = nil
				rollout = nil
				fmt.Printf("Excluded ns :  %v\n", *hpaEnforcerExcludedNamespaces)

			})
			AfterEach(func() {
				if deployment != nil && len(deployment.Name) != 0 {
					Expect(k8sClient.Delete(context.TODO(), deployment)).Should(Succeed())
				}
				if rollout != nil && len(rollout.Name) != 0 {
					Expect(k8sClient.Delete(context.TODO(), rollout)).Should(Succeed())
				}
				Eventually(func() bool {
					err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, policyReco)
					if err != nil {
						return true
					}
					Expect(k8sClient.Delete(context.TODO(), policyReco)).Should(Succeed())
					return true
				}, timeout, interval).Should(BeTrue())
				Eventually(func() bool {
					err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject)
					if err != nil {
						return true
					}
					Expect(k8sClient.Delete(context.TODO(), scaledObject)).Should(Succeed())
					return true
				}, timeout, interval).Should(BeTrue())
			})

			It("Should not create a ScaledObject for a Deployment", func() {
				policyReco = &v1alpha1.PolicyRecommendation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: v1alpha1.PolicyRecommendationSpec{
						WorkloadMeta: v1alpha1.WorkloadMeta{
							TypeMeta: metav1.TypeMeta{
								Kind:       "Deployment",
								APIVersion: "apps/v1",
							},
							Name: HPAEnforcerPolicyRecoName,
						},
						TargetHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               60,
							Max:               100,
							TargetMetricValue: 50,
						},
						CurrentHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               20,
							Max:               100,
							TargetMetricValue: 40,
						},
						Policy:             "random",
						QueuedForExecution: &falseBool,
					},
				}
				Expect(k8sClient.Create(context.TODO(), policyReco)).To(Succeed())
				updatedPolicyReco := &v1alpha1.PolicyRecommendation{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)
				replicas := int32(10)
				deployment = &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: &replicas,
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test-app",
							},
						},
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "test-app",
								},
							},
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  "test-container",
										Image: "nginx:1.17.5",
									},
								},
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), deployment)).To(Succeed())
				updatedPolicyReco.Status = v1alpha1.PolicyRecommendationStatus{
					Conditions: []metav1.Condition{
						{
							Type:               string(v1alpha1.Initialized),
							Status:             metav1.ConditionTrue,
							LastTransitionTime: metav1.Now(),
							Reason:             PolicyRecommendationCreated,
							Message:            InitializedMessage,
						},
						{
							Type:               string(v1alpha1.RecoTaskProgress),
							Status:             metav1.ConditionFalse,
							Reason:             RecoTaskRecommendationGenerated,
							Message:            RecommendationGeneratedMessage,
							LastTransitionTime: metav1.Now(),
						},
					}}
				Expect(k8sClient.Status().Update(context.TODO(), updatedPolicyReco)).To(Succeed())
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ = json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)

				scaledObject = &kedaapi.ScaledObject{}
				Eventually(func() bool {
					err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject)
					if err != nil {
						return true
					}
					return false
				}, timeout, interval).Should(BeTrue())

			})

			It("Should not create a ScaledObject for a Deployment", func() {
				policyReco = &v1alpha1.PolicyRecommendation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: v1alpha1.PolicyRecommendationSpec{
						WorkloadMeta: v1alpha1.WorkloadMeta{
							TypeMeta: metav1.TypeMeta{
								Kind:       "Deployment",
								APIVersion: "apps/v1",
							},
							Name: HPAEnforcerPolicyRecoName,
						},
						TargetHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               60,
							Max:               100,
							TargetMetricValue: 50,
						},
						CurrentHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               20,
							Max:               100,
							TargetMetricValue: 40,
						},
						Policy:             "random",
						QueuedForExecution: &falseBool,
					},
				}
				Expect(k8sClient.Create(context.TODO(), policyReco)).To(Succeed())
				updatedPolicyReco := &v1alpha1.PolicyRecommendation{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)
				replicas := int32(10)
				deployment = &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:        HPAEnforcerPolicyRecoName,
						Namespace:   HPAEnforcerPolicyRecoNamespace,
						Annotations: map[string]string{"ottoscalr.io/skip-hpa-enforcement": "true"},
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: &replicas,
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test-app",
							},
						},
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "test-app",
								},
							},
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  "test-container",
										Image: "nginx:1.17.5",
									},
								},
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), deployment)).To(Succeed())
				updatedPolicyReco.Status = v1alpha1.PolicyRecommendationStatus{
					Conditions: []metav1.Condition{
						{
							Type:               string(v1alpha1.Initialized),
							Status:             metav1.ConditionTrue,
							LastTransitionTime: metav1.Now(),
							Reason:             PolicyRecommendationCreated,
							Message:            InitializedMessage,
						},
						{
							Type:               string(v1alpha1.RecoTaskProgress),
							Status:             metav1.ConditionFalse,
							Reason:             RecoTaskRecommendationGenerated,
							Message:            RecommendationGeneratedMessage,
							LastTransitionTime: metav1.Now(),
						},
					}}
				Expect(k8sClient.Status().Update(context.TODO(), updatedPolicyReco)).To(Succeed())
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ = json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)

				scaledObject = &kedaapi.ScaledObject{}
				Eventually(func() bool {
					err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject)
					if err != nil {
						return true
					}
					return false
				}, timeout, interval).Should(BeTrue())
			})

			It("Should not create a ScaledObject for a Rollout", func() {
				fmt.Fprintf(GinkgoWriter, "Step test pr %v\n", policyReco)
				fmt.Fprintf(GinkgoWriter, "Step test d %v\n", deployment)
				fmt.Fprintf(GinkgoWriter, "Step test r %v\n", rollout)
				fmt.Fprintf(GinkgoWriter, "Step test so %v\n", scaledObject)
				policyReco = &v1alpha1.PolicyRecommendation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: v1alpha1.PolicyRecommendationSpec{
						WorkloadMeta: v1alpha1.WorkloadMeta{
							TypeMeta: metav1.TypeMeta{
								Kind:       argoRolloutsKind,
								APIVersion: argoRolloutVersion,
							},
							Name: HPAEnforcerPolicyRecoName,
						},
						TargetHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               60,
							Max:               100,
							TargetMetricValue: 50,
						},
						CurrentHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               20,
							Max:               100,
							TargetMetricValue: 40,
						},
						Policy:             "random",
						QueuedForExecution: &falseBool,
					},
				}
				Expect(k8sClient.Create(context.TODO(), policyReco)).To(Succeed())
				updatedPolicyReco := &v1alpha1.PolicyRecommendation{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)
				fmt.Println("step 1")
				replicas := int32(10)
				rollout = &argov1alpha1.Rollout{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: argov1alpha1.RolloutSpec{
						Replicas: &replicas,
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test-app",
							},
						},
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "test-app",
								},
							},
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  "test-container",
										Image: "nginx:1.17.5",
									},
								},
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), rollout)).To(Succeed())
				updatedPolicyReco.Status = v1alpha1.PolicyRecommendationStatus{
					Conditions: []metav1.Condition{
						{
							Type:               string(v1alpha1.Initialized),
							Status:             metav1.ConditionTrue,
							LastTransitionTime: metav1.Now(),
							Reason:             PolicyRecommendationCreated,
							Message:            InitializedMessage,
						},
						{
							Type:               string(v1alpha1.RecoTaskProgress),
							Status:             metav1.ConditionFalse,
							Reason:             RecoTaskRecommendationGenerated,
							Message:            RecommendationGeneratedMessage,
							LastTransitionTime: metav1.Now(),
						},
					}}
				Expect(k8sClient.Status().Update(context.TODO(), updatedPolicyReco)).To(Succeed())
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ = json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)

				scaledObject = &kedaapi.ScaledObject{}
				Eventually(func() bool {
					err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject)
					if err != nil {
						return true
					}
					return false
				}, timeout, interval).Should(BeTrue())

			})

			It("Should not create a ScaledObject for a Rollout", func() {

				policyReco = &v1alpha1.PolicyRecommendation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: v1alpha1.PolicyRecommendationSpec{
						WorkloadMeta: v1alpha1.WorkloadMeta{
							TypeMeta: metav1.TypeMeta{
								Kind:       argoRolloutsKind,
								APIVersion: argoRolloutVersion,
							},
							Name: HPAEnforcerPolicyRecoName,
						},
						TargetHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               60,
							Max:               100,
							TargetMetricValue: 50,
						},
						CurrentHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               20,
							Max:               100,
							TargetMetricValue: 40,
						},
						Policy:             "random",
						QueuedForExecution: &falseBool,
					},
					Status: v1alpha1.PolicyRecommendationStatus{
						Conditions: []metav1.Condition{
							{
								Type:    string(v1alpha1.Initialized),
								Status:  metav1.ConditionTrue,
								Reason:  PolicyRecommendationCreated,
								Message: InitializedMessage,
							},
							{
								Type:    string(v1alpha1.RecoTaskProgress),
								Status:  metav1.ConditionFalse,
								Reason:  RecoTaskRecommendationGenerated,
								Message: RecommendationGeneratedMessage,
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), policyReco)).To(Succeed())
				updatedPolicyReco := &v1alpha1.PolicyRecommendation{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)
				replicas := int32(10)
				rollout = &argov1alpha1.Rollout{
					ObjectMeta: metav1.ObjectMeta{
						Name:        HPAEnforcerPolicyRecoName,
						Namespace:   HPAEnforcerPolicyRecoNamespace,
						Annotations: map[string]string{"ottoscalr.io/skip-hpa-enforcement": "true"},
					},
					Spec: argov1alpha1.RolloutSpec{
						Replicas: &replicas,
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test-app",
							},
						},
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "test-app",
								},
							},
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  "test-container",
										Image: "nginx:1.17.5",
									},
								},
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), rollout)).To(Succeed())
				updatedPolicyReco.Status = v1alpha1.PolicyRecommendationStatus{
					Conditions: []metav1.Condition{
						{
							Type:               string(v1alpha1.Initialized),
							Status:             metav1.ConditionTrue,
							LastTransitionTime: metav1.Now(),
							Reason:             PolicyRecommendationCreated,
							Message:            InitializedMessage,
						},
						{
							Type:               string(v1alpha1.RecoTaskProgress),
							Status:             metav1.ConditionFalse,
							Reason:             RecoTaskRecommendationGenerated,
							Message:            RecommendationGeneratedMessage,
							LastTransitionTime: metav1.Now(),
						},
					}}
				Expect(k8sClient.Status().Update(context.TODO(), updatedPolicyReco)).To(Succeed())
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ = json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)

				scaledObject = &kedaapi.ScaledObject{}
				Eventually(func() bool {
					err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject)
					if err != nil {
						return true
					}
					return false
				}, timeout, interval).Should(BeTrue())
			})
		})

		When("A policy reco is generated in blacklist mode with namespace whitelisted", func() {
			var scaledObject *kedaapi.ScaledObject
			var policyReco *v1alpha1.PolicyRecommendation
			var deployment *appsv1.Deployment
			var rollout *argov1alpha1.Rollout
			BeforeEach(func() {
				*hpaEnforcerIsDryRun = false
				*whitelistMode = falseBool
				*hpaEnforcerIncludedNamespaces = nil
				*hpaEnforcerExcludedNamespaces = nil
				*hpaEnforcerIncludedNamespaces = append(*hpaEnforcerExcludedNamespaces, "default")
				policyReco = nil
				scaledObject = nil
				deployment = nil
				rollout = nil
				fmt.Printf("Excluded ns :  %v\n", *hpaEnforcerExcludedNamespaces)

			})
			AfterEach(func() {
				if deployment != nil && len(deployment.Name) != 0 {
					Expect(k8sClient.Delete(context.TODO(), deployment)).Should(Succeed())
				}
				if rollout != nil && len(rollout.Name) != 0 {
					Expect(k8sClient.Delete(context.TODO(), rollout)).Should(Succeed())
				}
				Eventually(func() bool {
					err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, policyReco)
					if err != nil {
						return true
					}
					Expect(k8sClient.Delete(context.TODO(), policyReco)).Should(Succeed())
					return true
				}, timeout, interval).Should(BeTrue())
				Eventually(func() bool {
					err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject)
					if err != nil {
						return true
					}
					Expect(k8sClient.Delete(context.TODO(), scaledObject)).Should(Succeed())
					return true
				}, timeout, interval).Should(BeTrue())
			})

			It("Should create a ScaledObject for a Deployment", func() {
				policyReco = &v1alpha1.PolicyRecommendation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: v1alpha1.PolicyRecommendationSpec{
						WorkloadMeta: v1alpha1.WorkloadMeta{
							TypeMeta: metav1.TypeMeta{
								Kind:       "Deployment",
								APIVersion: "apps/v1",
							},
							Name: HPAEnforcerPolicyRecoName,
						},
						TargetHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               60,
							Max:               100,
							TargetMetricValue: 50,
						},
						CurrentHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               20,
							Max:               100,
							TargetMetricValue: 40,
						},
						Policy:             "random",
						QueuedForExecution: &falseBool,
					},
				}
				Expect(k8sClient.Create(context.TODO(), policyReco)).To(Succeed())
				updatedPolicyReco := &v1alpha1.PolicyRecommendation{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)
				replicas := int32(10)
				deployment = &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: &replicas,
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test-app",
							},
						},
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "test-app",
								},
							},
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  "test-container",
										Image: "nginx:1.17.5",
									},
								},
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), deployment)).To(Succeed())
				updatedPolicyReco.Status = v1alpha1.PolicyRecommendationStatus{
					Conditions: []metav1.Condition{
						{
							Type:               string(v1alpha1.Initialized),
							Status:             metav1.ConditionTrue,
							LastTransitionTime: metav1.Now(),
							Reason:             PolicyRecommendationCreated,
							Message:            InitializedMessage,
						},
						{
							Type:               string(v1alpha1.RecoTaskProgress),
							Status:             metav1.ConditionFalse,
							Reason:             RecoTaskRecommendationGenerated,
							Message:            RecommendationGeneratedMessage,
							LastTransitionTime: metav1.Now(),
						},
					}}
				Expect(k8sClient.Status().Update(context.TODO(), updatedPolicyReco)).To(Succeed())
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ = json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)

				scaledObject = &kedaapi.ScaledObject{}
				Eventually(k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject), timeout, interval).Should(Succeed())
				scaledObjectString, _ := json.MarshalIndent(scaledObject, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", scaledObjectString)
				fmt.Println("step 22")
				Expect(scaledObject.OwnerReferences[0].Name).Should(Equal(deployment.Name))
				Expect(scaledObject.OwnerReferences[0].Kind).Should(Equal("Deployment"))
				Expect(scaledObject.OwnerReferences[0].APIVersion).Should(Equal("apps/v1"))
				Expect(scaledObject.Spec.ScaleTargetRef.Name).Should(Equal(deployment.Name))
				Expect(scaledObject.Spec.ScaleTargetRef.Kind).Should(Equal("Deployment"))
				Expect(scaledObject.Spec.ScaleTargetRef.APIVersion).Should(Equal("apps/v1"))
				Expect(int(*scaledObject.Spec.MaxReplicaCount)).Should(Equal(policyReco.Spec.CurrentHPAConfiguration.Max))
				Expect(int(*scaledObject.Spec.MinReplicaCount)).Should(Equal(policyReco.Spec.CurrentHPAConfiguration.Min))
				Expect(len(scaledObject.Spec.Triggers)).Should(Equal(2))
				Expect(scaledObject.Spec.Triggers[0].Type).Should(Equal("cpu"))
				n, _ := strconv.Atoi(scaledObject.Spec.Triggers[0].Metadata["value"])
				Expect(n).Should(Equal((policyReco.Spec.CurrentHPAConfiguration.TargetMetricValue)))

			})

			It("Should not create a ScaledObject for a Deployment", func() {
				policyReco = &v1alpha1.PolicyRecommendation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: v1alpha1.PolicyRecommendationSpec{
						WorkloadMeta: v1alpha1.WorkloadMeta{
							TypeMeta: metav1.TypeMeta{
								Kind:       "Deployment",
								APIVersion: "apps/v1",
							},
							Name: HPAEnforcerPolicyRecoName,
						},
						TargetHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               60,
							Max:               100,
							TargetMetricValue: 50,
						},
						CurrentHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               20,
							Max:               100,
							TargetMetricValue: 40,
						},
						Policy:             "random",
						QueuedForExecution: &falseBool,
					},
				}
				Expect(k8sClient.Create(context.TODO(), policyReco)).To(Succeed())
				updatedPolicyReco := &v1alpha1.PolicyRecommendation{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)
				replicas := int32(10)
				deployment = &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:        HPAEnforcerPolicyRecoName,
						Namespace:   HPAEnforcerPolicyRecoNamespace,
						Annotations: map[string]string{"ottoscalr.io/skip-hpa-enforcement": "true"},
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: &replicas,
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test-app",
							},
						},
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "test-app",
								},
							},
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  "test-container",
										Image: "nginx:1.17.5",
									},
								},
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), deployment)).To(Succeed())
				updatedPolicyReco.Status = v1alpha1.PolicyRecommendationStatus{
					Conditions: []metav1.Condition{
						{
							Type:               string(v1alpha1.Initialized),
							Status:             metav1.ConditionTrue,
							LastTransitionTime: metav1.Now(),
							Reason:             PolicyRecommendationCreated,
							Message:            InitializedMessage,
						},
						{
							Type:               string(v1alpha1.RecoTaskProgress),
							Status:             metav1.ConditionFalse,
							Reason:             RecoTaskRecommendationGenerated,
							Message:            RecommendationGeneratedMessage,
							LastTransitionTime: metav1.Now(),
						},
					}}
				Expect(k8sClient.Status().Update(context.TODO(), updatedPolicyReco)).To(Succeed())
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ = json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)

				scaledObject = &kedaapi.ScaledObject{}
				Eventually(func() bool {
					err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject)
					if err != nil {
						return true
					}
					return false
				}, timeout, interval).Should(BeTrue())
			})

			It("Should create a ScaledObject for a Rollout", func() {
				fmt.Fprintf(GinkgoWriter, "Step test pr %v\n", policyReco)
				fmt.Fprintf(GinkgoWriter, "Step test d %v\n", deployment)
				fmt.Fprintf(GinkgoWriter, "Step test r %v\n", rollout)
				fmt.Fprintf(GinkgoWriter, "Step test so %v\n", scaledObject)
				policyReco = &v1alpha1.PolicyRecommendation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: v1alpha1.PolicyRecommendationSpec{
						WorkloadMeta: v1alpha1.WorkloadMeta{
							TypeMeta: metav1.TypeMeta{
								Kind:       argoRolloutsKind,
								APIVersion: argoRolloutVersion,
							},
							Name: HPAEnforcerPolicyRecoName,
						},
						TargetHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               60,
							Max:               100,
							TargetMetricValue: 50,
						},
						CurrentHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               20,
							Max:               100,
							TargetMetricValue: 40,
						},
						Policy:             "random",
						QueuedForExecution: &falseBool,
					},
				}
				Expect(k8sClient.Create(context.TODO(), policyReco)).To(Succeed())
				updatedPolicyReco := &v1alpha1.PolicyRecommendation{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)
				fmt.Println("step 1")
				replicas := int32(10)
				rollout = &argov1alpha1.Rollout{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: argov1alpha1.RolloutSpec{
						Replicas: &replicas,
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test-app",
							},
						},
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "test-app",
								},
							},
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  "test-container",
										Image: "nginx:1.17.5",
									},
								},
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), rollout)).To(Succeed())
				updatedPolicyReco.Status = v1alpha1.PolicyRecommendationStatus{
					Conditions: []metav1.Condition{
						{
							Type:               string(v1alpha1.Initialized),
							Status:             metav1.ConditionTrue,
							LastTransitionTime: metav1.Now(),
							Reason:             PolicyRecommendationCreated,
							Message:            InitializedMessage,
						},
						{
							Type:               string(v1alpha1.RecoTaskProgress),
							Status:             metav1.ConditionFalse,
							Reason:             RecoTaskRecommendationGenerated,
							Message:            RecommendationGeneratedMessage,
							LastTransitionTime: metav1.Now(),
						},
					}}
				Expect(k8sClient.Status().Update(context.TODO(), updatedPolicyReco)).To(Succeed())
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ = json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)

				scaledObject = &kedaapi.ScaledObject{}
				Eventually(k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject), timeout, interval).Should(Succeed())
				scaledObjectString, _ := json.MarshalIndent(scaledObject, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", scaledObjectString)
				fmt.Println("step 22")
				Expect(scaledObject.OwnerReferences[0].Name).Should(Equal(rollout.Name))
				Expect(scaledObject.OwnerReferences[0].Kind).Should(Equal("Rollout"))
				Expect(scaledObject.OwnerReferences[0].APIVersion).Should(Equal("argoproj.io/v1alpha1"))
				Expect(scaledObject.Spec.ScaleTargetRef.Name).Should(Equal(rollout.Name))
				Expect(scaledObject.Spec.ScaleTargetRef.Kind).Should(Equal("Rollout"))
				Expect(scaledObject.Spec.ScaleTargetRef.APIVersion).Should(Equal("argoproj.io/v1alpha1"))
				Expect(int(*scaledObject.Spec.MaxReplicaCount)).Should(Equal(policyReco.Spec.CurrentHPAConfiguration.Max))
				Expect(int(*scaledObject.Spec.MinReplicaCount)).Should(Equal(policyReco.Spec.CurrentHPAConfiguration.Min))
				Expect(len(scaledObject.Spec.Triggers)).Should(Equal(2))
				Expect(scaledObject.Spec.Triggers[0].Type).Should(Equal("cpu"))
				n, _ := strconv.Atoi(scaledObject.Spec.Triggers[0].Metadata["value"])
				Expect(n).Should(Equal((policyReco.Spec.CurrentHPAConfiguration.TargetMetricValue)))

			})

			It("Should not create a ScaledObject for a Rollout", func() {

				policyReco = &v1alpha1.PolicyRecommendation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: v1alpha1.PolicyRecommendationSpec{
						WorkloadMeta: v1alpha1.WorkloadMeta{
							TypeMeta: metav1.TypeMeta{
								Kind:       argoRolloutsKind,
								APIVersion: argoRolloutVersion,
							},
							Name: HPAEnforcerPolicyRecoName,
						},
						TargetHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               60,
							Max:               100,
							TargetMetricValue: 50,
						},
						CurrentHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               20,
							Max:               100,
							TargetMetricValue: 40,
						},
						Policy:             "random",
						QueuedForExecution: &falseBool,
					},
					Status: v1alpha1.PolicyRecommendationStatus{
						Conditions: []metav1.Condition{
							{
								Type:    string(v1alpha1.Initialized),
								Status:  metav1.ConditionTrue,
								Reason:  PolicyRecommendationCreated,
								Message: InitializedMessage,
							},
							{
								Type:    string(v1alpha1.RecoTaskProgress),
								Status:  metav1.ConditionFalse,
								Reason:  RecoTaskRecommendationGenerated,
								Message: RecommendationGeneratedMessage,
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), policyReco)).To(Succeed())
				updatedPolicyReco := &v1alpha1.PolicyRecommendation{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)
				replicas := int32(10)
				rollout = &argov1alpha1.Rollout{
					ObjectMeta: metav1.ObjectMeta{
						Name:        HPAEnforcerPolicyRecoName,
						Namespace:   HPAEnforcerPolicyRecoNamespace,
						Annotations: map[string]string{"ottoscalr.io/skip-hpa-enforcement": "true"},
					},
					Spec: argov1alpha1.RolloutSpec{
						Replicas: &replicas,
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test-app",
							},
						},
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "test-app",
								},
							},
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  "test-container",
										Image: "nginx:1.17.5",
									},
								},
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), rollout)).To(Succeed())
				updatedPolicyReco.Status = v1alpha1.PolicyRecommendationStatus{
					Conditions: []metav1.Condition{
						{
							Type:               string(v1alpha1.Initialized),
							Status:             metav1.ConditionTrue,
							LastTransitionTime: metav1.Now(),
							Reason:             PolicyRecommendationCreated,
							Message:            InitializedMessage,
						},
						{
							Type:               string(v1alpha1.RecoTaskProgress),
							Status:             metav1.ConditionFalse,
							Reason:             RecoTaskRecommendationGenerated,
							Message:            RecommendationGeneratedMessage,
							LastTransitionTime: metav1.Now(),
						},
					}}
				Expect(k8sClient.Status().Update(context.TODO(), updatedPolicyReco)).To(Succeed())
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ = json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)

				scaledObject = &kedaapi.ScaledObject{}
				Eventually(func() bool {
					err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject)
					if err != nil {
						return true
					}
					return false
				}, timeout, interval).Should(BeTrue())
			})

		})
	*/

	When("A workload doesn't qualify for reconciliation when the scaledobject already exists.", func() {
		var scaledObject *kedaapi.ScaledObject
		var policyReco *v1alpha1.PolicyRecommendation
		var deployment *appsv1.Deployment
		var rollout *argov1alpha1.Rollout
		BeforeEach(func() {
			*hpaEnforcerIsDryRun = false
			*whitelistMode = falseBool
			*hpaEnforcerIncludedNamespaces = nil
			*hpaEnforcerExcludedNamespaces = nil
			policyReco = nil
			scaledObject = nil
			deployment = nil
			rollout = nil
		})
		AfterEach(func() {
			if deployment != nil && len(deployment.Name) != 0 {
				Expect(k8sClient.Delete(context.TODO(), deployment)).Should(Succeed())
			}
			if rollout != nil && len(rollout.Name) != 0 {
				Expect(k8sClient.Delete(context.TODO(), rollout)).Should(Succeed())
			}
			Eventually(func() bool {
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, policyReco)
				if err != nil {
					return true
				}
				Expect(k8sClient.Delete(context.TODO(), policyReco)).Should(Succeed())
				return true
			}, timeout, interval).Should(BeTrue())
			Eventually(func() bool {
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject)
				if err != nil {
					return true
				}
				Expect(k8sClient.Delete(context.TODO(), scaledObject)).Should(Succeed())
				return true
			}, timeout, interval).Should(BeTrue())
		})

		It("Should delete the ScaledObject for a Deployment", func() {
			initialMax := 100
			policyReco = &v1alpha1.PolicyRecommendation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      HPAEnforcerPolicyRecoName,
					Namespace: HPAEnforcerPolicyRecoNamespace,
				},
				Spec: v1alpha1.PolicyRecommendationSpec{
					WorkloadMeta: v1alpha1.WorkloadMeta{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Deployment",
							APIVersion: "apps/v1",
						},
						Name: HPAEnforcerPolicyRecoName,
					},
					TargetHPAConfiguration: v1alpha1.HPAConfiguration{
						Min:               60,
						Max:               100,
						TargetMetricValue: 50,
					},
					CurrentHPAConfiguration: v1alpha1.HPAConfiguration{
						Min:               20,
						Max:               initialMax,
						TargetMetricValue: 40,
					},
					Policy:             "random",
					QueuedForExecution: &falseBool,
				},
			}
			Expect(k8sClient.Create(context.TODO(), policyReco)).To(Succeed())
			updatedPolicyReco := &v1alpha1.PolicyRecommendation{}
			Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
				Namespace: HPAEnforcerPolicyRecoNamespace,
				Name:      HPAEnforcerPolicyRecoName,
			}, updatedPolicyReco)).To(Succeed())
			updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicyReco, "", "   ")
			fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)
			replicas := int32(10)
			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      HPAEnforcerPolicyRecoName,
					Namespace: HPAEnforcerPolicyRecoNamespace,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test-app",
						},
					},
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test-app",
							},
						},
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name:  "test-container",
									Image: "nginx:1.17.5",
								},
							},
						},
					}},
			}
			Expect(k8sClient.Create(context.TODO(), deployment)).To(Succeed())
			updatedPolicyReco.Status = v1alpha1.PolicyRecommendationStatus{
				Conditions: []metav1.Condition{
					{
						Type:               string(v1alpha1.Initialized),
						Status:             metav1.ConditionTrue,
						LastTransitionTime: metav1.Now(),
						Reason:             PolicyRecommendationCreated,
						Message:            InitializedMessage,
					},
					{
						Type:               string(v1alpha1.RecoTaskProgress),
						Status:             metav1.ConditionFalse,
						Reason:             RecoTaskRecommendationGenerated,
						Message:            RecommendationGeneratedMessage,
						LastTransitionTime: metav1.Now(),
					},
				}}
			Expect(k8sClient.Status().Update(context.TODO(), updatedPolicyReco)).To(Succeed())
			Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
				Namespace: HPAEnforcerPolicyRecoNamespace,
				Name:      HPAEnforcerPolicyRecoName,
			}, updatedPolicyReco)).To(Succeed())
			updatedPolicyRecoString, _ = json.MarshalIndent(updatedPolicyReco, "", "   ")
			fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)

			scaledObject = &kedaapi.ScaledObject{}
			Eventually(k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject), timeout, interval).Should(Succeed())
			scaledObjectString, _ := json.MarshalIndent(scaledObject, "", "   ")
			fmt.Fprintf(GinkgoWriter, "%s\n", scaledObjectString)
			Expect(scaledObject.OwnerReferences[0].Name).Should(Equal(deployment.Name))
			Expect(scaledObject.OwnerReferences[0].Kind).Should(Equal("Deployment"))
			Expect(scaledObject.OwnerReferences[0].APIVersion).Should(Equal("apps/v1"))
			Expect(scaledObject.Spec.ScaleTargetRef.Name).Should(Equal(deployment.Name))
			Expect(scaledObject.Spec.ScaleTargetRef.Kind).Should(Equal("Deployment"))
			Expect(scaledObject.Spec.ScaleTargetRef.APIVersion).Should(Equal("apps/v1"))
			Expect(int(*scaledObject.Spec.MaxReplicaCount)).Should(Equal(policyReco.Spec.CurrentHPAConfiguration.Max))
			Expect(int(*scaledObject.Spec.MinReplicaCount)).Should(Equal(policyReco.Spec.CurrentHPAConfiguration.Min))
			Expect(len(scaledObject.Spec.Triggers)).Should(Equal(2))
			Expect(scaledObject.Spec.Triggers[0].Type).Should(Equal("cpu"))
			n, _ := strconv.Atoi(scaledObject.Spec.Triggers[0].Metadata["value"])
			Expect(n).Should(Equal((policyReco.Spec.CurrentHPAConfiguration.TargetMetricValue)))

			updatedPolicyReco = &v1alpha1.PolicyRecommendation{}
			Eventually(func() bool {
				err := k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)
				if err != nil {
					return false
				}
				for _, v := range updatedPolicyReco.Status.Conditions {
					if v.Type == string(v1alpha1.HPAEnforced) {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())

			updatedPolicyReco.Spec.CurrentHPAConfiguration.Min = 1
			updatedPolicyReco.Spec.CurrentHPAConfiguration.Max = 2

			Expect(k8sClient.Update(context.TODO(), updatedPolicyReco)).To(Succeed())

			deployment := &appsv1.Deployment{}
			Eventually(func() int {
				err := k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, deployment)
				if err != nil {
					return -1
				}
				return int(*deployment.Spec.Replicas)
			}, timeout, interval).Should(Equal(initialMax))

			scaledObject = &kedaapi.ScaledObject{}
			Eventually(k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject), timeout, interval).ShouldNot(Succeed())
		})

		It("Should delete the ScaledObject for a Deployment when marked with skip annotation", func() {
			initialMax := 100
			policyReco = &v1alpha1.PolicyRecommendation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      HPAEnforcerPolicyRecoName,
					Namespace: HPAEnforcerPolicyRecoNamespace,
				},
				Spec: v1alpha1.PolicyRecommendationSpec{
					WorkloadMeta: v1alpha1.WorkloadMeta{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Deployment",
							APIVersion: "apps/v1",
						},
						Name: HPAEnforcerPolicyRecoName,
					},
					TargetHPAConfiguration: v1alpha1.HPAConfiguration{
						Min:               60,
						Max:               100,
						TargetMetricValue: 50,
					},
					CurrentHPAConfiguration: v1alpha1.HPAConfiguration{
						Min:               20,
						Max:               initialMax,
						TargetMetricValue: 40,
					},
					Policy:             "random",
					QueuedForExecution: &falseBool,
				},
			}
			Expect(k8sClient.Create(context.TODO(), policyReco)).To(Succeed())
			updatedPolicyReco := &v1alpha1.PolicyRecommendation{}
			Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
				Namespace: HPAEnforcerPolicyRecoNamespace,
				Name:      HPAEnforcerPolicyRecoName,
			}, updatedPolicyReco)).To(Succeed())
			updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicyReco, "", "   ")
			fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)
			replicas := int32(10)
			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      HPAEnforcerPolicyRecoName,
					Namespace: HPAEnforcerPolicyRecoNamespace,
					UID:       "abcdef",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test-app",
						},
					},
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test-app",
							},
						},
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name:  "test-container",
									Image: "nginx:1.17.5",
								},
							},
						},
					}},
			}
			Expect(k8sClient.Create(context.TODO(), deployment)).To(Succeed())
			updatedPolicyReco.Status = v1alpha1.PolicyRecommendationStatus{
				Conditions: []metav1.Condition{
					{
						Type:               string(v1alpha1.Initialized),
						Status:             metav1.ConditionTrue,
						LastTransitionTime: metav1.Now(),
						Reason:             PolicyRecommendationCreated,
						Message:            InitializedMessage,
					},
					{
						Type:               string(v1alpha1.RecoTaskProgress),
						Status:             metav1.ConditionFalse,
						Reason:             RecoTaskRecommendationGenerated,
						Message:            RecommendationGeneratedMessage,
						LastTransitionTime: metav1.Now(),
					},
				}}
			Expect(k8sClient.Status().Update(context.TODO(), updatedPolicyReco)).To(Succeed())
			Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
				Namespace: HPAEnforcerPolicyRecoNamespace,
				Name:      HPAEnforcerPolicyRecoName,
			}, updatedPolicyReco)).To(Succeed())
			updatedPolicyReco.OwnerReferences = []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       HPAEnforcerPolicyRecoName,
					UID:        deployment.UID,
				},
			}
			Expect(k8sClient.Update(context.TODO(), updatedPolicyReco)).To(Succeed())

			updatedPolicyRecoString, _ = json.MarshalIndent(updatedPolicyReco, "", "   ")
			fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)

			scaledObject = &kedaapi.ScaledObject{}
			Eventually(k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject), timeout, interval).Should(Succeed())
			scaledObjectString, _ := json.MarshalIndent(scaledObject, "", "   ")
			fmt.Fprintf(GinkgoWriter, "%s\n", scaledObjectString)
			Expect(scaledObject.OwnerReferences[0].Name).Should(Equal(deployment.Name))
			Expect(scaledObject.OwnerReferences[0].Kind).Should(Equal("Deployment"))
			Expect(scaledObject.OwnerReferences[0].APIVersion).Should(Equal("apps/v1"))
			Expect(scaledObject.Spec.ScaleTargetRef.Name).Should(Equal(deployment.Name))
			Expect(scaledObject.Spec.ScaleTargetRef.Kind).Should(Equal("Deployment"))
			Expect(scaledObject.Spec.ScaleTargetRef.APIVersion).Should(Equal("apps/v1"))
			Expect(int(*scaledObject.Spec.MaxReplicaCount)).Should(Equal(policyReco.Spec.CurrentHPAConfiguration.Max))
			Expect(int(*scaledObject.Spec.MinReplicaCount)).Should(Equal(policyReco.Spec.CurrentHPAConfiguration.Min))
			Expect(len(scaledObject.Spec.Triggers)).Should(Equal(2))
			Expect(scaledObject.Spec.Triggers[0].Type).Should(Equal("cpu"))
			n, _ := strconv.Atoi(scaledObject.Spec.Triggers[0].Metadata["value"])
			Expect(n).Should(Equal((policyReco.Spec.CurrentHPAConfiguration.TargetMetricValue)))

			deployment = &appsv1.Deployment{}
			Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
				Namespace: HPAEnforcerPolicyRecoNamespace,
				Name:      HPAEnforcerPolicyRecoName,
			}, deployment)).Should(Succeed())
			deployment.Annotations = map[string]string{hpaEnforcementDisabledAnnotation: "true"}
			Expect(k8sClient.Update(context.TODO(), deployment)).Should(Succeed())

			deployment = &appsv1.Deployment{}
			k8sClient.Get(context.TODO(), types.NamespacedName{
				Namespace: HPAEnforcerPolicyRecoNamespace,
				Name:      HPAEnforcerPolicyRecoName,
			}, deployment)
			deployStr, _ := json.MarshalIndent(deployment, "", "   ")
			fmt.Fprintf(GinkgoWriter, "Deployment after skip annotation %s", deployStr)

			deployment := &appsv1.Deployment{}
			Eventually(func() int {
				err := k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, deployment)
				if err != nil {
					return -1
				}
				return int(*deployment.Spec.Replicas)
			}, timeout, interval).Should(Equal(initialMax))

			scaledObject = &kedaapi.ScaledObject{}
			Eventually(k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject), timeout, interval).ShouldNot(Succeed())

		})

		It("Should delete the ScaledObject for a Rollout", func() {
			initialMax := 100
			policyReco = &v1alpha1.PolicyRecommendation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      HPAEnforcerPolicyRecoName,
					Namespace: HPAEnforcerPolicyRecoNamespace,
				},
				Spec: v1alpha1.PolicyRecommendationSpec{
					WorkloadMeta: v1alpha1.WorkloadMeta{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Rollout",
							APIVersion: "argoproj.io/v1alpha1",
						},
						Name: HPAEnforcerPolicyRecoName,
					},
					TargetHPAConfiguration: v1alpha1.HPAConfiguration{
						Min:               60,
						Max:               100,
						TargetMetricValue: 50,
					},
					CurrentHPAConfiguration: v1alpha1.HPAConfiguration{
						Min:               20,
						Max:               initialMax,
						TargetMetricValue: 40,
					},
					Policy:             "random",
					QueuedForExecution: &falseBool,
				},
			}
			Expect(k8sClient.Create(context.TODO(), policyReco)).To(Succeed())
			updatedPolicyReco := &v1alpha1.PolicyRecommendation{}
			Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
				Namespace: HPAEnforcerPolicyRecoNamespace,
				Name:      HPAEnforcerPolicyRecoName,
			}, updatedPolicyReco)).To(Succeed())
			updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicyReco, "", "   ")
			fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)
			fmt.Println("step 1")
			replicas := int32(10)
			rollout = &argov1alpha1.Rollout{
				ObjectMeta: metav1.ObjectMeta{
					Name:      HPAEnforcerPolicyRecoName,
					Namespace: HPAEnforcerPolicyRecoNamespace,
				},
				Spec: argov1alpha1.RolloutSpec{
					Replicas: &replicas,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test-app",
						},
					},
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test-app",
							},
						},
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name:  "test-container",
									Image: "nginx:1.17.5",
								},
							},
						},
					}},
			}
			Expect(k8sClient.Create(context.TODO(), rollout)).To(Succeed())
			updatedPolicyReco.Status = v1alpha1.PolicyRecommendationStatus{
				Conditions: []metav1.Condition{
					{
						Type:               string(v1alpha1.Initialized),
						Status:             metav1.ConditionTrue,
						LastTransitionTime: metav1.Now(),
						Reason:             PolicyRecommendationCreated,
						Message:            InitializedMessage,
					},
					{
						Type:               string(v1alpha1.RecoTaskProgress),
						Status:             metav1.ConditionFalse,
						Reason:             RecoTaskRecommendationGenerated,
						Message:            RecommendationGeneratedMessage,
						LastTransitionTime: metav1.Now(),
					},
				}}
			Expect(k8sClient.Status().Update(context.TODO(), updatedPolicyReco)).To(Succeed())
			Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
				Namespace: HPAEnforcerPolicyRecoNamespace,
				Name:      HPAEnforcerPolicyRecoName,
			}, updatedPolicyReco)).To(Succeed())
			updatedPolicyRecoString, _ = json.MarshalIndent(updatedPolicyReco, "", "   ")
			fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)

			scaledObject = &kedaapi.ScaledObject{}
			Eventually(k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject), timeout, interval).Should(Succeed())
			scaledObjectString, _ := json.MarshalIndent(scaledObject, "", "   ")
			fmt.Fprintf(GinkgoWriter, "%s\n", scaledObjectString)
			fmt.Println("step 22")
			Expect(scaledObject.OwnerReferences[0].Name).Should(Equal(rollout.Name))
			Expect(scaledObject.OwnerReferences[0].Kind).Should(Equal("Rollout"))
			Expect(scaledObject.OwnerReferences[0].APIVersion).Should(Equal("argoproj.io/v1alpha1"))
			Expect(scaledObject.Spec.ScaleTargetRef.Name).Should(Equal(rollout.Name))
			Expect(scaledObject.Spec.ScaleTargetRef.Kind).Should(Equal("Rollout"))
			Expect(scaledObject.Spec.ScaleTargetRef.APIVersion).Should(Equal("argoproj.io/v1alpha1"))
			Expect(int(*scaledObject.Spec.MaxReplicaCount)).Should(Equal(policyReco.Spec.CurrentHPAConfiguration.Max))
			Expect(int(*scaledObject.Spec.MinReplicaCount)).Should(Equal(policyReco.Spec.CurrentHPAConfiguration.Min))
			Expect(len(scaledObject.Spec.Triggers)).Should(Equal(2))
			Expect(scaledObject.Spec.Triggers[0].Type).Should(Equal("cpu"))
			n, _ := strconv.Atoi(scaledObject.Spec.Triggers[0].Metadata["value"])
			Expect(n).Should(Equal((policyReco.Spec.CurrentHPAConfiguration.TargetMetricValue)))

			updatedPolicyReco = &v1alpha1.PolicyRecommendation{}
			Eventually(func() bool {
				err := k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)
				if err != nil {
					return false
				}
				for _, v := range updatedPolicyReco.Status.Conditions {
					if v.Type == string(v1alpha1.HPAEnforced) {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())

			updatedPolicyReco.Spec.CurrentHPAConfiguration.Min = 1
			updatedPolicyReco.Spec.CurrentHPAConfiguration.Max = 2

			Expect(k8sClient.Update(context.TODO(), updatedPolicyReco)).To(Succeed())

			rollout := &argov1alpha1.Rollout{}
			Eventually(func() int {
				err := k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, rollout)
				if err != nil {
					return -1
				}
				return int(*rollout.Spec.Replicas)
			}, timeout, interval).Should(Equal(initialMax))

			scaledObject = &kedaapi.ScaledObject{}
			Eventually(k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject), timeout, interval).ShouldNot(Succeed())
		})
		/*
			It("Should delete the ScaledObject for a Rollout when marked with skip annotation", func() {
				initialMax := 100
				policyReco = &v1alpha1.PolicyRecommendation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: v1alpha1.PolicyRecommendationSpec{
						WorkloadMeta: v1alpha1.WorkloadMeta{
							TypeMeta: metav1.TypeMeta{
								Kind:       "Rollout",
								APIVersion: "argoproj.io/v1alpha1",
							},
							Name: HPAEnforcerPolicyRecoName,
						},
						TargetHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               60,
							Max:               100,
							TargetMetricValue: 50,
						},
						CurrentHPAConfiguration: v1alpha1.HPAConfiguration{
							Min:               20,
							Max:               initialMax,
							TargetMetricValue: 40,
						},
						Policy:             "random",
						QueuedForExecution: &falseBool,
					},
				}
				Expect(k8sClient.Create(context.TODO(), policyReco)).To(Succeed())
				updatedPolicyReco := &v1alpha1.PolicyRecommendation{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)
				replicas := int32(10)
				rollout = &argov1alpha1.Rollout{
					ObjectMeta: metav1.ObjectMeta{
						Name:      HPAEnforcerPolicyRecoName,
						Namespace: HPAEnforcerPolicyRecoNamespace,
					},
					Spec: argov1alpha1.RolloutSpec{
						Replicas: &replicas,
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test-app",
							},
						},
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "test-app",
								},
							},
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{
										Name:  "test-container",
										Image: "nginx:1.17.5",
									},
								},
							},
						}},
				}
				Expect(k8sClient.Create(context.TODO(), rollout)).To(Succeed())
				updatedPolicyReco.Status = v1alpha1.PolicyRecommendationStatus{
					Conditions: []metav1.Condition{
						{
							Type:               string(v1alpha1.Initialized),
							Status:             metav1.ConditionTrue,
							LastTransitionTime: metav1.Now(),
							Reason:             PolicyRecommendationCreated,
							Message:            InitializedMessage,
						},
						{
							Type:               string(v1alpha1.RecoTaskProgress),
							Status:             metav1.ConditionFalse,
							Reason:             RecoTaskRecommendationGenerated,
							Message:            RecommendationGeneratedMessage,
							LastTransitionTime: metav1.Now(),
						},
					}}
				Expect(k8sClient.Status().Update(context.TODO(), updatedPolicyReco)).To(Succeed())
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, updatedPolicyReco)).To(Succeed())
				updatedPolicyReco.OwnerReferences = []metav1.OwnerReference{
					{
						APIVersion: "argoproj.io/v1alpha1",
						Kind:       "Rollout",
						Name:       HPAEnforcerPolicyRecoName,
						UID:        rollout.UID,
					},
				}
				Expect(k8sClient.Update(context.TODO(), updatedPolicyReco)).To(Succeed())
				updatedPolicyRecoString, _ = json.MarshalIndent(updatedPolicyReco, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", updatedPolicyRecoString)

				scaledObject = &kedaapi.ScaledObject{}
				Eventually(k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject), interval, timeout).Should(Succeed())
				scaledObjectString, _ := json.MarshalIndent(scaledObject, "", "   ")
				fmt.Fprintf(GinkgoWriter, "%s\n", scaledObjectString)
				Expect(scaledObject.OwnerReferences[0].Name).Should(Equal(rollout.Name))
				Expect(scaledObject.OwnerReferences[0].Kind).Should(Equal("Rollout"))
				Expect(scaledObject.OwnerReferences[0].APIVersion).Should(Equal("argoproj.io/v1alpha1"))
				Expect(scaledObject.Spec.ScaleTargetRef.Name).Should(Equal(rollout.Name))
				Expect(scaledObject.Spec.ScaleTargetRef.Kind).Should(Equal("Rollout"))
				Expect(scaledObject.Spec.ScaleTargetRef.APIVersion).Should(Equal("argoproj.io/v1alpha1"))
				Expect(int(*scaledObject.Spec.MaxReplicaCount)).Should(Equal(policyReco.Spec.CurrentHPAConfiguration.Max))
				Expect(int(*scaledObject.Spec.MinReplicaCount)).Should(Equal(policyReco.Spec.CurrentHPAConfiguration.Min))
				Expect(len(scaledObject.Spec.Triggers)).Should(Equal(2))
				Expect(scaledObject.Spec.Triggers[0].Type).Should(Equal("cpu"))
				n, _ := strconv.Atoi(scaledObject.Spec.Triggers[0].Metadata["value"])
				Expect(n).Should(Equal((policyReco.Spec.CurrentHPAConfiguration.TargetMetricValue)))

				updatedPolicyReco = &v1alpha1.PolicyRecommendation{}
				Eventually(func() bool {
					err := k8sClient.Get(context.TODO(), types.NamespacedName{
						Namespace: HPAEnforcerPolicyRecoNamespace,
						Name:      HPAEnforcerPolicyRecoName,
					}, updatedPolicyReco)
					if err != nil {
						return false
					}
					for _, v := range updatedPolicyReco.Status.Conditions {
						if v.Type == string(v1alpha1.HPAEnforced) {
							return true
						}
					}
					return false
				}, timeout, interval).Should(BeTrue())

				rollout = &argov1alpha1.Rollout{}
				Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, rollout)).Should(Succeed())
				rollout.Annotations = map[string]string{hpaEnforcementDisabledAnnotation: "true"}
				Expect(k8sClient.Update(context.TODO(), rollout)).Should(Succeed())

				rollout = &argov1alpha1.Rollout{}
				k8sClient.Get(context.TODO(), types.NamespacedName{
					Namespace: HPAEnforcerPolicyRecoNamespace,
					Name:      HPAEnforcerPolicyRecoName,
				}, rollout)

				rollout := &argov1alpha1.Rollout{}
				Eventually(func() int {
					err := k8sClient.Get(context.TODO(), types.NamespacedName{
						Namespace: HPAEnforcerPolicyRecoNamespace,
						Name:      HPAEnforcerPolicyRecoName,
					}, rollout)
					if err != nil {
						return -1
					}
					return int(*rollout.Spec.Replicas)
				}, timeout, interval).Should(Equal(initialMax))

				scaledObject = &kedaapi.ScaledObject{}
				Eventually(k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: HPAEnforcerPolicyRecoNamespace, Name: HPAEnforcerPolicyRecoName}, scaledObject), timeout, interval).ShouldNot(Succeed())
			})

		*/
	})

})
