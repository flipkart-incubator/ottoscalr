package reco

import (
	rolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/flipkart-incubator/ottoscalr/pkg/metrics"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

var _ = Describe("CpuUtilizationBasedRecommender", func() {

	Context("Recommend", func() {
		It("should return a valid HPA configuration based on the given workloadSpec", func() {
			//workloadSpec := v1alpha1.WorkloadSpec{
			//	Namespace: workloadNamespace,
			//	Name:      workloadName,
			//}
			//
			//hpaConfig, err := recommender.Recommend(workloadSpec)
			//Expect(err).To(BeNil())
			//Expect(hpaConfig).NotTo(BeNil())
			//Expect(hpaConfig.TargetMetricValue).To(BeNumerically(">=", minTarget))
			//Expect(hpaConfig.TargetMetricValue).To(BeNumerically("<=", maxTarget))
		})
	})

	Context("simulateHPA", func() {
		// Add test cases for the simulateHPA method
	})

	var _ = Describe("hasNoBreachOccurred", func() {
		var (
			original  []metrics.DataPoint
			simulated []metrics.DataPoint
		)

		Context("when no breaches occur", func() {
			BeforeEach(func() {

				t1 := time.Now()
				t2 := t1.Add(time.Second)
				t3 := t2.Add(time.Second)
				original = []metrics.DataPoint{
					{Timestamp: t1, Value: 100},
					{Timestamp: t2, Value: 200},
					{Timestamp: t3, Value: 300},
				}
				simulated = []metrics.DataPoint{
					{Timestamp: t1, Value: 150},
					{Timestamp: t2, Value: 250},
					{Timestamp: t3, Value: 350},
				}
			})

			It("should return true", func() {
				Expect(recommender.hasNoBreachOccurred(original, simulated)).To(BeTrue())
			})
		})

		Context("when a breach occurs", func() {
			BeforeEach(func() {
				t1 := time.Now()
				t2 := t1.Add(time.Second)
				t3 := t2.Add(time.Second)
				original = []metrics.DataPoint{
					{Timestamp: t1, Value: 100},
					{Timestamp: t2, Value: 200},
					{Timestamp: t3, Value: 300},
				}
				simulated = []metrics.DataPoint{
					{Timestamp: t1, Value: 150},
					{Timestamp: t2, Value: 180},
					{Timestamp: t3, Value: 350},
				}
			})

			It("should return false", func() {
				Expect(recommender.hasNoBreachOccurred(original, simulated)).To(BeFalse())
			})
		})
	})

	Context("findOptimalTargetUtilization", func() {
		// Add test cases for the findOptimalTargetUtilization method
	})

	var _ = Describe("getContainerCPULimitsSum", func() {
		var (
			deploymentNamespace = "default"
			deploymentName      = "test-deployment"
			rolloutNamespace    = "default"
			rolloutName         = "test-rollout"
			rollout             *rolloutv1alpha1.Rollout
			deployment          *appsv1.Deployment
		)

		BeforeEach(func() {
			rollout = &rolloutv1alpha1.Rollout{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "argoproj.io/v1alpha1",
					Kind:       "Rollout",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      rolloutName,
					Namespace: rolloutNamespace,
				},
				Spec: rolloutv1alpha1.RolloutSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "container-1",
									Image: "container-image",
									Resources: corev1.ResourceRequirements{
										Limits: corev1.ResourceList{
											corev1.ResourceCPU: resource.MustParse("1"),
										},
									},
								},
								{
									Name:  "container-2",
									Image: "container-image",
									Resources: corev1.ResourceRequirements{
										Limits: corev1.ResourceList{
											corev1.ResourceCPU: resource.MustParse("0.2"),
										},
									},
								},
							},
						},
					},
				},
			}
			err := k8sClient.Create(ctx, rollout)
			Expect(err).ToNot(HaveOccurred())

			deployment = &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      deploymentName,
					Namespace: deploymentNamespace,
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test-app",
						},
					},

					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test-app",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "container-1",
									Image: "container-image",
									Resources: corev1.ResourceRequirements{
										Limits: corev1.ResourceList{
											corev1.ResourceCPU: resource.MustParse("1"),
										},
									},
								},
								{
									Name:  "container-2",
									Image: "container-image",
									Resources: corev1.ResourceRequirements{
										Limits: corev1.ResourceList{
											corev1.ResourceCPU: resource.MustParse("0.5"),
										},
									},
								},
							},
						},
					},
				},
			}

			err = k8sClient.Create(ctx, deployment)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := k8sClient.Delete(ctx, rollout)
			Expect(err).ToNot(HaveOccurred())
			err = k8sClient.Delete(ctx, deployment)
			Expect(err).ToNot(HaveOccurred())

		})

		It("should return the correct sum of CPU limits for a Deployment", func() {
			actualSum, err := recommender.getContainerCPULimitsSum(deploymentNamespace, "Deployment", deploymentName)
			Expect(err).To(BeNil())
			Expect(actualSum).To(Equal(float64(1.5)))
		})

		It("should return the correct sum of CPU limits for a Rollout", func() {
			actualSum, err := recommender.getContainerCPULimitsSum(rolloutNamespace, "Rollout", rolloutName)
			Expect(err).To(BeNil())
			Expect(actualSum).To(Equal(float64(1.2)))
		})

		It("should return an error for an unsupported object kind", func() {
			_, err := recommender.getContainerCPULimitsSum(deploymentNamespace, "UnsupportedKind", deploymentName)
			Expect(err).NotTo(BeNil())
		})

		It("should return an error if the object is not found", func() {
			_, err := recommender.getContainerCPULimitsSum(deploymentNamespace, "Deployment", "non-existent-deployment")
			Expect(err).NotTo(BeNil())
		})
	})
})
