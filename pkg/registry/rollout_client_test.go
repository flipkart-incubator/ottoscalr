package registry

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	rolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("rolloutClient", func() {

	var (
		rolloutNamespace = "default"
		rolloutName      = "test-rollout"
		rollout          *rolloutv1alpha1.Rollout
		rolloutPod       *corev1.Pod
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
				Replicas: int32Ptr(3),
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
		err = k8sClient.Delete(ctx, rolloutPod)
		Expect(err).ToNot(HaveOccurred())

	})

	Describe("GetContainerResourceLimits", func() {

		It("should return the correct sum of CPU limits for a rollout", func() {
			actualSum, err := rolloutClient.GetContainerResourceLimits(rolloutNamespace, rolloutName)
			Expect(err).To(BeNil())
			Expect(actualSum).To(Equal(float64(1.2)))
		})

		It("should return an error if the object is not found", func() {
			_, err := rolloutClient.GetContainerResourceLimits(rolloutNamespace, "non-existent-rollout")
			Expect(err).NotTo(BeNil())
		})

	})

	Describe("GetReplicaCount", func() {
		Context("when the rollout exists", func() {
			It("returns the rollout replica count", func() {

				// Call GetReplicaCount with the rollout namespace and name
				replicaCount, err := rolloutClient.GetReplicaCount("default", "test-rollout")
				Expect(err).NotTo(HaveOccurred())
				Expect(replicaCount).To(Equal(3))
			})
		})

	})

	Describe("Scale", func() {
		Context("when the rollout exists", func() {
			It("scales the rollout to the specified number of replicas", func() {

				// Call Scale with the rollout namespace, name, and a replica count of 5
				Expect(rolloutClient.Scale("default", "test-rollout", 5)).To(Succeed())
				time.Sleep(2 * time.Second)

				// Get the updated rollout object
				updatedrollout := &rolloutv1alpha1.Rollout{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: "test-rollout"}, updatedrollout)).To(Succeed())

				// Verify that the rollout object has a replica count of 5
				Expect(*updatedrollout.Spec.Replicas).To(Equal(int32(5)))
			})
		})
	})
	Describe("GetKind", func() {
		It("returns the kind of the rollout client", func() {
			kind := rolloutClient.GetKind()
			Expect(kind).To(Equal("Rollout"))
		})
	})
	Describe("GetObjectType", func() {
		It("returns the type of the rollout client", func() {
			objectType := rolloutClient.GetObjectType()
			Expect(objectType).To(Equal(&rolloutv1alpha1.Rollout{}))
		})
	})
	Describe("GetMaxReplicaFromAnnotation", func() {
		Context("when the rollout does not have a max replica annotation", func() {
			It("returns the error", func() {
				maxReplica, err := rolloutClient.GetMaxReplicaFromAnnotation("default", "test-rollout")
				Expect(maxReplica).To(Equal(0))
				Expect(err).To(HaveOccurred())
			})
		})
		Context("when the rollout has a valid max replica annotation", func() {
			It("returns the max replica value", func() {
				rollout = &rolloutv1alpha1.Rollout{}
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: "test-rollout"}, rollout)
				Expect(err).ToNot(HaveOccurred())
				rollout.Annotations = map[string]string{
					"ottoscalr.io/max-pods": "10",
				}
				err = k8sClient.Update(ctx, rollout)
				time.Sleep(2 * time.Second)
				Expect(err).ToNot(HaveOccurred())
				maxReplica, err := rolloutClient.GetMaxReplicaFromAnnotation("default", "test-rollout")
				Expect(err).ToNot(HaveOccurred())
				Expect(maxReplica).To(Equal(10))
			})
		})
	})
	Describe("GetObject", func() {
		Context("when the rollout exists", func() {
			It("returns the rollout object", func() {

				// Call GetObject with the rollout namespace and name
				object, err := rolloutClient.GetObject("default", "test-rollout")
				Expect(err).NotTo(HaveOccurred())
				Expect(object).To(Equal(rollout))
			})
		})
	})

})
