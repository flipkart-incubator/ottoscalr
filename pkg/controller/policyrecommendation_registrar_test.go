package controller

import (
	"context"
	"fmt"
	"time"

	rolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	ottoscaleriov1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

//+kubebuilder:docs-gen:collapse=Imports

var _ = Describe("PolicyRecommendationRegistrar controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		RolloutName         = "test-rollout"
		RolloutNamespace    = "default"
		DeploymentName      = "test-deployment"
		DeploymentNamespace = "default"

		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	BeforeEach(func() {
		queuedAllRecos = false
		DeferCleanup(func() {
			queuedAllRecos = false
		})
	})

	Context("When creating a new Rollout", func() {
		var rollout *rolloutv1alpha1.Rollout
		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, rollout)).Should(Succeed())
		})
		It("Should Create a new PolicyRecommendation if replicas is more than minRequiredToGeneratePolicy", func() {
			By("By creating a new Rollout")
			ctx := context.TODO()
			replicas := int32(5)
			rollout = &rolloutv1alpha1.Rollout{
				ObjectMeta: metav1.ObjectMeta{
					Name:      RolloutName,
					Namespace: RolloutNamespace,
				},

				Spec: rolloutv1alpha1.RolloutSpec{
					Replicas: &replicas,
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
			createdRollout := &rolloutv1alpha1.Rollout{}
			createdPolicy := &ottoscaleriov1alpha1.PolicyRecommendation{}

			time.Sleep(5 * time.Second)

			Eventually(func() bool {
				err := k8sClient.Get(ctx,
					types.NamespacedName{Name: RolloutName, Namespace: RolloutNamespace},
					createdRollout)
				if err != nil {
					return false
				}

				err = k8sClient.Get(ctx,
					types.NamespacedName{Name: RolloutName, Namespace: RolloutNamespace},
					createdPolicy)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			Expect(createdRollout.Name).Should(Equal(RolloutName))

			fmt.Fprintf(GinkgoWriter, "PolicyReco: %v", createdPolicy)
			Expect(createdPolicy.Name).Should(Equal(RolloutName))
			Expect(createdPolicy.Namespace).Should(Equal(RolloutNamespace))
			Expect(createdPolicy.Spec.Policy).Should(Equal("safest-policy"))
			Expect(createdPolicy.OwnerReferences[0].Name).Should(Equal(RolloutName))
			Expect(createdPolicy.OwnerReferences[0].Kind).Should(Equal("Rollout"))
			Expect(createdPolicy.OwnerReferences[0].APIVersion).Should(Equal("argoproj.io/v1alpha1"))

			By("Testing that monitor has been queuedAllRecos")
			Eventually(Expect(queuedAllRecos).Should(BeTrue()))
		})
		It("Should not create a new PolicyRecommendation if replicas is less than or equal to minRequiredToGeneratePolicy", func() {
			By("By creating a new Rollout")
			ctx := context.TODO()
			replicas := int32(3)
			rollout = &rolloutv1alpha1.Rollout{
				ObjectMeta: metav1.ObjectMeta{
					Name:      RolloutName + "-low-replicas",
					Namespace: RolloutNamespace,
				},

				Spec: rolloutv1alpha1.RolloutSpec{
					Replicas: &replicas,
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
			createdPolicy := &ottoscaleriov1alpha1.PolicyRecommendation{}

			time.Sleep(2 * time.Second)
			Eventually(func() bool {
				err := k8sClient.Get(ctx,
					types.NamespacedName{Name: RolloutName + "-low-replicas", Namespace: RolloutNamespace},
					createdPolicy)
				return err == nil
			}, timeout, interval).Should(BeFalse())
		})
	})

	Context("When creating a new Deployment", func() {
		var deployment *appsv1.Deployment
		var createdPolicy *ottoscaleriov1alpha1.PolicyRecommendation

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, deployment)).Should(Succeed())
		})
		It("Should Create a new PolicyRecommendation if replicas is more than minRequiredToGeneratePolicy", func() {
			By("By creating a new Deployment")
			ctx := context.TODO()
			replicas := int32(5)
			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DeploymentName,
					Namespace: DeploymentNamespace,
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
					},
				},
			}
			Expect(k8sClient.Create(ctx, deployment)).Should(Succeed())
			createdDeployment := &appsv1.Deployment{}
			createdPolicy = &ottoscaleriov1alpha1.PolicyRecommendation{}

			time.Sleep(5 * time.Second)

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
			Expect(createdPolicy.Name).Should(Equal(DeploymentName))
			Expect(createdPolicy.Namespace).Should(Equal(DeploymentNamespace))
			Expect(createdPolicy.Spec.Policy).Should(Equal("safest-policy"))
			Expect(createdPolicy.OwnerReferences[0].Name).Should(Equal(DeploymentName))
			Expect(createdPolicy.OwnerReferences[0].Kind).Should(Equal("Deployment"))
			Expect(createdPolicy.OwnerReferences[0].APIVersion).Should(Equal("apps/v1"))

			fmt.Fprintf(GinkgoWriter, "PolicyReco: %v", createdPolicy)
			Expect(createdPolicy.Status.Conditions).To(ContainElement(SatisfyAll(
				HaveField("Type", Equal("Initialized")))))

			By("Testing that monitor has been queuedAllRecos")
			Eventually(Expect(queuedAllRecos).Should(BeTrue()))
		})

		It("Should not create a new PolicyRecommendation if replicas is less than or equal to minRequiredToGeneratePolicy", func() {
			By("By creating a new Deployment")
			ctx := context.TODO()
			replicas := int32(3)
			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DeploymentName + "-low-replicas",
					Namespace: DeploymentNamespace,
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
					},
				},
			}
			Expect(k8sClient.Create(ctx, deployment)).Should(Succeed())
			createdPolicy = &ottoscaleriov1alpha1.PolicyRecommendation{}

			time.Sleep(2 * time.Second)

			Eventually(func() bool {
				err := k8sClient.Get(ctx,
					types.NamespacedName{Name: DeploymentName + "-low-replicas", Namespace: DeploymentNamespace},
					createdPolicy)
				return err == nil
			}, timeout, interval).Should(BeFalse())
		})
	})

	When("A PolicyRecommendation is deleted", func() {
		var deployment *appsv1.Deployment
		var createdPolicy *ottoscaleriov1alpha1.PolicyRecommendation

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, deployment)).Should(Succeed())
		})
		It("Should create a PolicyRecommendation again when deleted if replicas is greater than minRequiredToGeneratePolicy", func() {
			By("By reconciling the owner deployment")
			ctx := context.Background()
			replicas := int32(5)
			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DeploymentName,
					Namespace: DeploymentNamespace,
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
					},
				},
			}
			Expect(k8sClient.Create(ctx, deployment)).Should(Succeed())
			createdDeployment := &appsv1.Deployment{}
			createdPolicy = &ottoscaleriov1alpha1.PolicyRecommendation{}

			time.Sleep(5 * time.Second)

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

			Expect(createdPolicy.Name).Should(Equal(DeploymentName))
			Expect(createdPolicy.Namespace).Should(Equal(DeploymentNamespace))
			Expect(createdPolicy.Spec.Policy).Should(Equal("safest-policy"))
			Expect(createdPolicy.OwnerReferences[0].Name).Should(Equal(DeploymentName))
			Expect(createdPolicy.OwnerReferences[0].Kind).Should(Equal("Deployment"))
			Expect(createdPolicy.OwnerReferences[0].APIVersion).Should(Equal("apps/v1"))

			By("Testing that monitor has been queuedAllRecos")
			Eventually(Expect(queuedAllRecos).Should(BeTrue()))

			Expect(k8sClient.Delete(ctx, createdPolicy)).Should(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx,
					types.NamespacedName{Name: DeploymentName, Namespace: DeploymentNamespace},
					createdPolicy)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			Expect(createdPolicy.Name).Should(Equal(DeploymentName))
			Expect(createdPolicy.Namespace).Should(Equal(DeploymentNamespace))
			Expect(createdPolicy.Spec.Policy).Should(Equal("safest-policy"))
			Expect(createdPolicy.OwnerReferences[0].Name).Should(Equal(DeploymentName))
			Expect(createdPolicy.OwnerReferences[0].Kind).Should(Equal("Deployment"))
			Expect(createdPolicy.OwnerReferences[0].APIVersion).Should(Equal("apps/v1"))
		})

	})

	Context("When creating a new Deployment in the excluded namespace", func() {
		var deployment *appsv1.Deployment
		var namespace *v1.Namespace
		var createdPolicy *ottoscaleriov1alpha1.PolicyRecommendation
		ctx := context.TODO()

		BeforeEach(func() {
			namespace = &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "namespace1",
				},
			}
			Expect(k8sClient.Create(ctx, namespace)).Should(Succeed())
			time.Sleep(5 * time.Second)
		})

		AfterEach(func() {
			time.Sleep(time.Second * 5)
			Expect(k8sClient.Delete(ctx, deployment)).Should(Succeed())
		})
		It("Should Not Create a new PolicyRecommendation", func() {
			By("By creating a new Deployment")
			replicas := int32(5)
			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DeploymentName,
					Namespace: "namespace1",
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
					},
				},
			}
			Expect(k8sClient.Create(ctx, deployment)).Should(Succeed())
			createdDeployment := &appsv1.Deployment{}
			createdPolicy = &ottoscaleriov1alpha1.PolicyRecommendation{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx,
					types.NamespacedName{Name: DeploymentName, Namespace: "namespace1"},
					createdDeployment)
				if err != nil {
					return false
				}

				err = k8sClient.Get(ctx,
					types.NamespacedName{Name: DeploymentName, Namespace: "namespace1"},
					createdPolicy)
				if !errors.IsNotFound(err) {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			Expect(createdDeployment.Name).Should(Equal(DeploymentName))

		})
	})

	Context("When updating a existing Deployment in the included namespace", func() {
		var deployment *appsv1.Deployment
		var createdPolicy *ottoscaleriov1alpha1.PolicyRecommendation
		var policyAfterDeploymentUpdate *ottoscaleriov1alpha1.PolicyRecommendation
		ctx := context.TODO()

		BeforeEach(func() {
			replicas := int32(5)
			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DeploymentName,
					Namespace: DeploymentNamespace,
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
					},
				},
			}
			Expect(k8sClient.Create(ctx, deployment)).Should(Succeed())

		})

		AfterEach(func() {
			time.Sleep(time.Second * 5)
			Expect(k8sClient.Delete(ctx, deployment)).Should(Succeed())
		})
		It("Should not Create a new PolicyRecommendation", func() {
			By("By creating a new Deployment")

			createdDeployment := &appsv1.Deployment{}
			createdPolicy = &ottoscaleriov1alpha1.PolicyRecommendation{}
			policyAfterDeploymentUpdate = &ottoscaleriov1alpha1.PolicyRecommendation{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: DeploymentName, Namespace: DeploymentNamespace},
				createdDeployment)).Should(Succeed())

			time.Sleep(5 * time.Second)
			Expect(k8sClient.Get(ctx,
				types.NamespacedName{Name: DeploymentName, Namespace: DeploymentNamespace},
				createdPolicy)).Should(Succeed())

			labels := map[string]string{"app1": "test-app-1"}
			createdDeployment.Labels = labels

			Expect(k8sClient.Update(ctx, createdDeployment)).Should(Succeed())

			time.Sleep(10 * time.Second)

			Expect(k8sClient.Get(ctx,
				types.NamespacedName{Name: DeploymentName, Namespace: DeploymentNamespace},
				policyAfterDeploymentUpdate)).Should(Succeed())

			var time1, time2 metav1.Time
			for _, c := range createdPolicy.Status.Conditions {
				if c.Type == string(ottoscaleriov1alpha1.Initialized) {
					time1 = c.LastTransitionTime
				}
			}
			for _, c := range policyAfterDeploymentUpdate.Status.Conditions {
				if c.Type == string(ottoscaleriov1alpha1.Initialized) {
					time2 = c.LastTransitionTime
				}
			}
			Expect(time1).Should(Equal(time2))
			Expect(createdDeployment.Name).Should(Equal(DeploymentName))

		})
	})

	Context("When updating a new Deployment", func() {
		var deployment *appsv1.Deployment
		var createdPolicy *ottoscaleriov1alpha1.PolicyRecommendation

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, deployment)).Should(Succeed())
		})
		It("Should Create a new PolicyRecommendation if previous replicas was less than minRequiredToGeneratePolicy and updated replicas is more than minRequiredToGeneratePolicy ", func() {
			By("By creating a new Deployment")
			ctx := context.TODO()
			replicas := int32(3)
			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DeploymentName + "-update-replicas",
					Namespace: DeploymentNamespace,
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
					},
				},
			}
			Expect(k8sClient.Create(ctx, deployment)).Should(Succeed())
			createdDeployment := &appsv1.Deployment{}
			createdPolicy = &ottoscaleriov1alpha1.PolicyRecommendation{}

			time.Sleep(2 * time.Second)

			err := k8sClient.Get(ctx,
				types.NamespacedName{Name: DeploymentName + "-update-replicas", Namespace: DeploymentNamespace},
				createdPolicy)
			Expect(err).To(HaveOccurred())

			err = k8sClient.Get(ctx,
				types.NamespacedName{Name: DeploymentName + "-update-replicas", Namespace: DeploymentNamespace},
				createdDeployment)
			Expect(err).ToNot(HaveOccurred())

			newReplicas := int32(7)
			createdDeployment.Spec.Replicas = &newReplicas
			err = k8sClient.Update(ctx, createdDeployment)
			Expect(err).ToNot(HaveOccurred())

			time.Sleep(2 * time.Second)

			Eventually(func() bool {
				err := k8sClient.Get(ctx,
					types.NamespacedName{Name: DeploymentName + "-update-replicas", Namespace: DeploymentNamespace},
					createdPolicy)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
			Expect(createdDeployment.Name).Should(Equal(DeploymentName + "-update-replicas"))
			Expect(createdPolicy.Name).Should(Equal(DeploymentName + "-update-replicas"))
			Expect(createdPolicy.Namespace).Should(Equal(DeploymentNamespace))
			Expect(createdPolicy.Spec.Policy).Should(Equal("safest-policy"))
			Expect(createdPolicy.OwnerReferences[0].Name).Should(Equal(DeploymentName + "-update-replicas"))
			Expect(createdPolicy.OwnerReferences[0].Kind).Should(Equal("Deployment"))
			Expect(createdPolicy.OwnerReferences[0].APIVersion).Should(Equal("apps/v1"))

			fmt.Fprintf(GinkgoWriter, "PolicyReco: %v", createdPolicy)
			Expect(createdPolicy.Status.Conditions).To(ContainElement(SatisfyAll(
				HaveField("Type", Equal("Initialized")))))

			By("Testing that monitor has been queuedAllRecos")
			Eventually(Expect(queuedAllRecos).Should(BeTrue()))
		})

		It("Should Delete the PolicyRecommendation if previous replicas was more than minRequiredToGeneratePolicy and updated replicas is less than minRequiredToGeneratePolicy", func() {
			By("By creating a new Deployment")
			ctx := context.TODO()
			replicas := int32(5)
			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DeploymentName + "-update-replicas-1",
					Namespace: DeploymentNamespace,
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
					},
				},
			}
			Expect(k8sClient.Create(ctx, deployment)).Should(Succeed())
			createdPolicy = &ottoscaleriov1alpha1.PolicyRecommendation{}

			time.Sleep(2 * time.Second)

			Eventually(func() bool {
				err := k8sClient.Get(ctx,
					types.NamespacedName{Name: DeploymentName + "-update-replicas-1", Namespace: DeploymentNamespace},
					createdPolicy)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
			Expect(createdPolicy.Name).Should(Equal(DeploymentName + "-update-replicas-1"))
			Expect(createdPolicy.Namespace).Should(Equal(DeploymentNamespace))
			Expect(createdPolicy.Spec.Policy).Should(Equal("safest-policy"))
			Expect(createdPolicy.OwnerReferences[0].Name).Should(Equal(DeploymentName + "-update-replicas-1"))
			Expect(createdPolicy.OwnerReferences[0].Kind).Should(Equal("Deployment"))
			Expect(createdPolicy.OwnerReferences[0].APIVersion).Should(Equal("apps/v1"))

			newReplicas := int32(2)
			createdDeployment := &appsv1.Deployment{}
			err := k8sClient.Get(ctx,
				types.NamespacedName{Name: DeploymentName + "-update-replicas-1", Namespace: DeploymentNamespace},
				createdDeployment)
			Expect(err).ToNot(HaveOccurred())

			createdDeployment.Spec.Replicas = &newReplicas
			err = k8sClient.Update(ctx, createdDeployment)
			Expect(err).ToNot(HaveOccurred())

			time.Sleep(2 * time.Second)

			Eventually(func() bool {
				err := k8sClient.Get(ctx,
					types.NamespacedName{Name: DeploymentName + "-update-replicas-1", Namespace: DeploymentNamespace},
					createdPolicy)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})
	})
	Context("When PolicyRecommendation Exists and replicas are less than minRequiredToGeneratePolicy", func() {
		var deployment *appsv1.Deployment
		var createdPolicy *ottoscaleriov1alpha1.PolicyRecommendation

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, deployment)).Should(Succeed())
		})
		It("Should Delete the policyrecommendation ", func() {
			By("By creating a new Deployment")
			ctx := context.TODO()
			createdPolicy = &ottoscaleriov1alpha1.PolicyRecommendation{}

			time.Sleep(2 * time.Second)

			err := k8sClient.Get(ctx,
				types.NamespacedName{Name: DeploymentName + "-update-replicas-2", Namespace: DeploymentNamespace},
				createdPolicy)
			Expect(err).To(HaveOccurred())

			//Create PolicyReco manually
			now := metav1.Now()
			policyreco := &ottoscaleriov1alpha1.PolicyRecommendation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DeploymentName + "-update-replicas-2",
					Namespace: DeploymentNamespace,
				},
				Spec: ottoscaleriov1alpha1.PolicyRecommendationSpec{
					WorkloadMeta: ottoscaleriov1alpha1.WorkloadMeta{
						Name: DeploymentName + "-update-replicas-2",
					},
					Policy:             "policy-1",
					GeneratedAt:        &now,
					TransitionedAt:     &now,
					QueuedForExecution: &falseBool,
				},
			}
			Expect(k8sClient.Create(ctx, policyreco)).Should(Succeed())
			Expect(k8sClient.Get(ctx,
				types.NamespacedName{Name: DeploymentName + "-update-replicas-2", Namespace: DeploymentNamespace},
				createdPolicy),
			).Should(Succeed())

			// Create deployment to trigger reconcile
			replicas := int32(1)
			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DeploymentName + "-update-replicas-2",
					Namespace: DeploymentNamespace,
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
					},
				},
			}
			Expect(k8sClient.Create(ctx, deployment)).Should(Succeed())
			createdDeployment := &appsv1.Deployment{}

			err = k8sClient.Get(ctx,
				types.NamespacedName{Name: DeploymentName + "-update-replicas-2", Namespace: DeploymentNamespace},
				createdDeployment)
			Expect(err).ToNot(HaveOccurred())

			time.Sleep(2 * time.Second)

			Eventually(func() bool {
				err := k8sClient.Get(ctx,
					types.NamespacedName{Name: DeploymentName + "-update-replicas-2", Namespace: DeploymentNamespace},
					createdPolicy)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})
	})
})
