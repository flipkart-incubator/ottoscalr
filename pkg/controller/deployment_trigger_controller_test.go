package controller

import (
	"context"
	"encoding/json"
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

//+kubebuilder:docs-gen:collapse=Imports

var _ = Describe("Deployment controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		RolloutName         = "test-rollout"
		RolloutNamespace    = "default"
		DeploymentName      = "test-deployment"
		DeploymentNamespace = "default"

		timeout  = time.Minute
		interval = time.Millisecond * 250
	)

	Context("When updating the max pod annotation for a Rollout", func() {
		var rollout *rolloutv1alpha1.Rollout
		var policyreco *ottoscaleriov1alpha1.PolicyRecommendation
		now := metav1.Now()
		ctx := context.TODO()
		BeforeEach(func() {
			rollout = &rolloutv1alpha1.Rollout{
				ObjectMeta: metav1.ObjectMeta{
					Name:      RolloutName,
					Namespace: RolloutNamespace,
					Annotations: map[string]string{
						"ottoscalr.io/max-pods": "60",
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
			policyreco = &ottoscaleriov1alpha1.PolicyRecommendation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      RolloutName,
					Namespace: RolloutNamespace,
				},
				Spec: ottoscaleriov1alpha1.PolicyRecommendationSpec{
					WorkloadMeta: ottoscaleriov1alpha1.WorkloadMeta{
						Name: RolloutName,
					},
					Policy:               "test-policy",
					GeneratedAt:          &now,
					TransitionedAt:       &now,
					QueuedForExecution:   &falseBool,
					QueuedForExecutionAt: &now,
				},
			}
			Expect(k8sClient1.Create(ctx, policyreco)).Should(Succeed())

			Expect(k8sClient1.Create(ctx, rollout)).Should(Succeed())

		})
		AfterEach(func() {
			Expect(k8sClient1.Delete(ctx, rollout)).Should(Succeed())
			Expect(k8sClient1.Delete(ctx, policyreco)).Should(Succeed())
		})
		It("Should requeue the policyreco", func() {
			By("By updating the max pod annotation for a Rollout")

			createdRollout := &rolloutv1alpha1.Rollout{}
			createdPolicy := &ottoscaleriov1alpha1.PolicyRecommendation{}

			time.Sleep(5 * time.Second)

			Eventually(func() bool {
				err := k8sClient1.Get(ctx,
					types.NamespacedName{Name: RolloutName, Namespace: RolloutNamespace},
					createdRollout)
				if err != nil {
					return false
				}

				err = k8sClient1.Get(ctx,
					types.NamespacedName{Name: RolloutName, Namespace: RolloutNamespace},
					createdPolicy)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			Expect(createdRollout.Name).Should(Equal(RolloutName))

			createdPolicyString, _ := json.MarshalIndent(createdPolicy, "", "   ")
			fmt.Fprintf(GinkgoWriter, "%s\n", createdPolicyString)

			Expect(createdPolicy.Spec.QueuedForExecution).Should(Equal(&falseBool))

			createdRollout.Annotations["ottoscalr.io/max-pods"] = "50"
			Expect(k8sClient1.Update(ctx, createdRollout)).Should(Succeed())
			k8sClient1.Get(ctx,
				types.NamespacedName{Name: RolloutName, Namespace: RolloutNamespace},
				createdRollout)
			testString, _ := json.MarshalIndent(createdRollout, "", "   ")
			fmt.Fprintf(GinkgoWriter, "Rollout updated: %s\n", testString)
			updatedPolicy := &ottoscaleriov1alpha1.PolicyRecommendation{}

			time.Sleep(5 * time.Second)

			k8sClient1.Get(ctx,
				types.NamespacedName{Name: RolloutName, Namespace: RolloutNamespace},
				updatedPolicy)
			updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicy, "", "   ")
			fmt.Fprintf(GinkgoWriter, "Updated: %s\n", updatedPolicyRecoString)

			Expect(updatedPolicy.Spec.QueuedForExecution).Should(Equal(&trueBool))

		})
	})

	Context("When updating the max pod annotation for a Deployment", func() {
		var deployment *appsv1.Deployment
		var policyreco *ottoscaleriov1alpha1.PolicyRecommendation
		ctx := context.TODO()
		now := metav1.Now()
		BeforeEach(func() {
			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DeploymentName,
					Namespace: DeploymentNamespace,
					Annotations: map[string]string{
						"ottoscalr.io/max-pods": "60",
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

			policyreco = &ottoscaleriov1alpha1.PolicyRecommendation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DeploymentName,
					Namespace: DeploymentNamespace,
				},
				Spec: ottoscaleriov1alpha1.PolicyRecommendationSpec{
					WorkloadMeta: ottoscaleriov1alpha1.WorkloadMeta{
						Name: DeploymentName,
					},
					Policy:               "test-policy",
					GeneratedAt:          &now,
					TransitionedAt:       &now,
					QueuedForExecution:   &falseBool,
					QueuedForExecutionAt: &now,
				},
			}
			Expect(k8sClient1.Create(ctx, policyreco)).Should(Succeed())

			Expect(k8sClient1.Create(ctx, deployment)).Should(Succeed())
		})
		AfterEach(func() {
			Expect(k8sClient1.Delete(ctx, deployment)).Should(Succeed())
			Expect(k8sClient1.Delete(ctx, policyreco)).Should(Succeed())
		})
		It("Should requeue the policyreco", func() {
			By("By updating the max pod annotation for a Deployment")

			createdDeployment := &appsv1.Deployment{}
			createdPolicy := &ottoscaleriov1alpha1.PolicyRecommendation{}

			time.Sleep(5 * time.Second)

			Eventually(func() bool {
				err := k8sClient1.Get(ctx,
					types.NamespacedName{Name: DeploymentName, Namespace: DeploymentNamespace},
					createdDeployment)
				if err != nil {
					return false
				}

				err = k8sClient1.Get(ctx,
					types.NamespacedName{Name: DeploymentName, Namespace: DeploymentNamespace},
					createdPolicy)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			Expect(createdDeployment.Name).Should(Equal(DeploymentName))

			createdPolicyString, _ := json.MarshalIndent(createdPolicy, "", "   ")
			fmt.Fprintf(GinkgoWriter, "%s\n", createdPolicyString)

			Expect(createdPolicy.Spec.QueuedForExecution).Should(Equal(&falseBool))

			createdDeployment.Annotations["ottoscalr.io/max-pods"] = "50"
			Expect(k8sClient1.Update(ctx, createdDeployment)).Should(Succeed())
			updatedPolicy := &ottoscaleriov1alpha1.PolicyRecommendation{}

			time.Sleep(5 * time.Second)

			k8sClient1.Get(ctx,
				types.NamespacedName{Name: DeploymentName, Namespace: DeploymentNamespace},
				updatedPolicy)
			updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicy, "", "   ")
			fmt.Fprintf(GinkgoWriter, "Updated: %s\n", updatedPolicyRecoString)

			Expect(updatedPolicy.Spec.QueuedForExecution).Should(Equal(&trueBool))

		})
	})

	Context("When updating the any other annotation for a Deployment", func() {
		var deployment *appsv1.Deployment
		var policyreco *ottoscaleriov1alpha1.PolicyRecommendation
		ctx := context.TODO()
		now := metav1.Now()
		BeforeEach(func() {
			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DeploymentName,
					Namespace: DeploymentNamespace,
					Annotations: map[string]string{
						"ottoscalr.io/max-pods": "60",
						"test-annotation":       "true",
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

			policyreco = &ottoscaleriov1alpha1.PolicyRecommendation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DeploymentName,
					Namespace: DeploymentNamespace,
				},
				Spec: ottoscaleriov1alpha1.PolicyRecommendationSpec{
					WorkloadMeta: ottoscaleriov1alpha1.WorkloadMeta{
						Name: DeploymentName,
					},
					Policy:               "test-policy",
					GeneratedAt:          &now,
					TransitionedAt:       &now,
					QueuedForExecution:   &falseBool,
					QueuedForExecutionAt: &now,
				},
			}
			Expect(k8sClient1.Create(ctx, policyreco)).Should(Succeed())

			Expect(k8sClient1.Create(ctx, deployment)).Should(Succeed())
		})
		AfterEach(func() {
			Expect(k8sClient1.Delete(ctx, deployment)).Should(Succeed())
			Expect(k8sClient1.Delete(ctx, policyreco)).Should(Succeed())
		})
		It("Should not requeue the policyreco", func() {
			By("By updating the any other annotation instead of max pod annotation for a Deployment")

			createdDeployment := &appsv1.Deployment{}
			createdPolicy := &ottoscaleriov1alpha1.PolicyRecommendation{}

			time.Sleep(5 * time.Second)

			Eventually(func() bool {
				err := k8sClient1.Get(ctx,
					types.NamespacedName{Name: DeploymentName, Namespace: DeploymentNamespace},
					createdDeployment)
				if err != nil {
					return false
				}

				err = k8sClient1.Get(ctx,
					types.NamespacedName{Name: DeploymentName, Namespace: DeploymentNamespace},
					createdPolicy)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			Expect(createdDeployment.Name).Should(Equal(DeploymentName))

			createdPolicyString, _ := json.MarshalIndent(createdPolicy, "", "   ")
			fmt.Fprintf(GinkgoWriter, "%s\n", createdPolicyString)

			Expect(createdPolicy.Spec.QueuedForExecution).Should(Equal(&falseBool))

			createdDeployment.Annotations["test-annotation"] = "false"
			Expect(k8sClient1.Update(ctx, createdDeployment)).Should(Succeed())
			updatedPolicy := &ottoscaleriov1alpha1.PolicyRecommendation{}

			time.Sleep(5 * time.Second)

			k8sClient1.Get(ctx,
				types.NamespacedName{Name: DeploymentName, Namespace: DeploymentNamespace},
				updatedPolicy)
			updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicy, "", "   ")
			fmt.Fprintf(GinkgoWriter, "Updated: %s\n", updatedPolicyRecoString)

			Expect(updatedPolicy.Spec.QueuedForExecution).Should(Equal(&falseBool))

		})
	})

	Context("When updating the default annotation for a Deployment", func() {
		var deployment *appsv1.Deployment
		var policyreco *ottoscaleriov1alpha1.PolicyRecommendation
		ctx := context.TODO()
		now := metav1.Now()
		BeforeEach(func() {
			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DeploymentName,
					Namespace: DeploymentNamespace,
					Annotations: map[string]string{
						"ottoscalr.io/max-pods": "60",
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

			policyreco = &ottoscaleriov1alpha1.PolicyRecommendation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DeploymentName,
					Namespace: DeploymentNamespace,
				},
				Spec: ottoscaleriov1alpha1.PolicyRecommendationSpec{
					WorkloadMeta: ottoscaleriov1alpha1.WorkloadMeta{
						Name: DeploymentName,
					},
					Policy:               "test-policy",
					GeneratedAt:          &now,
					TransitionedAt:       &now,
					QueuedForExecution:   &falseBool,
					QueuedForExecutionAt: &now,
				},
			}
			Expect(k8sClient1.Create(ctx, policyreco)).Should(Succeed())

			Expect(k8sClient1.Create(ctx, deployment)).Should(Succeed())
		})
		AfterEach(func() {
			Expect(k8sClient1.Delete(ctx, deployment)).Should(Succeed())
			Expect(k8sClient1.Delete(ctx, policyreco)).Should(Succeed())
		})
		It("Should requeue the policyreco", func() {
			By("By updating the max pod annotation for a Deployment")

			createdDeployment := &appsv1.Deployment{}
			createdPolicy := &ottoscaleriov1alpha1.PolicyRecommendation{}

			time.Sleep(5 * time.Second)

			Eventually(func() bool {
				err := k8sClient1.Get(ctx,
					types.NamespacedName{Name: DeploymentName, Namespace: DeploymentNamespace},
					createdDeployment)
				if err != nil {
					return false
				}

				err = k8sClient1.Get(ctx,
					types.NamespacedName{Name: DeploymentName, Namespace: DeploymentNamespace},
					createdPolicy)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			Expect(createdDeployment.Name).Should(Equal(DeploymentName))

			createdPolicyString, _ := json.MarshalIndent(createdPolicy, "", "   ")
			fmt.Fprintf(GinkgoWriter, "%s\n", createdPolicyString)

			Expect(createdPolicy.Spec.QueuedForExecution).Should(Equal(&falseBool))

			createdDeployment.Annotations[reco.DefaultPolicyAnnotation] = "p-target-30"
			Expect(k8sClient1.Update(ctx, createdDeployment)).Should(Succeed())
			updatedPolicy := &ottoscaleriov1alpha1.PolicyRecommendation{}

			time.Sleep(5 * time.Second)

			k8sClient1.Get(ctx,
				types.NamespacedName{Name: DeploymentName, Namespace: DeploymentNamespace},
				updatedPolicy)
			updatedPolicyRecoString, _ := json.MarshalIndent(updatedPolicy, "", "   ")
			fmt.Fprintf(GinkgoWriter, "Updated: %s\n", updatedPolicyRecoString)

			Expect(updatedPolicy.Spec.QueuedForExecution).Should(Equal(&trueBool))

		})
	})

})
