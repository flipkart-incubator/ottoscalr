package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	rolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	kedaapi "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

//+kubebuilder:docs-gen:collapse=Imports

var _ = Describe("ScaledObject Manager controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		RolloutName         = "test-rollout1"
		RolloutNamespace    = "default"
		DeploymentName      = "test-deployment1"
		DeploymentNamespace = "default"

		timeout  = 10 * time.Second
		interval = time.Millisecond * 250
	)

	Context("When updating the max pod annotation for a Rollout", func() {
		var rollout *rolloutv1alpha1.Rollout
		var scaledObject *kedaapi.ScaledObject
		ctx := context.TODO()
		BeforeEach(func() {
			rollout = &rolloutv1alpha1.Rollout{
				ObjectMeta: metav1.ObjectMeta{
					Name:      RolloutName,
					Namespace: RolloutNamespace,
					Annotations: map[string]string{
						"ottoscalr.io/max-pods": "10",
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
			min := int32(1)
			max := int32(10)
			scaledObject = &kedaapi.ScaledObject{
				ObjectMeta: metav1.ObjectMeta{
					Name:      RolloutName,
					Namespace: RolloutNamespace,
					Labels: map[string]string{
						"created-by": "ottoscalr",
					},
				},
				Spec: kedaapi.ScaledObjectSpec{
					ScaleTargetRef: &kedaapi.ScaleTarget{
						Name:       RolloutName,
						APIVersion: "argoproj.io/v1alpha1",
						Kind:       "Rollout",
					},
					MinReplicaCount: &min,
					MaxReplicaCount: &max,
					Triggers: []kedaapi.ScaleTriggers{
						{
							Type: "cpu",
							Metadata: map[string]string{
								"type":  "Utilization",
								"value": "50",
							},
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, rollout)).Should(Succeed())
			Expect(k8sClient.Create(ctx, scaledObject)).Should(Succeed())

		})
		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, rollout)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, scaledObject)).Should(Succeed())
		})
		It("Should update the scaledobect's maxReplica Count", func() {
			By("By updating the max pod annotation for a Rollout")

			createdRollout := &rolloutv1alpha1.Rollout{}
			createdScaledObject := &kedaapi.ScaledObject{}

			time.Sleep(1 * time.Second)

			Eventually(func() bool {
				err := k8sClient.Get(ctx,
					types.NamespacedName{Name: RolloutName, Namespace: RolloutNamespace},
					createdRollout)
				if err != nil {
					return false
				}

				err = k8sClient.Get(ctx,
					types.NamespacedName{Name: RolloutName, Namespace: RolloutNamespace},
					createdScaledObject)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			Expect(createdRollout.Name).Should(Equal(RolloutName))
			max := int32(10)
			Expect(createdScaledObject.Spec.MaxReplicaCount).Should(Equal(&max))

			createdRollout.Annotations["ottoscalr.io/max-pods"] = "15"
			Expect(k8sClient.Update(ctx, createdRollout)).Should(Succeed())
			k8sClient.Get(ctx,
				types.NamespacedName{Name: RolloutName, Namespace: RolloutNamespace},
				createdRollout)
			testString, _ := json.MarshalIndent(createdRollout, "", "   ")
			fmt.Fprintf(GinkgoWriter, "Rollout updated: %s\n", testString)
			updatedScaledObject := &kedaapi.ScaledObject{}

			time.Sleep(1 * time.Second)

			k8sClient.Get(ctx,
				types.NamespacedName{Name: RolloutName, Namespace: RolloutNamespace},
				updatedScaledObject)

			max = int32(15)
			Expect(updatedScaledObject.Spec.MaxReplicaCount).Should(Equal(&max))

		})
	})

	Context("When updating the max pod annotation for a Deployment", func() {
		var deployment *appsv1.Deployment
		var scaledObject *kedaapi.ScaledObject
		ctx := context.TODO()
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

			min := int32(1)
			max := int32(10)
			scaledObject = &kedaapi.ScaledObject{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DeploymentName,
					Namespace: DeploymentNamespace,
					Labels: map[string]string{
						"created-by": "ottoscalr",
					},
				},
				Spec: kedaapi.ScaledObjectSpec{
					ScaleTargetRef: &kedaapi.ScaleTarget{
						Name:       DeploymentName,
						APIVersion: "apps/v1",
						Kind:       "Deployment",
					},
					MinReplicaCount: &min,
					MaxReplicaCount: &max,
					Triggers: []kedaapi.ScaleTriggers{
						{
							Type: "cpu",
							Metadata: map[string]string{
								"type":  "Utilization",
								"value": "50",
							},
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, deployment)).Should(Succeed())
			Expect(k8sClient.Create(ctx, scaledObject)).Should(Succeed())
		})
		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, deployment)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, scaledObject)).Should(Succeed())
		})
		It("Should update the scaledobect's maxReplica Count", func() {
			By("By updating the max pod annotation for a Deployment")

			createdDeployment := &appsv1.Deployment{}
			createdScaledObject := &kedaapi.ScaledObject{}

			time.Sleep(1 * time.Second)

			Eventually(func() bool {
				err := k8sClient.Get(ctx,
					types.NamespacedName{Name: DeploymentName, Namespace: DeploymentNamespace},
					createdDeployment)
				if err != nil {
					return false
				}

				err = k8sClient.Get(ctx,
					types.NamespacedName{Name: DeploymentName, Namespace: DeploymentNamespace},
					createdScaledObject)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			Expect(createdDeployment.Name).Should(Equal(DeploymentName))
			max := int32(10)
			Expect(createdScaledObject.Spec.MaxReplicaCount).Should(Equal(&max))

			createdDeployment.Annotations["ottoscalr.io/max-pods"] = "20"
			Expect(k8sClient.Update(ctx, createdDeployment)).Should(Succeed())
			k8sClient.Get(ctx,
				types.NamespacedName{Name: DeploymentName, Namespace: DeploymentNamespace},
				createdDeployment)
			testString, _ := json.MarshalIndent(createdDeployment, "", "   ")
			fmt.Fprintf(GinkgoWriter, "Deployment updated: %s\n", testString)
			updatedScaledObject := &kedaapi.ScaledObject{}

			time.Sleep(1 * time.Second)

			k8sClient.Get(ctx,
				types.NamespacedName{Name: DeploymentName, Namespace: DeploymentNamespace},
				updatedScaledObject)

			max = int32(20)
			Expect(updatedScaledObject.Spec.MaxReplicaCount).Should(Equal(&max))

		})
	})

	Context("When updating the max pod annotation for a Deployment and scaledObject is not created by ottoscalr", func() {
		var deployment *appsv1.Deployment
		var scaledObject *kedaapi.ScaledObject
		ctx := context.TODO()
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

			min := int32(1)
			max := int32(10)
			scaledObject = &kedaapi.ScaledObject{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DeploymentName,
					Namespace: DeploymentNamespace,
				},
				Spec: kedaapi.ScaledObjectSpec{
					ScaleTargetRef: &kedaapi.ScaleTarget{
						Name:       DeploymentName,
						APIVersion: "apps/v1",
						Kind:       "Deployment",
					},
					MinReplicaCount: &min,
					MaxReplicaCount: &max,
					Triggers: []kedaapi.ScaleTriggers{
						{
							Type: "cpu",
							Metadata: map[string]string{
								"type":  "Utilization",
								"value": "50",
							},
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, deployment)).Should(Succeed())
			Expect(k8sClient.Create(ctx, scaledObject)).Should(Succeed())
		})
		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, deployment)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, scaledObject)).Should(Succeed())
		})
		It("Should not update the scaledobect's maxReplica Count", func() {
			By("By updating the max pod annotation for a Deployment")

			createdDeployment := &appsv1.Deployment{}
			createdScaledObject := &kedaapi.ScaledObject{}

			time.Sleep(1 * time.Second)

			Eventually(func() bool {
				err := k8sClient.Get(ctx,
					types.NamespacedName{Name: DeploymentName, Namespace: DeploymentNamespace},
					createdDeployment)
				if err != nil {
					return false
				}

				err = k8sClient.Get(ctx,
					types.NamespacedName{Name: DeploymentName, Namespace: DeploymentNamespace},
					createdScaledObject)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			Expect(createdDeployment.Name).Should(Equal(DeploymentName))
			max := int32(10)
			Expect(createdScaledObject.Spec.MaxReplicaCount).Should(Equal(&max))

			createdDeployment.Annotations["ottoscalr.io/max-pods"] = "20"
			Expect(k8sClient.Update(ctx, createdDeployment)).Should(Succeed())
			k8sClient.Get(ctx,
				types.NamespacedName{Name: DeploymentName, Namespace: DeploymentNamespace},
				createdDeployment)
			testString, _ := json.MarshalIndent(createdDeployment, "", "   ")
			fmt.Fprintf(GinkgoWriter, "Deployment updated: %s\n", testString)
			updatedScaledObject := &kedaapi.ScaledObject{}

			time.Sleep(1 * time.Second)

			k8sClient.Get(ctx,
				types.NamespacedName{Name: DeploymentName, Namespace: DeploymentNamespace},
				updatedScaledObject)
			Expect(updatedScaledObject.Spec.MaxReplicaCount).Should(Equal(&max))

		})
	})

	Context("When scaledObject's maxReplica is updated and is not equal to the maxPod Annotation", func() {
		var rollout *rolloutv1alpha1.Rollout
		var scaledObject *kedaapi.ScaledObject
		ctx := context.TODO()
		BeforeEach(func() {
			rollout = &rolloutv1alpha1.Rollout{
				ObjectMeta: metav1.ObjectMeta{
					Name:      RolloutName,
					Namespace: RolloutNamespace,
					Annotations: map[string]string{
						"ottoscalr.io/max-pods": "10",
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
			min := int32(1)
			max := int32(10)
			scaledObject = &kedaapi.ScaledObject{
				ObjectMeta: metav1.ObjectMeta{
					Name:      RolloutName,
					Namespace: RolloutNamespace,
					Labels: map[string]string{
						"created-by": "ottoscalr",
					},
				},
				Spec: kedaapi.ScaledObjectSpec{
					ScaleTargetRef: &kedaapi.ScaleTarget{
						Name:       RolloutName,
						APIVersion: "argoproj.io/v1alpha1",
						Kind:       "Rollout",
					},
					MinReplicaCount: &min,
					MaxReplicaCount: &max,
					Triggers: []kedaapi.ScaleTriggers{
						{
							Type: "cpu",
							Metadata: map[string]string{
								"type":  "Utilization",
								"value": "50",
							},
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, rollout)).Should(Succeed())
			Expect(k8sClient.Create(ctx, scaledObject)).Should(Succeed())

		})
		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, rollout)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, scaledObject)).Should(Succeed())
		})
		It("Should update the scaledobect's maxReplica Count", func() {
			By("By updating the max pod annotation for a Rollout")

			createdRollout := &rolloutv1alpha1.Rollout{}
			createdScaledObject := &kedaapi.ScaledObject{}

			time.Sleep(1 * time.Second)

			Eventually(func() bool {
				err := k8sClient.Get(ctx,
					types.NamespacedName{Name: RolloutName, Namespace: RolloutNamespace},
					createdRollout)
				if err != nil {
					return false
				}

				err = k8sClient.Get(ctx,
					types.NamespacedName{Name: RolloutName, Namespace: RolloutNamespace},
					createdScaledObject)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			Expect(createdRollout.Name).Should(Equal(RolloutName))
			max := int32(10)
			Expect(createdScaledObject.Spec.MaxReplicaCount).Should(Equal(&max))

			max = int32(9)
			createdScaledObject.Spec.MaxReplicaCount = &max
			Expect(k8sClient.Update(ctx, createdScaledObject)).Should(Succeed())
			k8sClient.Get(ctx,
				types.NamespacedName{Name: RolloutName, Namespace: RolloutNamespace},
				createdScaledObject)

			updatedScaledObject := &kedaapi.ScaledObject{}

			time.Sleep(1 * time.Second)

			k8sClient.Get(ctx,
				types.NamespacedName{Name: RolloutName, Namespace: RolloutNamespace},
				updatedScaledObject)

			max = int32(10)
			Expect(updatedScaledObject.Spec.MaxReplicaCount).Should(Equal(&max))

		})
	})

})
