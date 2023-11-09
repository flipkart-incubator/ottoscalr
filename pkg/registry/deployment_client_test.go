package registry

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("DeploymentClient", func() {

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
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test-app",
					},
				},
				Replicas: int32Ptr(3),

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

	})

	AfterEach(func() {
		err := k8sClient.Delete(ctx, deployment)
		Expect(err).ToNot(HaveOccurred())
		err = k8sClient.Delete(ctx, deploymentPod)
		Expect(err).ToNot(HaveOccurred())

	})

	Describe("GetContainerResourceLimits", func() {

		It("should return the correct sum of CPU limits for a Deployment", func() {
			actualSum, err := deploymentClient.GetContainerResourceLimits(deploymentNamespace, deploymentName)
			Expect(err).To(BeNil())
			Expect(actualSum).To(Equal(float64(1.5)))
		})

		It("should return an error if the object is not found", func() {
			_, err := deploymentClient.GetContainerResourceLimits(deploymentNamespace, "non-existent-deployment")
			Expect(err).NotTo(BeNil())
		})

	})

	Describe("GetReplicaCount", func() {
		Context("when the deployment exists", func() {
			It("returns the deployment replica count", func() {

				// Call GetReplicaCount with the deployment namespace and name
				replicaCount, err := deploymentClient.GetReplicaCount("default", "test-deployment")
				Expect(err).NotTo(HaveOccurred())
				Expect(replicaCount).To(Equal(3))
			})
		})

	})

	Describe("Scale", func() {
		Context("when the deployment exists", func() {
			It("scales the deployment to the specified number of replicas", func() {

				// Call Scale with the deployment namespace, name, and a replica count of 5
				Expect(deploymentClient.Scale("default", "test-deployment", 5)).To(Succeed())
				time.Sleep(2 * time.Second)

				// Get the updated Deployment object
				updatedDeployment := &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: "test-deployment"}, updatedDeployment)).To(Succeed())

				// Verify that the Deployment object has a replica count of 5
				Expect(*updatedDeployment.Spec.Replicas).To(Equal(int32(5)))
			})
		})
	})
	Describe("GetKind", func() {
		It("returns the kind of the deployment client", func() {
			kind := deploymentClient.GetKind()
			Expect(kind).To(Equal("Deployment"))
		})
	})
	Describe("GetObjectType", func() {
		It("returns the type of the deployment client", func() {
			objectType := deploymentClient.GetObjectType()
			Expect(objectType).To(Equal(&appsv1.Deployment{}))
		})
	})
	Describe("GetMaxReplicaFromAnnotation", func() {
		Context("when the deployment does not have a max replica annotation", func() {
			It("returns the error", func() {
				maxReplica, err := deploymentClient.GetMaxReplicaFromAnnotation("default", "test-deployment")
				Expect(maxReplica).To(Equal(0))
				Expect(err).To(HaveOccurred())
			})
		})
		Context("when the deployment has a valid max replica annotation", func() {
			It("returns the max replica value", func() {
				deployment = &appsv1.Deployment{}
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: "test-deployment"}, deployment)
				Expect(err).ToNot(HaveOccurred())
				deployment.Annotations = map[string]string{
					"ottoscalr.io/max-pods": "10",
				}
				err = k8sClient.Update(ctx, deployment)
				time.Sleep(2 * time.Second)
				Expect(err).ToNot(HaveOccurred())
				maxReplica, err := deploymentClient.GetMaxReplicaFromAnnotation("default", "test-deployment")
				Expect(err).ToNot(HaveOccurred())
				Expect(maxReplica).To(Equal(10))
			})
		})
	})
	Describe("GetObject", func() {
		Context("when the deployment exists", func() {
			It("returns the deployment object", func() {

				// Call GetObject with the deployment namespace and name
				object, err := deploymentClient.GetObject("default", "test-deployment")
				Expect(err).NotTo(HaveOccurred())
				Expect(object).To(Equal(deployment))
			})
		})
	})

})

func int32Ptr(i int32) *int32 {
	return &i
}
