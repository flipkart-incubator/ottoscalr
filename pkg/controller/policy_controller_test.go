package controller

import (
	"context"
	"fmt"
	"time"

	rolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	ottoscaleriov1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	"github.com/flipkart-incubator/ottoscalr/pkg/reco"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("PolicyWatcher controller", func() {
	const (
		timeout             = time.Second * 10
		interval            = time.Millisecond * 250
		PolicyRecoName      = "test-deployment-afgre"
		PolicyRecoNamespace = "default"
	)

	BeforeEach(func() {
		queuedAllRecos = false
	})
	var policy1, policy2, policy3 ottoscaleriov1alpha1.Policy
	Context("When updating default policy", func() {

		It("Should mark other policies as non-default and requeue all policy recommendations ", func() {
			By("Seeding all policies")
			ctx := context.TODO()

			policy1 = ottoscaleriov1alpha1.Policy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "policy1",
					Namespace: "default",
				},
				Spec: ottoscaleriov1alpha1.PolicySpec{
					IsDefault: true,
				},
			}
			policy2 = ottoscaleriov1alpha1.Policy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "policy2",
					Namespace: "default",
				},
				Spec: ottoscaleriov1alpha1.PolicySpec{
					IsDefault: false,
				},
			}
			policy3 = ottoscaleriov1alpha1.Policy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "policy3",
					Namespace: "default",
				},
				Spec: ottoscaleriov1alpha1.PolicySpec{
					IsDefault: false,
				},
			}

			Expect(k8sClient.Create(ctx, &policy1)).Should(Succeed())
			Expect(k8sClient.Create(ctx, &policy2)).Should(Succeed())
			Expect(k8sClient.Create(ctx, &policy3)).Should(Succeed())

			time.Sleep(2 * time.Second)
			policy2 = ottoscaleriov1alpha1.Policy{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "policy2"}, &policy2)).Should(Succeed())
			policy2.Spec.IsDefault = true
			err := k8sClient.Update(ctx, &policy2)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() bool {
				updatedPolicy1 := ottoscaleriov1alpha1.Policy{}
				updatedPolicy2 := ottoscaleriov1alpha1.Policy{}
				updatedPolicy3 := ottoscaleriov1alpha1.Policy{}
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: "default",
					Name: "policy1"}, &updatedPolicy1)
				if err != nil {
					return false
				}

				err = k8sClient.Get(ctx, types.NamespacedName{Namespace: "default",
					Name: "policy2"}, &updatedPolicy2)
				if err != nil {

					return false
				}

				err = k8sClient.Get(ctx, types.NamespacedName{Namespace: "default",
					Name: "policy3"}, &updatedPolicy3)
				if err != nil {
					return false
				}
				if updatedPolicy1.Spec.IsDefault || !updatedPolicy2.Spec.IsDefault || updatedPolicy3.Spec.IsDefault {
					return false
				}

				return true
			}, timeout, interval).Should(BeTrue())

			By("Testing that queuedAllRecos was called")
			Eventually(Expect(queuedAllRecos).Should(BeTrue()))

			Expect(k8sClient.Delete(ctx, &policy1)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, &policy2)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, &policy3)).Should(Succeed())
			time.Sleep(1 * time.Second)
		})
	})
	Context("When handling Add/Update/Delete on a Policy", func() {
		policyreco1 := &ottoscaleriov1alpha1.PolicyRecommendation{}
		policyreco2 := &ottoscaleriov1alpha1.PolicyRecommendation{}

		It("Should add finalizer if not present and requeue all policyrecommendations having this policy ", func() {
			By("Seeding all policies")
			ctx := context.TODO()

			policy1 = ottoscaleriov1alpha1.Policy{
				ObjectMeta: metav1.ObjectMeta{Name: "policy1", Namespace: "default"},
				Spec: ottoscaleriov1alpha1.PolicySpec{
					IsDefault:               false,
					RiskIndex:               10,
					MinReplicaPercentageCut: 100,
					TargetUtilization:       15,
				},
			}
			policy2 = ottoscaleriov1alpha1.Policy{
				ObjectMeta: metav1.ObjectMeta{Name: "policy2", Namespace: "default"},
				Spec: ottoscaleriov1alpha1.PolicySpec{
					IsDefault:               true,
					RiskIndex:               20,
					MinReplicaPercentageCut: 100,
					TargetUtilization:       20,
				},
			}
			policy3 = ottoscaleriov1alpha1.Policy{
				ObjectMeta: metav1.ObjectMeta{Name: "policy3", Namespace: "default"},
				Spec: ottoscaleriov1alpha1.PolicySpec{
					IsDefault:               false,
					RiskIndex:               30,
					MinReplicaPercentageCut: 100,
					TargetUtilization:       30,
				},
			}

			Expect(k8sClient.Create(ctx, &policy1)).Should(Succeed())
			Expect(k8sClient.Create(ctx, &policy2)).Should(Succeed())
			Expect(k8sClient.Create(ctx, &policy3)).Should(Succeed())

			time.Sleep(2 * time.Second)

			//check if finalizer is added
			policy1 = ottoscaleriov1alpha1.Policy{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "policy1"}, &policy1)).Should(Succeed())
			Expect(policy1.Finalizers).ShouldNot(BeNil())
			Expect(policy1.Finalizers).Should(ContainElement(policyFinalizerName))

			policy2 = ottoscaleriov1alpha1.Policy{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "policy2"}, &policy2)).Should(Succeed())
			Expect(policy2.Finalizers).ShouldNot(BeNil())
			Expect(policy2.Finalizers).Should(ContainElement(policyFinalizerName))

			policy3 = ottoscaleriov1alpha1.Policy{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "policy3"}, &policy3)).Should(Succeed())
			Expect(policy3.Finalizers).ShouldNot(BeNil())
			Expect(policy3.Finalizers).Should(ContainElement(policyFinalizerName))

			//create two policy recommendations for policy2
			now := metav1.Now()
			policyreco1 = &ottoscaleriov1alpha1.PolicyRecommendation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PolicyRecoName,
					Namespace: PolicyRecoNamespace,
				},
				Spec: ottoscaleriov1alpha1.PolicyRecommendationSpec{
					WorkloadMeta: ottoscaleriov1alpha1.WorkloadMeta{
						Name: PolicyRecoName,
					},
					Policy:             policy2.Name,
					GeneratedAt:        &now,
					TransitionedAt:     &now,
					QueuedForExecution: &falseBool,
				},
			}
			policyreco2 = &ottoscaleriov1alpha1.PolicyRecommendation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PolicyRecoName + "2",
					Namespace: PolicyRecoNamespace,
				},
				Spec: ottoscaleriov1alpha1.PolicyRecommendationSpec{
					WorkloadMeta: ottoscaleriov1alpha1.WorkloadMeta{
						Name: PolicyRecoName + "2",
					},
					Policy:             policy2.Name,
					GeneratedAt:        &now,
					TransitionedAt:     &now,
					QueuedForExecution: &falseBool,
				},
			}

			Expect(k8sClient.Create(ctx, policyreco1)).Should(Succeed())
			Expect(k8sClient.Create(ctx, policyreco2)).Should(Succeed())

			time.Sleep(2 * time.Second)

			//update policy2 now
			policy2.Spec.RiskIndex = 4
			queuedOneReco = nil
			err := k8sClient.Update(ctx, &policy2)
			Expect(err).ToNot(HaveOccurred())

			time.Sleep(2 * time.Second)
			//check if policyreco1 and policyreco2 are queued for execution
			Expect(len(queuedOneReco)).Should(Equal(2))
			Expect(queuedOneReco[0]).Should(Equal(true))
			Expect(queuedOneReco[1]).Should(Equal(true))

			//delete policy2 now, again check if policyreco1 and policyreco2 are queued for execution
			queuedOneReco = nil
			policy2 := ottoscaleriov1alpha1.Policy{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "policy2"}, &policy2)).Should(Succeed())
			fmt.Println(policy2)
			Expect(k8sClient.Delete(ctx, &policy2)).Should(Succeed())
			time.Sleep(5 * time.Second)
			Expect(len(queuedOneReco)).Should(Equal(2))
			Expect(queuedOneReco[0]).Should(Equal(true))
			Expect(queuedOneReco[1]).Should(Equal(true))

			Expect(k8sClient.Delete(ctx, &policy1)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, &policy3)).Should(Succeed())
		})

		It("Should update the DefaultPolicyAnnotation on all the Deployments/Rollouts if the present policy annotation is deleted", func() {
			By("Seeding all policies")
			ctx := context.TODO()

			policy1 = ottoscaleriov1alpha1.Policy{
				ObjectMeta: metav1.ObjectMeta{Name: "policy4", Namespace: "default"},
				Spec: ottoscaleriov1alpha1.PolicySpec{
					IsDefault:               false,
					RiskIndex:               10,
					MinReplicaPercentageCut: 100,
					TargetUtilization:       15,
				},
			}
			policy2 = ottoscaleriov1alpha1.Policy{
				ObjectMeta: metav1.ObjectMeta{Name: "policy5", Namespace: "default"},
				Spec: ottoscaleriov1alpha1.PolicySpec{
					IsDefault:               false,
					RiskIndex:               20,
					MinReplicaPercentageCut: 100,
					TargetUtilization:       20,
				},
			}
			policy3 = ottoscaleriov1alpha1.Policy{
				ObjectMeta: metav1.ObjectMeta{Name: "policy6", Namespace: "default"},
				Spec: ottoscaleriov1alpha1.PolicySpec{
					IsDefault:               true,
					RiskIndex:               30,
					MinReplicaPercentageCut: 100,
					TargetUtilization:       30,
				},
			}

			Expect(k8sClient.Create(ctx, &policy1)).Should(Succeed())
			Expect(k8sClient.Create(ctx, &policy2)).Should(Succeed())
			Expect(k8sClient.Create(ctx, &policy3)).Should(Succeed())

			time.Sleep(2 * time.Second)

			//create one deployment with defaultPolicyAnnotation
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deploymentpl",
					Namespace: "default",
					Annotations: map[string]string{
						reco.DefaultPolicyAnnotation: policy2.Name,
					},
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
			//Create one deployment with defaultPolicyAnnotation
			deployment1 := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deploymentpl1",
					Namespace: "default",
					Annotations: map[string]string{
						reco.DefaultPolicyAnnotation: policy3.Name,
					},
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test-app1",
						},
					},
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test-app1",
							},
						},
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name:  "test-container1",
									Image: "nginx:1.17.5",
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, deployment1)).Should(Succeed())
			//Create one rollout with defaultPolicyAnnotation
			rollout := &rolloutv1alpha1.Rollout{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rolloutpl",
					Namespace: "default",
					Annotations: map[string]string{
						reco.DefaultPolicyAnnotation: policy2.Name,
					},
				},

				Spec: rolloutv1alpha1.RolloutSpec{

					Template: v1.PodTemplateSpec{
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
			Expect(k8sClient.Create(ctx, rollout)).Should(Succeed())
			//Create one rollout without defaultPolicyAnnotation
			rollout1 := &rolloutv1alpha1.Rollout{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rolloutpl1",
					Namespace: "default",
				},

				Spec: rolloutv1alpha1.RolloutSpec{

					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name:  "test-container1",
									Image: "nginx:1.17.5",
								},
							},
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, rollout1)).Should(Succeed())

			policyToBeDeleted := ottoscaleriov1alpha1.Policy{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "policy5", Namespace: "default"}, &policyToBeDeleted)).Should(Succeed())

			Expect(k8sClient.Delete(ctx, &policyToBeDeleted)).Should(Succeed())

			time.Sleep(5 * time.Second)
			deployment = &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-deploymentpl", Namespace: "default"}, deployment)).Should(Succeed())
			Expect(deployment.Annotations[reco.DefaultPolicyAnnotation]).Should(Equal(policy1.Name))
			deployment1 = &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-deploymentpl1", Namespace: "default"}, deployment1)).Should(Succeed())
			Expect(deployment1.Annotations[reco.DefaultPolicyAnnotation]).Should(Equal(policy3.Name))
			rollout = &rolloutv1alpha1.Rollout{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-rolloutpl", Namespace: "default"}, rollout)).Should(Succeed())
			Expect(rollout.Annotations[reco.DefaultPolicyAnnotation]).Should(Equal(policy1.Name))
			rollout1 = &rolloutv1alpha1.Rollout{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-rolloutpl1", Namespace: "default"}, rollout1)).Should(Succeed())
			//Rollout should have no annotation
			Expect(rollout1.Annotations[reco.DefaultPolicyAnnotation]).Should(Equal(""))
		})

	})
})
