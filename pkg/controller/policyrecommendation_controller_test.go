package controller

import (
	"context"
	"fmt"
	v1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

var _ = Describe("PolicyrecommendationController", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		RolloutName         = "test-rollout-cfxdw"
		RolloutNamespace    = "default"
		DeploymentName      = "test-deployment-cfxdw"
		DeploymentNamespace = "default"

		timeout   = time.Second * 10
		interval  = time.Millisecond * 250
		policyAge = 1 * time.Second
	)

	var safestPolicy, policy1, policy2, policy3, policy4 v1alpha1.Policy
	BeforeEach(func() {
		safestPolicy = v1alpha1.Policy{
			ObjectMeta: metav1.ObjectMeta{Name: "safest-policy"},
			Spec: v1alpha1.PolicySpec{
				IsDefault:               false,
				RiskIndex:               1,
				MinReplicaPercentageCut: 80,
				TargetUtilization:       10,
			},
		}
		policy1 = v1alpha1.Policy{
			ObjectMeta: metav1.ObjectMeta{Name: "policy-1"},
			Spec: v1alpha1.PolicySpec{
				IsDefault:               false,
				RiskIndex:               10,
				MinReplicaPercentageCut: 100,
				TargetUtilization:       15,
			},
		}
		policy2 = v1alpha1.Policy{
			ObjectMeta: metav1.ObjectMeta{Name: "policy-2"},
			Spec: v1alpha1.PolicySpec{
				IsDefault:               false,
				RiskIndex:               20,
				MinReplicaPercentageCut: 100,
				TargetUtilization:       20,
			},
		}
		policy3 = v1alpha1.Policy{
			ObjectMeta: metav1.ObjectMeta{Name: "policy-3"},
			Spec: v1alpha1.PolicySpec{
				IsDefault:               true,
				RiskIndex:               30,
				MinReplicaPercentageCut: 100,
				TargetUtilization:       30,
			},
		}
		policy4 = v1alpha1.Policy{
			ObjectMeta: metav1.ObjectMeta{Name: "policy-4"},
			Spec: v1alpha1.PolicySpec{
				IsDefault:               false,
				RiskIndex:               40,
				MinReplicaPercentageCut: 100,
				TargetUtilization:       40,
			},
		}
		Expect(k8sClient.Create(ctx, &safestPolicy)).Should(Succeed())
		Expect(k8sClient.Create(ctx, &policy1)).Should(Succeed())
		Expect(k8sClient.Create(ctx, &policy2)).Should(Succeed())
		Expect(k8sClient.Create(ctx, &policy3)).Should(Succeed())
		Expect(k8sClient.Create(ctx, &policy4)).Should(Succeed())
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(ctx, &safestPolicy)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, &policy1)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, &policy2)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, &policy3)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, &policy4)).Should(Succeed())
	})

	When("A deployment is created", func() {
		var deployment *appsv1.Deployment
		var createdPolicy, updatedPolicy *v1alpha1.PolicyRecommendation
		createdPolicy = &v1alpha1.PolicyRecommendation{}
		updatedPolicy = &v1alpha1.PolicyRecommendation{}

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, deployment)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, createdPolicy)).Should(Succeed())
			//Eventually(Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: createdPolicy.Namespace, Name: createdPolicy.Name}, createdPolicy)).Should(Not(Succeed())))
		})
		It("Creates a PolicyRecommendation", func() {
			By("Starting with the safest Policy", func() {
				ctx := context.Background()
				deployment = &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      DeploymentName,
						Namespace: DeploymentNamespace,
					},

					Spec: appsv1.DeploymentSpec{
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
						},
					},
				}
				Expect(k8sClient.Create(ctx, deployment)).Should(Succeed())
				createdDeployment := &appsv1.Deployment{}

				Eventually(func() bool {
					err := k8sClient.Get(ctx,
						types.NamespacedName{Name: DeploymentName, Namespace: DeploymentNamespace},
						createdDeployment)
					if err != nil {
						return false
					}

					err = k8sClient.Get(ctx,
						types.NamespacedName{Name: DeploymentName, Namespace: DeploymentNamespace},
						createdPolicy)
					if err != nil {
						return false
					}
					return true
				}, timeout, interval).Should(BeTrue())

				Expect(createdDeployment.Name).Should(Equal(DeploymentName))

				fmt.Fprintf(GinkgoWriter, "Created policy : %v", createdPolicy)
				Expect(createdPolicy.Name).Should(Equal(DeploymentName))
				Expect(createdPolicy.Namespace).Should(Equal(DeploymentNamespace))
				Expect(createdPolicy.Spec.Policy).Should(Equal("safest-policy"))
				Expect(createdPolicy.OwnerReferences[0].Name).Should(Equal(DeploymentName))
				Expect(createdPolicy.OwnerReferences[0].Kind).Should(Equal("Deployment"))
				Expect(createdPolicy.OwnerReferences[0].APIVersion).Should(Equal("apps/v1"))

				By("After the policy ages")
				time.Sleep(policyAge)

				now := metav1.Now()
				By("Requeuing the workload for a fresh reco")
				Expect(k8sClient.Patch(ctx, &v1alpha1.PolicyRecommendation{
					TypeMeta: metav1.TypeMeta{
						Kind:       "PolicyRecommendation",
						APIVersion: "ottoscaler.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      createdPolicy.Name,
						Namespace: createdPolicy.Namespace,
					},
					Spec: v1alpha1.PolicyRecommendationSpec{
						QueuedForExecution:   true,
						QueuedForExecutionAt: &now,
					},
				}, client.Apply, client.ForceOwnership, client.FieldOwner(POLICY_RECO_WORKFLOW_CTRL_NAME))).Should(Succeed())

				Eventually(func() bool {
					err := k8sClient.Get(ctx,
						types.NamespacedName{Name: DeploymentName, Namespace: DeploymentNamespace},
						createdDeployment)
					if err != nil {
						return false
					}

					err = k8sClient.Get(ctx,
						types.NamespacedName{Name: DeploymentName, Namespace: DeploymentNamespace},
						updatedPolicy)
					if err != nil {
						return false
					}
					return true
				}, timeout, interval).Should(BeTrue())
				//Eventually(Expect(k8sClient.Get(ctx, types.NamespacedName{Name: createdPolicy.Name, Namespace: createdPolicy.Namespace}, updatedPolicy)).Should(Succeed()))

				fmt.Fprintf(GinkgoWriter, "Updated policy : %v", updatedPolicy)
				Expect(updatedPolicy.Name).Should(Equal(createdPolicy.Name))
				Expect(updatedPolicy.Namespace).Should(Equal(createdPolicy.Namespace))
				Expect(updatedPolicy.Spec.Policy).Should(Equal("policy-1"))
				Expect(updatedPolicy.OwnerReferences[0].Name).Should(Equal(DeploymentName))
				Expect(updatedPolicy.OwnerReferences[0].Kind).Should(Equal("Deployment"))
				Expect(updatedPolicy.OwnerReferences[0].APIVersion).Should(Equal("apps/v1"))
			})

			//By("Refreshing the policy recommendation", func() {
			//
			//	By("After the policy ages")
			//	time.Sleep(policyAge)
			//
			//	Expect(k8sClient.Patch(ctx, &v1alpha1.PolicyRecommendation{
			//		ObjectMeta: metav1.ObjectMeta{
			//			Name:      createdPolicy.Name,
			//			Namespace: createdPolicy.Namespace,
			//		},
			//		Spec: v1alpha1.PolicyRecommendationSpec{
			//			QueuedForExecution:   true,
			//			QueuedForExecutionAt: metav1.Now(),
			//		},
			//	}, client.Apply, client.ForceOwnership)).Should(Succeed())
			//
			//	Eventually(Expect(k8sClient.Get(ctx, types.NamespacedName{Name: createdPolicy.Name, Namespace: createdPolicy.Namespace}, updatedPolicy)))
			//
			//	Expect(updatedPolicy.Name).Should(Equal(createdPolicy.Name))
			//	Expect(updatedPolicy.Namespace).Should(Equal(createdPolicy.Namespace))
			//	Expect(updatedPolicy.Spec.Policy).Should(Equal("policy-1"))
			//	Expect(updatedPolicy.OwnerReferences[0].Name).Should(Equal(DeploymentName))
			//	Expect(updatedPolicy.OwnerReferences[0].Kind).Should(Equal("Deployment"))
			//	Expect(updatedPolicy.OwnerReferences[0].APIVersion).Should(Equal("apps/v1"))
			//})
			//It("Should then progress to the next policy", func() {
			//
			//})
			//It("Should stop when the recommended HPA config is achieved", func() {
			//
			//})
			//It("Should not go beyond the default policy cap", func() {
			//
			//})
		})
	})
})
