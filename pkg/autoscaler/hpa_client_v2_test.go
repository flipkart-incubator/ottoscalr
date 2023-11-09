package autoscaler

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("HPAClientV2", func() {
	Describe("GetMaxReplicaCount and GetScaleTargetName", func() {
		Context("when the HPA has a MaxReplicaCount and ScaleTargetName", func() {
			It("returns the MaxReplicaCount and ScaleTargetName", func() {
				// Create a new HPA object
				hpa := &autoscalingv2.HorizontalPodAutoscaler{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-hpa",
						Namespace: "default",
					},
					Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
						ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
							Name: "test-deployment",
							Kind: "Deployment",
						},
						MaxReplicas: *int32Ptr(5),
						MinReplicas: int32Ptr(1),
						Metrics: []autoscalingv2.MetricSpec{
							{
								Type: "Resource",
								Resource: &autoscalingv2.ResourceMetricSource{
									Name: "cpu",
									Target: autoscalingv2.MetricTarget{
										Type:               "Utilization",
										AverageUtilization: int32Ptr(10), // Target CPU utilization percentage
									},
								},
							},
						},
					},
				}

				err := k8sClient.Create(context.Background(), hpa)
				Expect(err).NotTo(HaveOccurred())

				// Call GetMaxReplicaCount with the hpa
				maxReplicaCount := hpaClientV2.GetMaxReplicaCount(hpa)
				Expect(maxReplicaCount).To(Equal(int32(5)))

				scaleTargetName := hpaClientV2.GetScaleTargetName(hpa)
				Expect(scaleTargetName).To(Equal("test-deployment"))

				err = k8sClient.Delete(context.Background(), hpa)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
	Describe("GetType", func() {
		It("should return correct type", func() {
			Expect(hpaClientV2.GetType()).To(Equal(&autoscalingv2.HorizontalPodAutoscaler{}))
		})
	})
	Describe("CreateOrUpdateAutoscaler", func() {
		var (
			deployment          *appsv1.Deployment
			deploymentName      = "test-deployment"
			deploymentNamespace = "default"
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

			err := k8sClient.Create(ctx, deployment)
			Expect(err).ToNot(HaveOccurred())

			time.Sleep(2 * time.Second)
		})
		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, deployment)).To(Succeed())
		})

		It("should create a new HPA if it is not present", func() {
			deployment := &appsv1.Deployment{}
			err := k8sClient.Get(ctx, types.NamespacedName{Namespace: deploymentNamespace, Name: deploymentName}, deployment)
			Expect(err).ToNot(HaveOccurred())
			op, err := hpaClientV2.CreateOrUpdateAutoscaler(ctx, deployment,
				map[string]string{"created-by": "ottoscalr"}, *int32Ptr(10), *int32Ptr(5), *int32Ptr(4))

			Expect(err).ToNot(HaveOccurred())
			time.Sleep(2 * time.Second)
			Expect(op).To(Equal("created"))
			hpa := &autoscalingv2.HorizontalPodAutoscaler{}
			err = k8sClient.Get(ctx, types.NamespacedName{Namespace: deploymentNamespace, Name: deploymentName}, hpa)
			Expect(err).ToNot(HaveOccurred())
			Expect(hpa.Spec.MaxReplicas).To(Equal(*int32Ptr(10)))
			Expect(hpa.Spec.MinReplicas).To(Equal(int32Ptr(5)))
			Expect(hpa.Spec.Metrics[0].Resource.Target.AverageUtilization).To(Equal(int32Ptr(4)))
			Expect(hpa.Spec.ScaleTargetRef.Name).To(Equal(deploymentName))
			Expect(k8sClient.Delete(ctx, hpa)).To(Succeed())

		})
		It("should update an existing HPA if it is present", func() {
			deployment := &appsv1.Deployment{}
			err := k8sClient.Get(ctx, types.NamespacedName{Namespace: deploymentNamespace, Name: deploymentName}, deployment)
			Expect(err).ToNot(HaveOccurred())

			op, err := hpaClientV2.CreateOrUpdateAutoscaler(ctx, deployment,
				map[string]string{"created-by": "ottoscalr"}, *int32Ptr(10), *int32Ptr(5), *int32Ptr(4))
			Expect(err).ToNot(HaveOccurred())
			time.Sleep(2 * time.Second)
			Expect(op).To(Equal("created"))
			hpa := &autoscalingv2.HorizontalPodAutoscaler{}
			err = k8sClient.Get(ctx, types.NamespacedName{Namespace: deploymentNamespace, Name: deploymentName}, hpa)
			Expect(err).ToNot(HaveOccurred())
			Expect(hpa.Spec.MaxReplicas).To(Equal(*int32Ptr(10)))
			Expect(hpa.Spec.MinReplicas).To(Equal(int32Ptr(5)))
			Expect(hpa.Spec.Metrics[0].Resource.Target.AverageUtilization).To(Equal(int32Ptr(4)))
			Expect(hpa.Spec.ScaleTargetRef.Name).To(Equal(deploymentName))

			op, err = hpaClientV2.CreateOrUpdateAutoscaler(ctx, deployment,
				map[string]string{"created-by": "ottoscalr"}, *int32Ptr(8), *int32Ptr(5), *int32Ptr(10))
			Expect(err).ToNot(HaveOccurred())
			time.Sleep(2 * time.Second)
			Expect(op).To(Equal("updated"))
			hpa = &autoscalingv2.HorizontalPodAutoscaler{}
			err = k8sClient.Get(ctx, types.NamespacedName{Namespace: deploymentNamespace, Name: deploymentName}, hpa)
			Expect(err).ToNot(HaveOccurred())
			Expect(hpa.Spec.MaxReplicas).To(Equal(*int32Ptr(8)))
			Expect(hpa.Spec.MinReplicas).To(Equal(int32Ptr(5)))
			Expect(hpa.Spec.Metrics[0].Resource.Target.AverageUtilization).To(Equal(int32Ptr(10)))
			Expect(hpa.Spec.ScaleTargetRef.Name).To(Equal(deploymentName))
			Expect(k8sClient.Delete(ctx, hpa)).To(Succeed())

		})
	})
})
