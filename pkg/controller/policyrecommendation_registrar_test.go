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

		timeout  = time.Minute
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
		It("Should Create a new PolicyRecommendation", func() {
			By("By creating a new Rollout")
			ctx := context.TODO()
			rollout = &rolloutv1alpha1.Rollout{
				ObjectMeta: metav1.ObjectMeta{
					Name:      RolloutName,
					Namespace: RolloutNamespace,
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
	})

	Context("When creating a new Deployment", func() {
		var deployment *appsv1.Deployment
		var createdPolicy *ottoscaleriov1alpha1.PolicyRecommendation

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, deployment)).Should(Succeed())
		})
		It("Should Create a new PolicyRecommendation", func() {
			By("By creating a new Deployment")
			ctx := context.TODO()
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
	})

	When("A PolicyRecommendation is deleted", func() {
		var deployment *appsv1.Deployment
		var createdPolicy *ottoscaleriov1alpha1.PolicyRecommendation

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, deployment)).Should(Succeed())
		})
		It("Should create a PolicyRecommendation again when deleted", func() {
			By("By reconciling the owner deployment")
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
			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DeploymentName,
					Namespace: "namespace1",
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

			time.Sleep(3 * time.Second)
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: DeploymentName, Namespace: DeploymentNamespace},
				createdDeployment)).Should(Succeed())
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
})
