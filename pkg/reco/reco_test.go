package reco

import (
	"context"
	"fmt"
	rolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/flipkart-incubator/ottoscalr/pkg/metrics"
	kedaapi "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

var _ = Describe("CpuUtilizationBasedRecommender", func() {

	Describe("findOptimalTargetUtilization", func() {
		It("should return the optimal target utilization", func() {
			dataPoints := []metrics.DataPoint{
				{Timestamp: time.Now().Add(-10 * time.Minute), Value: 60},
				{Timestamp: time.Now().Add(-9 * time.Minute), Value: 80},
				{Timestamp: time.Now().Add(-8 * time.Minute), Value: 100},
				{Timestamp: time.Now().Add(-7 * time.Minute), Value: 50},
				{Timestamp: time.Now().Add(-6 * time.Minute), Value: 30},
			}
			acl := 5 * time.Minute
			minTarget := 10
			maxTarget := 60
			perPodResources := 8.2

			optimalTarget, min, max, err := recommender.findOptimalTargetUtilization(
				dataPoints, acl, minTarget, maxTarget, perPodResources, 24)

			Expect(err).To(Not(HaveOccurred()))
			Expect(optimalTarget).To(Equal(48))
			Expect(min).To(Equal(7))
			Expect(max).To(Equal(24))
		})
	})

	var _ = Describe("SimulateHPA", func() {
		var (
			dataPoints        []metrics.DataPoint
			acl               time.Duration
			targetUtilization int
		)

		BeforeEach(func() {

			t1 := time.Now()
			dataPoints = []metrics.DataPoint{
				{
					Timestamp: t1,
					Value:     70,
				},
				{
					Timestamp: t1.Add(5 * time.Minute),
					Value:     90,
				},
				{
					Timestamp: t1.Add(10 * time.Minute),
					Value:     90,
				},
				{
					Timestamp: t1.Add(15 * time.Minute),
					Value:     60,
				},
				{
					Timestamp: t1.Add(20 * time.Minute),
					Value:     90,
				},
				{
					Timestamp: t1.Add(25 * time.Minute),
					Value:     120,
				},
			}

			acl = 8 * time.Minute
			targetUtilization = 60
		})

		Context("with valid inputs", func() {
			It("should simulate HPA correctly", func() {
				simulatedDataPoints, min, max, err := recommender.simulateHPA(dataPoints, acl, targetUtilization, 8.2, 23)
				Expect(err).NotTo(HaveOccurred())

				Expect(simulatedDataPoints).ToNot(BeNil())
				Expect(len(simulatedDataPoints)).To(Equal(len(dataPoints)))
				fmt.Fprintf(GinkgoWriter, "Simulated: %v\n", simulatedDataPoints)
				expectedSimulatedResources := []float64{90.61, 90.61, 90.61, 83.63999999999999, 83.63999999999999, 83.63999999999999}
				for i, simulatedDataPoint := range simulatedDataPoints {
					Expect(simulatedDataPoint.Timestamp).To(Equal(dataPoints[i].Timestamp))
					Expect(simulatedDataPoint.Value).To(Equal(expectedSimulatedResources[i]))
				}
				Expect(min).To(Equal(12))
				Expect(max).To(Equal(23))
			})
		})

		Context("with edge cases", func() {
			It("should handle empty dataPoints", func() {
				dataPoints = []metrics.DataPoint{}

				simulatedDataPoints, _, _, err := recommender.simulateHPA(dataPoints, acl, targetUtilization, 8.2, 24)
				Expect(err).NotTo(HaveOccurred())
				Expect(simulatedDataPoints).ToNot(BeNil())
				Expect(len(simulatedDataPoints)).To(Equal(0))
			})

			It("should handle zero targetUtilization", func() {
				targetUtilization = 0

				_, _, _, err := recommender.simulateHPA(dataPoints, acl, targetUtilization, 8.2, 24)
				Expect(err).To(HaveOccurred())
			})
		})
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

	Describe("getContainerCPULimitsSum", func() {
		var (
			deploymentNamespace = "default"
			deploymentName      = "test-deployment"
			rolloutNamespace    = "default"
			rolloutName         = "test-rollout"
			rollout             *rolloutv1alpha1.Rollout
			deployment          *appsv1.Deployment
			deploymentPod       *corev1.Pod
			rolloutPod          *corev1.Pod
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
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test-app1",
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

			deploymentPod = &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Pod",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment-pod",
					Namespace: deploymentNamespace,
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
			}

			err = k8sClient.Create(ctx, deploymentPod)
			Expect(err).ToNot(HaveOccurred())

			rolloutPod = &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Pod",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rollout-pod",
					Namespace: rolloutNamespace,
					Labels: map[string]string{
						"app": "test-app1",
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
									corev1.ResourceCPU: resource.MustParse("0.2"),
								},
							},
						},
					},
				},
			}

			err = k8sClient.Create(ctx, rolloutPod)
			Expect(err).ToNot(HaveOccurred())

		})

		AfterEach(func() {
			err := k8sClient.Delete(ctx, rollout)
			Expect(err).ToNot(HaveOccurred())
			err = k8sClient.Delete(ctx, deployment)
			Expect(err).ToNot(HaveOccurred())
			err = k8sClient.Delete(ctx, deploymentPod)
			Expect(err).ToNot(HaveOccurred())
			err = k8sClient.Delete(ctx, rolloutPod)
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

	Describe("getMaxPods", func() {
		var (
			deploymentNamespace = "default"
			deploymentName      = "test-deployment"
			deploymentName1     = "test-deployment1"
			rolloutNamespace    = "default"
			rolloutName         = "test-rollout"
			rolloutName1        = "test-rollout1"
			rollout             *rolloutv1alpha1.Rollout
			rollout1            *rolloutv1alpha1.Rollout
			deployment          *appsv1.Deployment
			deployment1         *appsv1.Deployment
			scaledObj           kedaapi.ScaledObject
			scaledObj1          kedaapi.ScaledObject
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
							},
						},
					},
				},
			}
			err := k8sClient.Create(ctx, rollout)
			Expect(err).ToNot(HaveOccurred())

			replicas := int32(4)
			rollout1 = &rolloutv1alpha1.Rollout{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "argoproj.io/v1alpha1",
					Kind:       "Rollout",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      rolloutName1,
					Namespace: rolloutNamespace,
				},
				Spec: rolloutv1alpha1.RolloutSpec{
					Replicas: &replicas,
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
							},
						},
					},
				},
			}
			err = k8sClient.Create(ctx, rollout1)
			Expect(err).ToNot(HaveOccurred())

			deployment = &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      deploymentName,
					Namespace: deploymentNamespace,
					Annotations: map[string]string{
						OttoscalrMaxPodAnnotation: "3",
					},
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
							},
						},
					},
				},
			}

			err = k8sClient.Create(ctx, deployment)
			Expect(err).ToNot(HaveOccurred())

			deployment1 = &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      deploymentName1,
					Namespace: deploymentNamespace,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas,
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
							},
						},
					},
				},
			}

			err = k8sClient.Create(ctx, deployment1)
			Expect(err).ToNot(HaveOccurred())

			min := int32(5)
			max := int32(10)
			scaledObj = kedaapi.ScaledObject{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "keda.sh/v1alpha1",
					Kind:       "ScaledObject",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-scaledobject",
					Namespace: rolloutNamespace,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "v1alpha1",
						Kind:       "Rollout",
						Name:       rolloutName,
						UID:        "d8790740-8b18-4a43-a352-66417f3ca65c",
					}},
				},
				Spec: kedaapi.ScaledObjectSpec{
					ScaleTargetRef: &kedaapi.ScaleTarget{
						Name:       rolloutName,
						APIVersion: "argoproj.io/v1alpha1",
						Kind:       "Rollout",
					},
					MinReplicaCount: &min,
					MaxReplicaCount: &max,
					Triggers:        []kedaapi.ScaleTriggers{},
				},
			}

			err = k8sClient.Create(ctx, &scaledObj)
			Expect(err).ToNot(HaveOccurred())

			scaledObj1 = kedaapi.ScaledObject{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "keda.sh/v1alpha1",
					Kind:       "ScaledObject",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-scaledobject1",
					Namespace: rolloutNamespace,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "v1alpha1",
						Kind:       "Rollout",
						Name:       rolloutName1,
						UID:        "d8790740-8b18-4a43-a352-66417f3ca65c",
					}},
					Labels: map[string]string{
						CreatedByLabelKey: "ottoscalr",
					},
				},
				Spec: kedaapi.ScaledObjectSpec{
					ScaleTargetRef: &kedaapi.ScaleTarget{
						Name:       rolloutName1,
						APIVersion: "argoproj.io/v1alpha1",
						Kind:       "Rollout",
					},
					MinReplicaCount: &min,
					MaxReplicaCount: &max,
					Triggers:        []kedaapi.ScaleTriggers{},
				},
			}

			err = k8sClient.Create(ctx, &scaledObj1)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := k8sClient.Delete(ctx, rollout)
			Expect(err).ToNot(HaveOccurred())
			err = k8sClient.Delete(ctx, rollout1)
			Expect(err).ToNot(HaveOccurred())
			err = k8sClient.Delete(ctx, deployment)
			Expect(err).ToNot(HaveOccurred())
			err = k8sClient.Delete(ctx, deployment1)
			Expect(err).ToNot(HaveOccurred())
			err = k8sClient.Delete(ctx, &scaledObj)
			Expect(err).ToNot(HaveOccurred())
			err = k8sClient.Delete(ctx, &scaledObj1)
			Expect(err).ToNot(HaveOccurred())

		})

		It("should return the replicas from annotations for the deployment if the annotation is present", func() {
			maxReplicas, err := recommender1.getMaxPods(deploymentNamespace, "Deployment", deploymentName)
			Expect(err).To(BeNil())
			Expect(maxReplicas).To(Equal(3))
		})

		It("should return the scaledObject Max Replicas if ScaledObject is present which is not created by ottoscalr hpaEnforcer", func() {
			maxReplicas, err := recommender1.getMaxPods(rolloutNamespace, "Rollout", rolloutName)
			Expect(err).To(BeNil())
			Expect(maxReplicas).To(Equal(10))
		})

		It("should return the spec.replicas if no scaledobject is there which is not created by ottoscalr hpaEnforcer and no annotation is there", func() {
			maxReplicas, err := recommender1.getMaxPods(deploymentNamespace, "Deployment", deploymentName1)
			Expect(err).To(BeNil())
			Expect(maxReplicas).To(Equal(4))
		})

		It("should return the spec.replicas if scaledobject is there which is created by ottoscalr hpaEnforcer and no annotation is there", func() {
			maxReplicas, err := recommender1.getMaxPods(rolloutNamespace, "Rollout", rolloutName1)
			Expect(err).To(BeNil())
			Expect(maxReplicas).To(Equal(4))
		})
	})

	Describe("Recommend", func() {

		var (
			deploymentNamespace = "default"
			deploymentName      = "test-deployment"
			deployment          *appsv1.Deployment
			deploymentPod       *corev1.Pod
		)

		BeforeEach(func() {
			deployment = &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      deploymentName,
					Namespace: deploymentNamespace,
					Annotations: map[string]string{
						"ottoscalr.io/max-pods": "30",
					},
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
											corev1.ResourceCPU: resource.MustParse("8"),
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

			err := k8sClient.Create(ctx, deployment)
			Expect(err).ToNot(HaveOccurred())

			deploymentPod = &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Pod",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment-pod",
					Namespace: deploymentNamespace,
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
									corev1.ResourceCPU: resource.MustParse("8"),
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
			}

			err = k8sClient.Create(ctx, deploymentPod)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := k8sClient.Delete(ctx, deployment)
			Expect(err).ToNot(HaveOccurred())
			err = k8sClient.Delete(ctx, deploymentPod)
			Expect(err).ToNot(HaveOccurred())

		})
		It("should return recommend the optimal HPA configuration", func() {

			workloadSpec := WorkloadMeta{
				Name:      deploymentName,
				Namespace: deploymentNamespace,
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
			}
			hpaConfig, err := recommender.Recommend(context.TODO(), workloadSpec)

			Expect(err).To(Not(HaveOccurred()))
			Expect(hpaConfig.TargetMetricValue).To(Equal(48))
			Expect(hpaConfig.Min).To(Equal(7))
			Expect(hpaConfig.Max).To(Equal(30))
		})
	})
})
