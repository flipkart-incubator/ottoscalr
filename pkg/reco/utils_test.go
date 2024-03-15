package reco

import (
	"context"

	rolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var _ = Describe("UtilFunctions", func() {
	Context("GetMaxScaleUpPods with Rollout and Deployment having maxScaleUpPodsAnnotation", func() {

		var rollout *rolloutv1alpha1.Rollout
		var deployment *appsv1.Deployment
		_ = metav1.Now()
		ctx := context.TODO()
		BeforeEach(func() {
			rollout = &rolloutv1alpha1.Rollout{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-rollout",
					Namespace:   "default",
					Annotations: map[string]string{OttoscalrMaxScaleUpPodsAnnotation: "50"},
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
			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-deployment",
					Namespace:   "default",
					Annotations: map[string]string{OttoscalrMaxScaleUpPodsAnnotation: "40"},
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test-app",
						},
					},
					Strategy: appsv1.DeploymentStrategy{
						RollingUpdate: &appsv1.RollingUpdateDeployment{
							MaxSurge: &intstr.IntOrString{
								Type:   intstr.Int,
								IntVal: 10,
							},
							MaxUnavailable: &intstr.IntOrString{
								Type:   intstr.Int,
								IntVal: 12,
							},
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
					}},
			}
			Expect(k8sClient.Create(ctx, rollout)).Should(Succeed())
			Expect(k8sClient.Create(ctx, deployment)).Should(Succeed())

		})
		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, rollout)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, deployment)).Should(Succeed())
		})
		It("Should return correct maxScaleUpPods ", func() {
			maxScaleUpPods, err := GetMaxScaleUpPods(k8sClient, "default", "Rollout", "test-rollout", 100)
			Expect(err).To(BeNil())
			Expect(maxScaleUpPods).To(Equal(50))
			//Should be default value 15 as maxSurge is 10 and maxUnavailable is 12
			maxScaleUpPods, err = GetMaxScaleUpPods(k8sClient, "default", "Deployment", "test-deployment", 100)
			Expect(err).To(BeNil())
			Expect(maxScaleUpPods).To(Equal(40))
		})
	})
	Context("GetMaxScaleUpPods with Rollout and Deployment having MaxSurge and MaxUnavailable as int", func() {

		var rollout *rolloutv1alpha1.Rollout
		var deployment *appsv1.Deployment
		_ = metav1.Now()
		ctx := context.TODO()
		BeforeEach(func() {
			rollout = &rolloutv1alpha1.Rollout{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rollout",
					Namespace: "default",
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
					Strategy: rolloutv1alpha1.RolloutStrategy{
						Canary: &rolloutv1alpha1.CanaryStrategy{
							MaxSurge: &intstr.IntOrString{
								Type:   intstr.Int,
								IntVal: 25,
							},
							MaxUnavailable: &intstr.IntOrString{
								Type:   intstr.Int,
								IntVal: 30,
							},
						},
					},
				},
			}
			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test-app",
						},
					},
					Strategy: appsv1.DeploymentStrategy{
						RollingUpdate: &appsv1.RollingUpdateDeployment{
							MaxSurge: &intstr.IntOrString{
								Type:   intstr.Int,
								IntVal: 10,
							},
							MaxUnavailable: &intstr.IntOrString{
								Type:   intstr.Int,
								IntVal: 12,
							},
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
					}},
			}
			Expect(k8sClient.Create(ctx, rollout)).Should(Succeed())
			Expect(k8sClient.Create(ctx, deployment)).Should(Succeed())

		})
		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, rollout)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, deployment)).Should(Succeed())
		})
		It("Should return correct maxScaleUpPods ", func() {
			maxScaleUpPods, err := GetMaxScaleUpPods(k8sClient, "default", "Rollout", "test-rollout", 100)
			Expect(err).To(BeNil())
			Expect(maxScaleUpPods).To(Equal(30))
			//Should be default value 15 as maxSurge is 10 and maxUnavailable is 12
			maxScaleUpPods, err = GetMaxScaleUpPods(k8sClient, "default", "Deployment", "test-deployment", 100)
			Expect(err).To(BeNil())
			Expect(maxScaleUpPods).To(Equal(15))
		})

	})
	Context("GetMaxScaleUpPods with Rollout and Deployment having MaxSurge and MaxUnavailable as percentage", func() {

		var rollout *rolloutv1alpha1.Rollout
		var deployment *appsv1.Deployment
		_ = metav1.Now()
		ctx := context.TODO()
		BeforeEach(func() {
			rollout = &rolloutv1alpha1.Rollout{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rollout",
					Namespace: "default",
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
					Strategy: rolloutv1alpha1.RolloutStrategy{
						Canary: &rolloutv1alpha1.CanaryStrategy{
							MaxSurge: &intstr.IntOrString{
								Type:   intstr.String,
								StrVal: "30%",
							},
							MaxUnavailable: &intstr.IntOrString{
								Type:   intstr.Int,
								StrVal: "25%",
							},
						},
					},
				},
			}
			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test-app",
						},
					},
					Strategy: appsv1.DeploymentStrategy{
						RollingUpdate: &appsv1.RollingUpdateDeployment{
							MaxSurge: &intstr.IntOrString{
								Type:   intstr.String,
								StrVal: "18%",
							},
							MaxUnavailable: &intstr.IntOrString{
								Type:   intstr.String,
								StrVal: "20%",
							},
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
					}},
			}
			Expect(k8sClient.Create(ctx, rollout)).Should(Succeed())
			Expect(k8sClient.Create(ctx, deployment)).Should(Succeed())

		})
		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, rollout)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, deployment)).Should(Succeed())
		})
		It("Should return correct maxScaleUpPods ", func() {
			maxScaleUpPods, err := GetMaxScaleUpPods(k8sClient, "default", "Rollout", "test-rollout", 200)
			Expect(err).To(BeNil())
			Expect(maxScaleUpPods).To(Equal(60))
			maxScaleUpPods, err = GetMaxScaleUpPods(k8sClient, "default", "Deployment", "test-deployment", 200)
			Expect(err).To(BeNil())
			Expect(maxScaleUpPods).To(Equal(40))
		})

	})
	Context("GetMaxScaleUpPods with Rollout and Deployment not having MaxSurge or MaxUnavailable ", func() {

		var rollout *rolloutv1alpha1.Rollout
		var deployment *appsv1.Deployment
		_ = metav1.Now()
		ctx := context.TODO()
		BeforeEach(func() {
			rollout = &rolloutv1alpha1.Rollout{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rollout",
					Namespace: "default",
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
			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
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
					}},
			}
			Expect(k8sClient.Create(ctx, rollout)).Should(Succeed())
			Expect(k8sClient.Create(ctx, deployment)).Should(Succeed())

		})
		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, rollout)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, deployment)).Should(Succeed())
		})
		It("Should return correct maxScaleUpPods ", func() {
			//No strategy is added, so it will give default value of 15% of maxPods
			maxScaleUpPods, err := GetMaxScaleUpPods(k8sClient, "default", "Rollout", "test-rollout", 100)
			Expect(err).To(BeNil())
			Expect(maxScaleUpPods).To(Equal(15))

			//RollingUpdate is added with MaxSurge and MaxUnavailable as 25%
			maxScaleUpPods, err = GetMaxScaleUpPods(k8sClient, "default", "Deployment", "test-deployment", 100)
			Expect(err).To(BeNil())
			Expect(maxScaleUpPods).To(Equal(25))
		})

	})

	Context("EnableScaleUpBehaviourForHPA with Rollout and Deployment having enable annotation ", func() {

		var rollout *rolloutv1alpha1.Rollout
		var deployment *appsv1.Deployment
		_ = metav1.Now()
		ctx := context.TODO()
		BeforeEach(func() {
			rollout = &rolloutv1alpha1.Rollout{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-rollout",
					Namespace:   "default",
					Annotations: map[string]string{OttoscalrEnableScaleUpBehaviourAnnotation: "true"},
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
			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-deployment",
					Namespace:   "default",
					Annotations: map[string]string{OttoscalrEnableScaleUpBehaviourAnnotation: "true"},
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
					}},
			}
			Expect(k8sClient.Create(ctx, rollout)).Should(Succeed())
			Expect(k8sClient.Create(ctx, deployment)).Should(Succeed())

		})
		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, rollout)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, deployment)).Should(Succeed())
		})
		It("Should return expected behaviour", func() {
			enableScaleUpBehaviour := EnableScaleUpBehaviourForHPA(k8sClient, "default", "Rollout", "test-rollout")
			Expect(enableScaleUpBehaviour).To(Equal(true))
			enableScaleUpBehaviour = EnableScaleUpBehaviourForHPA(k8sClient, "default", "Deployment", "test-deployment")
			Expect(enableScaleUpBehaviour).To(Equal(true))
		})

	})

	Context("EnableScaleUpBehaviourForHPA with Rollout and Deployment having no enable annotation or not set to true", func() {

		var rollout *rolloutv1alpha1.Rollout
		var deployment *appsv1.Deployment
		_ = metav1.Now()
		ctx := context.TODO()
		BeforeEach(func() {
			rollout = &rolloutv1alpha1.Rollout{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-rollout",
					Namespace:   "default",
					Annotations: map[string]string{OttoscalrEnableScaleUpBehaviourAnnotation: "false"},
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
			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
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
					}},
			}
			Expect(k8sClient.Create(ctx, rollout)).Should(Succeed())
			Expect(k8sClient.Create(ctx, deployment)).Should(Succeed())

		})
		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, rollout)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, deployment)).Should(Succeed())
		})
		It("Should return expected behaviour", func() {
			enableScaleUpBehaviour := EnableScaleUpBehaviourForHPA(k8sClient, "default", "Rollout", "test-rollout")
			Expect(enableScaleUpBehaviour).To(Equal(false))
			enableScaleUpBehaviour = EnableScaleUpBehaviourForHPA(k8sClient, "default", "Deployment", "test-deployment")
			Expect(enableScaleUpBehaviour).To(Equal(false))
		})

	})
})
