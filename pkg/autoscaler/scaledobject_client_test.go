package autoscaler

import (
	"context"
	"time"

	kedaapi "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("ScaledObjectClient", func() {
	Describe("GetMaxReplicaCount and GetScaleTargetName", func() {
		Context("when the ScaledObject has a MaxReplicaCount and ScaleTargetName", func() {
			It("returns the MaxReplicaCount and ScaleTargetName", func() {
				// Create a new ScaledObject object
				scaledObject := &kedaapi.ScaledObject{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-scaledobject",
						Namespace: "default",
					},
					Spec: kedaapi.ScaledObjectSpec{
						ScaleTargetRef: &kedaapi.ScaleTarget{
							Name: "test-deployment",
							Kind: "Deployment",
						},
						MaxReplicaCount: int32Ptr(5),
						Triggers: []kedaapi.ScaleTriggers{
							{
								Type: "cpu",
								Metadata: map[string]string{
									"type":  "Utilization",
									"value": "5",
								},
							},
						},
					},
				}
				err := k8sClient.Create(context.Background(), scaledObject)
				Expect(err).NotTo(HaveOccurred())

				// Call GetMaxReplicaCount with the ScaledObject
				maxReplicaCount := scaledObjectClient.GetMaxReplicaCount(scaledObject)
				Expect(maxReplicaCount).To(Equal(int32(5)))

				scaleTargetName := scaledObjectClient.GetScaleTargetName(scaledObject)
				Expect(scaleTargetName).To(Equal("test-deployment"))

				err = k8sClient.Delete(context.Background(), scaledObject)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
	Describe("GetType", func() {
		It("should return correct type", func() {
			Expect(scaledObjectClient.GetType()).To(Equal(&kedaapi.ScaledObject{}))
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

		It("should create a new ScaledObject if it is not present", func() {
			deployment := &appsv1.Deployment{}
			err := k8sClient.Get(ctx, types.NamespacedName{Namespace: deploymentNamespace, Name: deploymentName}, deployment)
			Expect(err).ToNot(HaveOccurred())
			_, err = scaledObjectClient.CreateOrUpdateAutoscaler(ctx, deployment,
				map[string]string{"created-by": "ottoscalr"}, *int32Ptr(10), *int32Ptr(5), *int32Ptr(4))

			Expect(err).ToNot(HaveOccurred())
			time.Sleep(2 * time.Second)
			//Expect(op).To(Equal("created"))
			scaledObject := &kedaapi.ScaledObject{}
			err = k8sClient.Get(ctx, types.NamespacedName{Namespace: deploymentNamespace, Name: deploymentName}, scaledObject)
			Expect(err).ToNot(HaveOccurred())
			Expect(scaledObject.Spec.MaxReplicaCount).To(Equal(int32Ptr(10)))
			Expect(scaledObject.Spec.MinReplicaCount).To(Equal(int32Ptr(5)))
			Expect(scaledObject.Spec.Triggers[0].Type).To(Equal("cpu"))
			Expect(scaledObject.Spec.Triggers[0].Metadata["type"]).To(Equal("Utilization"))
			Expect(scaledObject.Spec.Triggers[0].Metadata["value"]).To(Equal("4"))
			Expect(scaledObject.Spec.ScaleTargetRef.Name).To(Equal(deploymentName))
			Expect(k8sClient.Delete(ctx, scaledObject)).To(Succeed())

		})
		It("should update an existing ScaledObject if it is present", func() {
			deployment := &appsv1.Deployment{}
			err := k8sClient.Get(ctx, types.NamespacedName{Namespace: deploymentNamespace, Name: deploymentName}, deployment)
			Expect(err).ToNot(HaveOccurred())

			op, err := scaledObjectClient.CreateOrUpdateAutoscaler(ctx, deployment,
				map[string]string{"created-by": "ottoscalr"}, *int32Ptr(10), *int32Ptr(5), *int32Ptr(4))
			Expect(err).ToNot(HaveOccurred())
			time.Sleep(2 * time.Second)
			Expect(op).To(Equal("created"))
			scaledObject := &kedaapi.ScaledObject{}
			err = k8sClient.Get(ctx, types.NamespacedName{Namespace: deploymentNamespace, Name: deploymentName}, scaledObject)
			Expect(err).ToNot(HaveOccurred())
			Expect(scaledObject.Spec.MaxReplicaCount).To(Equal(int32Ptr(10)))
			Expect(scaledObject.Spec.MinReplicaCount).To(Equal(int32Ptr(5)))
			Expect(scaledObject.Spec.Triggers[0].Type).To(Equal("cpu"))
			Expect(scaledObject.Spec.Triggers[0].Metadata["type"]).To(Equal("Utilization"))
			Expect(scaledObject.Spec.Triggers[0].Metadata["value"]).To(Equal("4"))
			Expect(scaledObject.Spec.ScaleTargetRef.Name).To(Equal(deploymentName))

			op, err = scaledObjectClient.CreateOrUpdateAutoscaler(ctx, deployment,
				map[string]string{"created-by": "ottoscalr"}, *int32Ptr(8), *int32Ptr(5), *int32Ptr(10))
			Expect(err).ToNot(HaveOccurred())
			time.Sleep(2 * time.Second)
			Expect(op).To(Equal("updated"))
			scaledObject = &kedaapi.ScaledObject{}
			err = k8sClient.Get(ctx, types.NamespacedName{Namespace: deploymentNamespace, Name: deploymentName}, scaledObject)
			Expect(err).ToNot(HaveOccurred())
			Expect(scaledObject.Spec.MaxReplicaCount).To(Equal(int32Ptr(8)))
			Expect(scaledObject.Spec.MinReplicaCount).To(Equal(int32Ptr(5)))
			Expect(scaledObject.Spec.Triggers[0].Type).To(Equal("cpu"))
			Expect(scaledObject.Spec.Triggers[0].Metadata["type"]).To(Equal("Utilization"))
			Expect(scaledObject.Spec.Triggers[0].Metadata["value"]).To(Equal("10"))
			Expect(scaledObject.Spec.ScaleTargetRef.Name).To(Equal(deploymentName))
			Expect(k8sClient.Delete(ctx, scaledObject)).To(Succeed())

		})
	})
})

func int32Ptr(i int32) *int32 {
	return &i
}
