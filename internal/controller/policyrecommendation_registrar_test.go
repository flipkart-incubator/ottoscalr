package controller

import (
	rolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	ottoscaleriov1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/net/context"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"time"
)

// +kubebuilder:docs-gen:collapse=Imports

var _ = Describe("PolicyRecommendationRegistrar controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		RolloutName         = "test-rollout"
		RolloutNamespace    = "default"
		DeploymentName      = "test-deployment"
		DeploymentNamespace = "default"

		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When creating a new Rollout", func() {
		It("Should Create a new PolicyRecommendation", func() {
			By("By creating a new Rollout")
			ctx := context.Background()
			rollout := &rolloutv1alpha1.Rollout{
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

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: RolloutName, Namespace: RolloutNamespace},
					createdRollout)
				if err != nil {
					return false
				}

				err = k8sClient.Get(ctx, types.NamespacedName{Name: RolloutName, Namespace: RolloutNamespace},
					createdPolicy)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			Expect(createdRollout.Name).Should(Equal(RolloutName))

			Expect(createdPolicy.Name).Should(Equal(RolloutName))
			Expect(createdPolicy.Namespace).Should(Equal(RolloutNamespace))
			Expect(createdPolicy.OwnerReferences[0].Name).Should(Equal(RolloutName))
			Expect(createdPolicy.OwnerReferences[0].Kind).Should(Equal("Rollout"))
			Expect(createdPolicy.OwnerReferences[0].APIVersion).Should(Equal("argoproj.io/v1alpha1"))
		})
	})

	Context("When creating a new Deployment", func() {
		It("Should Create a new PolicyRecommendation", func() {
			By("By creating a new Deployment")
			ctx := context.Background()
			deployment := &appsv1.Deployment{
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
			createdPolicy := &ottoscaleriov1alpha1.PolicyRecommendation{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: DeploymentName, Namespace: DeploymentNamespace},
					createdDeployment)
				if err != nil {
					return false
				}

				err = k8sClient.Get(ctx, types.NamespacedName{Name: DeploymentName, Namespace: DeploymentNamespace},
					createdPolicy)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			Expect(createdDeployment.Name).Should(Equal(DeploymentName))

			Expect(createdPolicy.Name).Should(Equal(DeploymentName))
			Expect(createdPolicy.Namespace).Should(Equal(DeploymentNamespace))
			Expect(createdPolicy.OwnerReferences[0].Name).Should(Equal(DeploymentName))
			Expect(createdPolicy.OwnerReferences[0].Kind).Should(Equal("Deployment"))
			Expect(createdPolicy.OwnerReferences[0].APIVersion).Should(Equal("apps/v1"))
		})
	})

})
