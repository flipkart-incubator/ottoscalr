package controller

import (
	"context"
	"encoding/json"
	"fmt"
	v1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

var _ = Describe("PolicyrecommendationController", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		PolicyRecoName      = "test-deployment-cfldw"
		PolicyRecoNamespace = "default"

		timeout             = time.Second * 10
		interval            = time.Millisecond * 250
		policyAge           = 1 * time.Second
		DeploymentName      = "test-deployment"
		DeploymentNamespace = "default"
	)

	BeforeEach(func() {
		// Defaults for the entire test
		recommender.Min = 10
		recommender.Threshold = 60
		recommender.Max = 60
	})

	When("A fresh PolicyRecommendation is created", func() {
		policyreco := &v1alpha1.PolicyRecommendation{}
		createdPolicy := &v1alpha1.PolicyRecommendation{}
		updatedPolicy := &v1alpha1.PolicyRecommendation{}

		var safestPolicy, policy1, policy2, policy3 /*, policy4*/ v1alpha1.Policy
		BeforeEach(func() {
			safestPolicy = v1alpha1.Policy{
				ObjectMeta: metav1.ObjectMeta{Name: "safest-policy"},
				Spec: v1alpha1.PolicySpec{
					IsDefault:               false,
					RiskIndex:               1,
					MinReplicaPercentageCut: 80,
					TargetUtilization:       10,
				},
			}
			policy1 = v1alpha1.Policy{
				ObjectMeta: metav1.ObjectMeta{Name: "policy-1"},
				Spec: v1alpha1.PolicySpec{
					IsDefault:               false,
					RiskIndex:               10,
					MinReplicaPercentageCut: 100,
					TargetUtilization:       15,
				},
			}
			policy2 = v1alpha1.Policy{
				ObjectMeta: metav1.ObjectMeta{Name: "policy-2"},
				Spec: v1alpha1.PolicySpec{
					IsDefault:               true,
					RiskIndex:               20,
					MinReplicaPercentageCut: 100,
					TargetUtilization:       20,
				},
			}
			policy3 = v1alpha1.Policy{
				ObjectMeta: metav1.ObjectMeta{Name: "policy-3"},
				Spec: v1alpha1.PolicySpec{
					IsDefault:               false,
					RiskIndex:               30,
					MinReplicaPercentageCut: 100,
					TargetUtilization:       30,
				},
			}
			Expect(k8sClient.Create(ctx, &safestPolicy)).Should(Succeed())
			Expect(k8sClient.Create(ctx, &policy1)).Should(Succeed())
			Expect(k8sClient.Create(ctx, &policy2)).Should(Succeed())
			Expect(k8sClient.Create(ctx, &policy3)).Should(Succeed())

			policyreco = &v1alpha1.PolicyRecommendation{}
			createdPolicy = &v1alpha1.PolicyRecommendation{}
			updatedPolicy = &v1alpha1.PolicyRecommendation{}
		})
		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, createdPolicy)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, &safestPolicy)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, &policy1)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, &policy2)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, &policy3)).Should(Succeed())
		})

		It("Lifecycle of a PolicyRecommendation with a default policy", func() {
			ctx := context.Background()
			now := metav1.Now()
			policyreco = &v1alpha1.PolicyRecommendation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PolicyRecoName,
					Namespace: PolicyRecoNamespace,
				},
				Spec: v1alpha1.PolicyRecommendationSpec{
					WorkloadMeta: v1alpha1.WorkloadMeta{
						Name:      PolicyRecoName,
						Namespace: PolicyRecoNamespace,
					},
					Policy:             safestPolicy.Name,
					GeneratedAt:        &now,
					TransitionedAt:     &now,
					QueuedForExecution: &falseBool,
				},
			}
			Expect(k8sClient.Create(ctx, policyreco)).Should(Succeed())
			Expect(k8sClient.Get(ctx,
				types.NamespacedName{Name: PolicyRecoName, Namespace: PolicyRecoNamespace},
				createdPolicy),
			).Should(Succeed())

			createdPolicyString, _ := json.MarshalIndent(createdPolicy, "", "   ")
			fmt.Fprintf(GinkgoWriter, "Created policy : %s", createdPolicyString)
			Expect(createdPolicy.Name).Should(Equal(PolicyRecoName))
			Expect(createdPolicy.Namespace).Should(Equal(PolicyRecoNamespace))
			Expect(createdPolicy.Spec.Policy).Should(Equal(safestPolicy.Name))
			Expect(*createdPolicy.Spec.QueuedForExecution).Should(BeFalse())

			By("Aging the policy once")
			time.Sleep(2 * policyAge)
			//Expect(queueReco(PolicyRecoName, PolicyRecoNamespace)).Should(Succeed())
			Expect(queueRecoByUpdateOp(PolicyRecoName, PolicyRecoNamespace)).Should(Succeed())

			Eventually(func() bool {
				fmt.Println("In eventually decorator ...")
				err := k8sClient.Get(ctx,
					types.NamespacedName{Name: PolicyRecoName, Namespace: PolicyRecoNamespace},
					updatedPolicy)
				if err != nil {
					return false
				}
				updatedPolicyString, _ := json.MarshalIndent(updatedPolicy, "", "   ")
				fmt.Fprintf(GinkgoWriter, "Updated policy: %s", updatedPolicyString)
				return *updatedPolicy.Spec.QueuedForExecution
			}, timeout, 8*interval).Should(BeFalse())
			Expect(updatedPolicy.Name).Should(Equal(PolicyRecoName))
			Expect(updatedPolicy.Spec.Policy).Should(Equal(policy1.Name))
			Expect(updatedPolicy.Namespace).Should(Equal(PolicyRecoNamespace))
			Expect(*updatedPolicy.Spec.QueuedForExecution).Should(BeFalse())
			Expect(updatedPolicy.Spec.GeneratedAt).Should(Equal(updatedPolicy.Spec.TransitionedAt))

			By("Aging the policy again")
			time.Sleep(2 * policyAge)
			updatedPolicy = &v1alpha1.PolicyRecommendation{}
			//Expect(queueReco(PolicyRecoName, PolicyRecoNamespace)).Should(Succeed())
			Expect(queueRecoByUpdateOp(PolicyRecoName, PolicyRecoNamespace)).Should(Succeed())

			Eventually(func() bool {
				fmt.Println("In eventually decorator ...")
				err := k8sClient.Get(ctx,
					types.NamespacedName{Name: PolicyRecoName, Namespace: PolicyRecoNamespace},
					updatedPolicy)
				if err != nil {
					return false
				}
				updatedPolicyString, _ := json.MarshalIndent(updatedPolicy, "", "   ")
				fmt.Fprintf(GinkgoWriter, "Updated policy: %s", updatedPolicyString)
				return *updatedPolicy.Spec.QueuedForExecution
			}, timeout, 8*interval).Should(BeFalse())
			Expect(updatedPolicy.Name).Should(Equal(PolicyRecoName))
			Expect(updatedPolicy.Spec.Policy).Should(Equal(policy2.Name))
			Expect(updatedPolicy.Namespace).Should(Equal(PolicyRecoNamespace))
			Expect(*updatedPolicy.Spec.QueuedForExecution).Should(BeFalse())
			Expect(updatedPolicy.Spec.GeneratedAt).Should(Equal(updatedPolicy.Spec.TransitionedAt))

			By("Aging the policy again")
			By("Policy should be capped to default")
			time.Sleep(2 * policyAge)
			updatedPolicy = &v1alpha1.PolicyRecommendation{}
			//Expect(queueReco(PolicyRecoName, PolicyRecoNamespace)).Should(Succeed())
			Expect(queueRecoByUpdateOp(PolicyRecoName, PolicyRecoNamespace)).Should(Succeed())

			Eventually(func() bool {
				fmt.Println("In eventually decorator ...")
				err := k8sClient.Get(ctx,
					types.NamespacedName{Name: PolicyRecoName, Namespace: PolicyRecoNamespace},
					updatedPolicy)
				if err != nil {
					return false
				}
				updatedPolicyString, _ := json.MarshalIndent(updatedPolicy, "", "   ")
				fmt.Fprintf(GinkgoWriter, "Updated policy: %s", updatedPolicyString)
				return *updatedPolicy.Spec.QueuedForExecution
			}, timeout, 8*interval).Should(BeFalse())
			Expect(updatedPolicy.Name).Should(Equal(PolicyRecoName))
			Expect(updatedPolicy.Spec.Policy).Should(Equal(policy2.Name))
			Expect(updatedPolicy.Namespace).Should(Equal(PolicyRecoNamespace))
			Expect(*updatedPolicy.Spec.QueuedForExecution).Should(BeFalse())
			Expect(updatedPolicy.Spec.GeneratedAt.Time).Should(BeTemporally(">", updatedPolicy.Spec.TransitionedAt.Time))

		})

	})

	When("A fresh PolicyRecommendation is created", func() {
		policyreco := &v1alpha1.PolicyRecommendation{}
		createdPolicy := &v1alpha1.PolicyRecommendation{}
		updatedPolicy := &v1alpha1.PolicyRecommendation{}

		var targetMin, targetMax, targetThreshold int
		var safestPolicy, policy1, policy2 v1alpha1.Policy
		BeforeEach(func() {
			targetMin = 10
			targetMax = 60
			targetThreshold = 10
			recommender.Min = targetMin
			recommender.Threshold = targetThreshold
			recommender.Max = targetMax
			safestPolicy = v1alpha1.Policy{
				ObjectMeta: metav1.ObjectMeta{Name: "safest-policy"},
				Spec: v1alpha1.PolicySpec{
					IsDefault:               false,
					RiskIndex:               1,
					MinReplicaPercentageCut: 80,
					TargetUtilization:       10,
				},
			}
			policy1 = v1alpha1.Policy{
				ObjectMeta: metav1.ObjectMeta{Name: "policy-1"},
				Spec: v1alpha1.PolicySpec{
					IsDefault:               false,
					RiskIndex:               10,
					MinReplicaPercentageCut: 100,
					TargetUtilization:       15,
				},
			}
			policy2 = v1alpha1.Policy{
				ObjectMeta: metav1.ObjectMeta{Name: "policy-2"},
				Spec: v1alpha1.PolicySpec{
					IsDefault:               true,
					RiskIndex:               20,
					MinReplicaPercentageCut: 100,
					TargetUtilization:       20,
				},
			}
			Expect(k8sClient.Create(ctx, &safestPolicy)).Should(Succeed())
			Expect(k8sClient.Create(ctx, &policy1)).Should(Succeed())
			Expect(k8sClient.Create(ctx, &policy2)).Should(Succeed())

			policyreco = &v1alpha1.PolicyRecommendation{}
			createdPolicy = &v1alpha1.PolicyRecommendation{}
			updatedPolicy = &v1alpha1.PolicyRecommendation{}
		})
		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, createdPolicy)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, &safestPolicy)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, &policy1)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, &policy2)).Should(Succeed())
		})

		It("With a conservative target reco", func() {
			ctx := context.Background()
			now := metav1.Now()
			policyreco = &v1alpha1.PolicyRecommendation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PolicyRecoName,
					Namespace: PolicyRecoNamespace,
				},
				Spec: v1alpha1.PolicyRecommendationSpec{
					WorkloadMeta: v1alpha1.WorkloadMeta{
						Name:      PolicyRecoName,
						Namespace: PolicyRecoNamespace,
					},
					Policy:             safestPolicy.Name,
					GeneratedAt:        &now,
					TransitionedAt:     &now,
					QueuedForExecution: &trueBool,
				},
			}
			Expect(k8sClient.Create(ctx, policyreco)).Should(Succeed())
			Eventually(func() bool {
				fmt.Println("In eventually decorator ...")
				err := k8sClient.Get(ctx,
					types.NamespacedName{Name: PolicyRecoName, Namespace: PolicyRecoNamespace},
					createdPolicy)
				if err != nil {
					return false
				}
				return *createdPolicy.Spec.QueuedForExecution
			}, timeout, 8*interval).Should(BeFalse())

			createdPolicyString, _ := json.MarshalIndent(createdPolicy, "", "   ")
			fmt.Fprintf(GinkgoWriter, "Created policy : %s", createdPolicyString)
			Expect(createdPolicy.Name).Should(Equal(PolicyRecoName))
			Expect(createdPolicy.Namespace).Should(Equal(PolicyRecoNamespace))
			Expect(createdPolicy.Spec.Policy).Should(Equal(safestPolicy.Name))
			Expect(*createdPolicy.Spec.QueuedForExecution).Should(BeFalse())
			Expect(createdPolicy.Spec.TargetHPAConfiguration.Max).Should(Equal(targetMax))
			Expect(createdPolicy.Spec.TargetHPAConfiguration.Min).Should(Equal(targetMin))
			Expect(createdPolicy.Spec.TargetHPAConfiguration.TargetMetricValue).Should(Equal(targetThreshold))
			Expect(createdPolicy.Spec.CurrentHPAConfiguration.Min).Should(Equal(targetMax - ((targetMax - targetMin) * (safestPolicy.Spec.MinReplicaPercentageCut) / 100)))
			Expect(createdPolicy.Spec.CurrentHPAConfiguration.Max).Should(Equal(targetMax))
			Expect(createdPolicy.Spec.CurrentHPAConfiguration.TargetMetricValue).Should(Equal(safestPolicy.Spec.TargetUtilization))

			By("Aging the policy once")
			time.Sleep(2 * policyAge)
			Expect(queueRecoByUpdateOp(PolicyRecoName, PolicyRecoNamespace)).Should(Succeed())

			Eventually(func() bool {
				fmt.Println("In eventually decorator ...")
				err := k8sClient.Get(ctx,
					types.NamespacedName{Name: PolicyRecoName, Namespace: PolicyRecoNamespace},
					updatedPolicy)
				if err != nil {
					return false
				}
				updatedPolicyString, _ := json.MarshalIndent(updatedPolicy, "", "   ")
				fmt.Fprintf(GinkgoWriter, "Updated policy: %s", updatedPolicyString)
				return *updatedPolicy.Spec.QueuedForExecution
			}, timeout, 8*interval).Should(BeFalse())
			Expect(updatedPolicy.Name).Should(Equal(PolicyRecoName))
			Expect(updatedPolicy.Spec.Policy).Should(Equal(safestPolicy.Name))
			Expect(updatedPolicy.Namespace).Should(Equal(PolicyRecoNamespace))
			Expect(*updatedPolicy.Spec.QueuedForExecution).Should(BeFalse())
			Expect(updatedPolicy.Spec.GeneratedAt).Should(Equal(updatedPolicy.Spec.TransitionedAt))
			Expect(updatedPolicy.Spec.TargetHPAConfiguration.Max).Should(Equal(targetMax))
			Expect(updatedPolicy.Spec.TargetHPAConfiguration.Min).Should(Equal(targetMin))
			Expect(updatedPolicy.Spec.TargetHPAConfiguration.TargetMetricValue).Should(Equal(targetThreshold))
			Expect(updatedPolicy.Spec.CurrentHPAConfiguration).Should(Equal(updatedPolicy.Spec.TargetHPAConfiguration))

			By("Aging the policy once again")
			time.Sleep(2 * policyAge)
			Expect(queueRecoByUpdateOp(PolicyRecoName, PolicyRecoNamespace)).Should(Succeed())

			Eventually(func() bool {
				fmt.Println("In eventually decorator ...")
				err := k8sClient.Get(ctx,
					types.NamespacedName{Name: PolicyRecoName, Namespace: PolicyRecoNamespace},
					updatedPolicy)
				if err != nil {
					return false
				}
				updatedPolicyString, _ := json.MarshalIndent(updatedPolicy, "", "   ")
				fmt.Fprintf(GinkgoWriter, "Updated policy: %s", updatedPolicyString)
				return *updatedPolicy.Spec.QueuedForExecution
			}, timeout, 8*interval).Should(BeFalse())
			Expect(updatedPolicy.Name).Should(Equal(PolicyRecoName))
			Expect(updatedPolicy.Spec.Policy).Should(Equal(safestPolicy.Name))
			Expect(updatedPolicy.Namespace).Should(Equal(PolicyRecoNamespace))
			Expect(*updatedPolicy.Spec.QueuedForExecution).Should(BeFalse())
			Expect(updatedPolicy.Spec.GeneratedAt.Time).Should(BeTemporally(">", updatedPolicy.Spec.TransitionedAt.Time))
			Expect(updatedPolicy.Spec.TargetHPAConfiguration.Max).Should(Equal(targetMax))
			Expect(updatedPolicy.Spec.TargetHPAConfiguration.Min).Should(Equal(targetMin))
			Expect(updatedPolicy.Spec.TargetHPAConfiguration.TargetMetricValue).Should(Equal(targetThreshold))
			Expect(updatedPolicy.Spec.CurrentHPAConfiguration).Should(Equal(updatedPolicy.Spec.TargetHPAConfiguration))
		})

		It("With an aggressive target reco", func() {
			targetMin = 10
			targetMax = 60
			targetThreshold = 80
			recommender.Min = targetMin
			recommender.Max = targetMax
			recommender.Threshold = targetThreshold
			ctx := context.Background()
			now := metav1.Now()
			policyreco = &v1alpha1.PolicyRecommendation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PolicyRecoName,
					Namespace: PolicyRecoNamespace,
				},
				Spec: v1alpha1.PolicyRecommendationSpec{
					WorkloadMeta: v1alpha1.WorkloadMeta{
						Name:      PolicyRecoName,
						Namespace: PolicyRecoNamespace,
					},
					Policy:             safestPolicy.Name,
					GeneratedAt:        &now,
					TransitionedAt:     &now,
					QueuedForExecution: &trueBool,
				},
				//Status: v1alpha1.PolicyRecommendationStatus{
				//	Conditions: []metav1.Condition{
				//		{
				//			Type:               string(v1alpha1.Initialized),
				//			Status:             metav1.ConditionTrue,
				//			LastTransitionTime: metav1.Now(),
				//			Reason:             PolicyRecommendationCRDCreated,
				//			Message:            InitializedMessage,
				//		},
				//	},
				//},
			}
			Expect(k8sClient.Create(ctx, policyreco)).Should(Succeed())
			Eventually(func() bool {
				fmt.Println("In eventually decorator ...")
				err := k8sClient.Get(ctx,
					types.NamespacedName{Name: PolicyRecoName, Namespace: PolicyRecoNamespace},
					createdPolicy)
				if err != nil {
					return false
				}
				return *createdPolicy.Spec.QueuedForExecution
			}, timeout, 8*interval).Should(BeFalse())

			fmt.Fprintf(GinkgoWriter, "Created Reco: %v", createdPolicy)

			createdPolicyString, _ := json.MarshalIndent(createdPolicy, "", "   ")
			fmt.Fprintf(GinkgoWriter, "Created policy : %s", createdPolicyString)
			Expect(createdPolicy.Name).Should(Equal(PolicyRecoName))
			Expect(createdPolicy.Namespace).Should(Equal(PolicyRecoNamespace))
			Expect(createdPolicy.Spec.Policy).Should(Equal(safestPolicy.Name))
			Expect(*createdPolicy.Spec.QueuedForExecution).Should(BeFalse())
			Expect(createdPolicy.Spec.TargetHPAConfiguration.Max).Should(Equal(targetMax))
			Expect(createdPolicy.Spec.TargetHPAConfiguration.Min).Should(Equal(targetMin))
			Expect(createdPolicy.Spec.TargetHPAConfiguration.TargetMetricValue).Should(Equal(targetThreshold))
			Expect(createdPolicy.Spec.CurrentHPAConfiguration.Min).Should(Equal(targetMax - ((targetMax - targetMin) * (safestPolicy.Spec.MinReplicaPercentageCut) / 100)))
			Expect(createdPolicy.Spec.CurrentHPAConfiguration.Max).Should(Equal(targetMax))
			Expect(createdPolicy.Spec.CurrentHPAConfiguration.TargetMetricValue).Should(Equal(safestPolicy.Spec.TargetUtilization))

			By("Aging the policy once")
			time.Sleep(2 * policyAge)
			Expect(queueRecoByUpdateOp(PolicyRecoName, PolicyRecoNamespace)).Should(Succeed())

			Eventually(func() bool {
				fmt.Println("In eventually decorator ...")
				err := k8sClient.Get(ctx,
					types.NamespacedName{Name: PolicyRecoName, Namespace: PolicyRecoNamespace},
					updatedPolicy)
				if err != nil {
					return false
				}
				updatedPolicyString, _ := json.MarshalIndent(updatedPolicy, "", "   ")
				fmt.Fprintf(GinkgoWriter, "Updated policy: %s", updatedPolicyString)
				return *updatedPolicy.Spec.QueuedForExecution
			}, timeout, 8*interval).Should(BeFalse())
			Expect(updatedPolicy.Name).Should(Equal(PolicyRecoName))
			Expect(updatedPolicy.Spec.Policy).Should(Equal(policy1.Name))
			Expect(updatedPolicy.Namespace).Should(Equal(PolicyRecoNamespace))
			Expect(updatedPolicy.Spec.GeneratedAt).Should(Equal(updatedPolicy.Spec.TransitionedAt))
			Expect(updatedPolicy.Spec.TargetHPAConfiguration.Max).Should(Equal(targetMax))
			Expect(updatedPolicy.Spec.TargetHPAConfiguration.Min).Should(Equal(targetMin))
			Expect(updatedPolicy.Spec.TargetHPAConfiguration.TargetMetricValue).Should(Equal(targetThreshold))
			Expect(updatedPolicy.Spec.CurrentHPAConfiguration.Min).Should(Equal(targetMax - ((targetMax - targetMin) * (policy1.Spec.MinReplicaPercentageCut) / 100)))
			Expect(updatedPolicy.Spec.CurrentHPAConfiguration.Max).Should(Equal(targetMax))
			Expect(updatedPolicy.Spec.CurrentHPAConfiguration.TargetMetricValue).Should(Equal(policy1.Spec.TargetUtilization))

			By("Aging the policy once")
			time.Sleep(2 * policyAge)
			updatedPolicy = &v1alpha1.PolicyRecommendation{}
			Expect(queueRecoByUpdateOp(PolicyRecoName, PolicyRecoNamespace)).Should(Succeed())

			Eventually(func() bool {
				fmt.Println("In eventually decorator ...")
				err := k8sClient.Get(ctx,
					types.NamespacedName{Name: PolicyRecoName, Namespace: PolicyRecoNamespace},
					updatedPolicy)
				if err != nil {
					return false
				}
				updatedPolicyString, _ := json.MarshalIndent(updatedPolicy, "", "   ")
				fmt.Fprintf(GinkgoWriter, "Updated policy: %s", updatedPolicyString)
				return *updatedPolicy.Spec.QueuedForExecution
			}, timeout, 8*interval).Should(BeFalse())
			Expect(updatedPolicy.Name).Should(Equal(PolicyRecoName))
			Expect(updatedPolicy.Spec.Policy).Should(Equal(policy2.Name))
			Expect(updatedPolicy.Namespace).Should(Equal(PolicyRecoNamespace))
			Expect(updatedPolicy.Spec.GeneratedAt).Should(Equal(updatedPolicy.Spec.TransitionedAt))
			Expect(updatedPolicy.Spec.TargetHPAConfiguration.Max).Should(Equal(targetMax))
			Expect(updatedPolicy.Spec.TargetHPAConfiguration.Min).Should(Equal(targetMin))
			Expect(updatedPolicy.Spec.TargetHPAConfiguration.TargetMetricValue).Should(Equal(targetThreshold))
			Expect(updatedPolicy.Spec.CurrentHPAConfiguration.Min).Should(Equal(targetMax - ((targetMax - targetMin) * (policy2.Spec.MinReplicaPercentageCut) / 100)))
			Expect(updatedPolicy.Spec.CurrentHPAConfiguration.Max).Should(Equal(targetMax))
			Expect(updatedPolicy.Spec.CurrentHPAConfiguration.TargetMetricValue).Should(Equal(policy2.Spec.TargetUtilization))

			By("Aging the policy once again and it should be capped to default policy")
			time.Sleep(2 * policyAge)
			updatedPolicy = &v1alpha1.PolicyRecommendation{}
			Expect(queueRecoByUpdateOp(PolicyRecoName, PolicyRecoNamespace)).Should(Succeed())

			Eventually(func() bool {
				fmt.Println("In eventually decorator ...")
				err := k8sClient.Get(ctx,
					types.NamespacedName{Name: PolicyRecoName, Namespace: PolicyRecoNamespace},
					updatedPolicy)
				if err != nil {
					return false
				}
				updatedPolicyString, _ := json.MarshalIndent(updatedPolicy, "", "   ")
				fmt.Fprintf(GinkgoWriter, "Updated policy: %s", updatedPolicyString)
				return *updatedPolicy.Spec.QueuedForExecution
			}, timeout, 8*interval).Should(BeFalse())
			Expect(updatedPolicy.Name).Should(Equal(PolicyRecoName))
			Expect(updatedPolicy.Spec.Policy).Should(Equal(policy2.Name))
			Expect(updatedPolicy.Namespace).Should(Equal(PolicyRecoNamespace))
			Expect(updatedPolicy.Spec.GeneratedAt.Time).Should(BeTemporally(">", updatedPolicy.Spec.TransitionedAt.Time))
			Expect(updatedPolicy.Spec.TargetHPAConfiguration.Max).Should(Equal(targetMax))
			Expect(updatedPolicy.Spec.TargetHPAConfiguration.Min).Should(Equal(targetMin))
			Expect(updatedPolicy.Spec.TargetHPAConfiguration.TargetMetricValue).Should(Equal(targetThreshold))
			Expect(updatedPolicy.Spec.CurrentHPAConfiguration.Min).Should(Equal(targetMax - ((targetMax - targetMin) * (policy2.Spec.MinReplicaPercentageCut) / 100)))
			Expect(updatedPolicy.Spec.CurrentHPAConfiguration.Max).Should(Equal(targetMax))
			Expect(updatedPolicy.Spec.CurrentHPAConfiguration.TargetMetricValue).Should(Equal(policy2.Spec.TargetUtilization))
		})
	})

	When("There's no default policy", func() {
		policyreco := &v1alpha1.PolicyRecommendation{}
		createdPolicy := &v1alpha1.PolicyRecommendation{}
		updatedPolicy := &v1alpha1.PolicyRecommendation{}

		var targetMin, targetMax, targetThreshold int
		var safestPolicy, policy1 v1alpha1.Policy
		BeforeEach(func() {
			targetMin = 10
			targetMax = 60
			targetThreshold = 10
			recommender.Min = targetMin
			recommender.Threshold = targetThreshold
			recommender.Max = targetMax
			safestPolicy = v1alpha1.Policy{
				ObjectMeta: metav1.ObjectMeta{Name: "safest-policy"},
				Spec: v1alpha1.PolicySpec{
					IsDefault:               false,
					RiskIndex:               1,
					MinReplicaPercentageCut: 80,
					TargetUtilization:       10,
				},
			}
			policy1 = v1alpha1.Policy{
				ObjectMeta: metav1.ObjectMeta{Name: "policy-1"},
				Spec: v1alpha1.PolicySpec{
					IsDefault:               false,
					RiskIndex:               10,
					MinReplicaPercentageCut: 100,
					TargetUtilization:       15,
				},
			}

			Expect(k8sClient.Create(ctx, &safestPolicy)).Should(Succeed())
			Expect(k8sClient.Create(ctx, &policy1)).Should(Succeed())

			policyreco = &v1alpha1.PolicyRecommendation{}
			createdPolicy = &v1alpha1.PolicyRecommendation{}
			updatedPolicy = &v1alpha1.PolicyRecommendation{}
		})
		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, createdPolicy)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, &safestPolicy)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, &policy1)).Should(Succeed())
		})

		It("With an aggressive target reco", func() {
			targetMin = 10
			targetMax = 60
			targetThreshold = 80
			recommender.Min = targetMin
			recommender.Max = targetMax
			recommender.Threshold = targetThreshold
			ctx := context.Background()
			now := metav1.Now()
			policyreco = &v1alpha1.PolicyRecommendation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PolicyRecoName,
					Namespace: PolicyRecoNamespace,
				},
				Spec: v1alpha1.PolicyRecommendationSpec{
					WorkloadMeta: v1alpha1.WorkloadMeta{
						Name:      PolicyRecoName,
						Namespace: PolicyRecoNamespace,
					},
					Policy:             safestPolicy.Name,
					GeneratedAt:        &now,
					TransitionedAt:     &now,
					QueuedForExecution: &trueBool,
				},
			}
			Expect(k8sClient.Create(ctx, policyreco)).Should(Succeed())
			Eventually(func() bool {
				fmt.Println("In eventually decorator ...")
				err := k8sClient.Get(ctx,
					types.NamespacedName{Name: PolicyRecoName, Namespace: PolicyRecoNamespace},
					createdPolicy)
				if err != nil {
					return false
				}
				return *createdPolicy.Spec.QueuedForExecution
			}, timeout, 8*interval).Should(BeFalse())

			createdPolicyString, _ := json.MarshalIndent(createdPolicy, "", "   ")
			fmt.Fprintf(GinkgoWriter, "Created policy : %s", createdPolicyString)
			Expect(createdPolicy.Name).Should(Equal(PolicyRecoName))
			Expect(createdPolicy.Namespace).Should(Equal(PolicyRecoNamespace))
			Expect(createdPolicy.Spec.Policy).Should(Equal(safestPolicy.Name))
			Expect(*createdPolicy.Spec.QueuedForExecution).Should(BeFalse())
			Expect(createdPolicy.Spec.TargetHPAConfiguration.Max).Should(Equal(targetMax))
			Expect(createdPolicy.Spec.TargetHPAConfiguration.Min).Should(Equal(targetMin))
			Expect(createdPolicy.Spec.TargetHPAConfiguration.TargetMetricValue).Should(Equal(targetThreshold))
			Expect(createdPolicy.Spec.CurrentHPAConfiguration.Min).Should(Equal(targetMax - ((targetMax - targetMin) * (safestPolicy.Spec.MinReplicaPercentageCut) / 100)))
			Expect(createdPolicy.Spec.CurrentHPAConfiguration.Max).Should(Equal(targetMax))
			Expect(createdPolicy.Spec.CurrentHPAConfiguration.TargetMetricValue).Should(Equal(safestPolicy.Spec.TargetUtilization))

			By("Aging the policy once")
			time.Sleep(2 * policyAge)
			Expect(queueRecoByUpdateOp(PolicyRecoName, PolicyRecoNamespace)).Should(Succeed())

			Eventually(func() bool {
				fmt.Println("In eventually decorator ...")
				err := k8sClient.Get(ctx,
					types.NamespacedName{Name: PolicyRecoName, Namespace: PolicyRecoNamespace},
					updatedPolicy)
				if err != nil {
					return false
				}
				updatedPolicyString, _ := json.MarshalIndent(updatedPolicy, "", "   ")
				fmt.Fprintf(GinkgoWriter, "Updated policy: %s", updatedPolicyString)
				return *updatedPolicy.Spec.QueuedForExecution
			}, timeout, 8*interval).Should(BeFalse())
			Expect(updatedPolicy.Name).Should(Equal(PolicyRecoName))
			Expect(updatedPolicy.Spec.Policy).Should(Equal(policy1.Name))
			Expect(updatedPolicy.Namespace).Should(Equal(PolicyRecoNamespace))
			Expect(updatedPolicy.Spec.GeneratedAt).Should(Equal(updatedPolicy.Spec.TransitionedAt))
			Expect(updatedPolicy.Spec.TargetHPAConfiguration.Max).Should(Equal(targetMax))
			Expect(updatedPolicy.Spec.TargetHPAConfiguration.Min).Should(Equal(targetMin))
			Expect(updatedPolicy.Spec.TargetHPAConfiguration.TargetMetricValue).Should(Equal(targetThreshold))
			Expect(updatedPolicy.Spec.CurrentHPAConfiguration.Min).Should(Equal(targetMax - ((targetMax - targetMin) * (policy1.Spec.MinReplicaPercentageCut) / 100)))
			Expect(updatedPolicy.Spec.CurrentHPAConfiguration.Max).Should(Equal(targetMax))
			Expect(updatedPolicy.Spec.CurrentHPAConfiguration.TargetMetricValue).Should(Equal(policy1.Spec.TargetUtilization))

			By("Aging the policy once again and it should be the target reco")
			time.Sleep(2 * policyAge)
			updatedPolicy = &v1alpha1.PolicyRecommendation{}
			Expect(queueRecoByUpdateOp(PolicyRecoName, PolicyRecoNamespace)).Should(Succeed())

			Eventually(func() bool {
				fmt.Println("In eventually decorator ...")
				err := k8sClient.Get(ctx,
					types.NamespacedName{Name: PolicyRecoName, Namespace: PolicyRecoNamespace},
					updatedPolicy)
				if err != nil {
					return false
				}
				updatedPolicyString, _ := json.MarshalIndent(updatedPolicy, "", "   ")
				fmt.Fprintf(GinkgoWriter, "Updated policy: %s", updatedPolicyString)
				return *updatedPolicy.Spec.QueuedForExecution
			}, timeout, 8*interval).Should(BeFalse())
			Expect(updatedPolicy.Name).Should(Equal(PolicyRecoName))
			Expect(updatedPolicy.Spec.Policy).Should(Equal(policy1.Name))
			Expect(updatedPolicy.Namespace).Should(Equal(PolicyRecoNamespace))
			Expect(updatedPolicy.Spec.TargetHPAConfiguration.Max).Should(Equal(targetMax))
			Expect(updatedPolicy.Spec.TargetHPAConfiguration.Min).Should(Equal(targetMin))
			Expect(updatedPolicy.Spec.TargetHPAConfiguration.TargetMetricValue).Should(Equal(targetThreshold))
			Expect(updatedPolicy.Spec.CurrentHPAConfiguration.Min).Should(Equal(targetMax - ((targetMax - targetMin) * (policy1.Spec.MinReplicaPercentageCut) / 100)))
			Expect(updatedPolicy.Spec.CurrentHPAConfiguration.Max).Should(Equal(targetMax))
			Expect(updatedPolicy.Spec.CurrentHPAConfiguration.TargetMetricValue).Should(Equal(policy1.Spec.TargetUtilization))
			Expect(updatedPolicy.Spec.GeneratedAt.Time).Should(BeTemporally(">", updatedPolicy.Spec.TransitionedAt.Time))
		})
	})

	Context("Test predicates", func() {
		policyreco := &v1alpha1.PolicyRecommendation{}
		createdPolicy := &v1alpha1.PolicyRecommendation{}
		updatedPolicy := &v1alpha1.PolicyRecommendation{}
		var safestPolicy *v1alpha1.Policy
		ctx := context.Background()

		BeforeEach(func() {
			safestPolicy = &v1alpha1.Policy{
				ObjectMeta: metav1.ObjectMeta{Name: "safest-policy"},
				Spec: v1alpha1.PolicySpec{
					IsDefault:               false,
					RiskIndex:               1,
					MinReplicaPercentageCut: 80,
					TargetUtilization:       10,
				},
			}
			Expect(k8sClient.Create(ctx, safestPolicy)).Should(Succeed())

		})
		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, safestPolicy)).Should(Succeed())
		})

		It("Should create basic policyreco with 'true' QueueForExecution", func() {
			now := metav1.Now()
			policyreco = &v1alpha1.PolicyRecommendation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PolicyRecoName,
					Namespace: PolicyRecoNamespace,
				},
				Spec: v1alpha1.PolicyRecommendationSpec{
					WorkloadMeta: v1alpha1.WorkloadMeta{
						Name:      PolicyRecoName,
						Namespace: PolicyRecoNamespace,
					},
					Policy:               "",
					GeneratedAt:          &now,
					TransitionedAt:       &now,
					QueuedForExecution:   &trueBool,
					QueuedForExecutionAt: &now,
				},
			}
			Expect(k8sClient.Create(ctx, policyreco)).Should(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, policyreco)).Should(Succeed())
			})
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Namespace: PolicyRecoNamespace,
					Name:      PolicyRecoName,
				}, createdPolicy)
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "error while fetching the created policy. %s", err.Error())
					return true
				}
				return *createdPolicy.Spec.QueuedForExecution
			}).Should(Equal(false))
		})

		It("Should create basic policyreco and then update", func() {
			now := metav1.Now()
			policyreco = &v1alpha1.PolicyRecommendation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PolicyRecoName,
					Namespace: PolicyRecoNamespace,
				},
				Spec: v1alpha1.PolicyRecommendationSpec{
					WorkloadMeta: v1alpha1.WorkloadMeta{
						Name:      PolicyRecoName,
						Namespace: PolicyRecoNamespace,
					},
					Policy:               "",
					GeneratedAt:          &now,
					TransitionedAt:       &now,
					QueuedForExecution:   &falseBool,
					QueuedForExecutionAt: &now,
				},
			}
			Expect(k8sClient.Create(ctx, policyreco)).Should(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, policyreco)).Should(Succeed())
			})
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Namespace: PolicyRecoNamespace,
				Name:      PolicyRecoName,
			}, createdPolicy)).Should(Succeed())
			Expect(*createdPolicy.Spec.QueuedForExecution).Should(BeFalse())
			createdPolicy.Spec.QueuedForExecution = &trueBool
			Expect(k8sClient.Update(ctx, createdPolicy)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Namespace: PolicyRecoNamespace,
					Name:      PolicyRecoName,
				}, createdPolicy)
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "error while fetching the created policy. %s", err.Error())
					return true
				}
				return *createdPolicy.Spec.QueuedForExecution
			}).Should(BeFalse())
		})

		It("Should create basic policyreco and then update", func() {
			now := metav1.Now()
			policyreco = &v1alpha1.PolicyRecommendation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PolicyRecoName,
					Namespace: PolicyRecoNamespace,
				},
				Spec: v1alpha1.PolicyRecommendationSpec{
					WorkloadMeta: v1alpha1.WorkloadMeta{
						Name:      PolicyRecoName,
						Namespace: PolicyRecoNamespace,
					},
					Policy:               "",
					GeneratedAt:          &now,
					TransitionedAt:       &now,
					QueuedForExecution:   &falseBool,
					QueuedForExecutionAt: &now,
				},
			}
			Expect(k8sClient.Create(ctx, policyreco)).Should(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, policyreco)).Should(Succeed())
			})
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Namespace: PolicyRecoNamespace,
				Name:      PolicyRecoName,
			}, createdPolicy)).Should(Succeed())
			Expect(*createdPolicy.Spec.QueuedForExecution).Should(BeFalse())
			// setting the queuedForExecutionAT later than the generatedAt
			generatedAt := metav1.NewTime(now.Add(-time.Minute))
			queuedForExecutionAt := metav1.NewTime(now.Add(-time.Second))
			createdPolicy.Spec.QueuedForExecutionAt = &queuedForExecutionAt
			createdPolicy.Spec.GeneratedAt = &generatedAt
			//time.Sleep(time.Second)
			Expect(k8sClient.Update(ctx, createdPolicy)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Namespace: PolicyRecoNamespace,
					Name:      PolicyRecoName,
				}, updatedPolicy)
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "error while fetching the created policy. %s", err.Error())
					return false
				}
				return updatedPolicy.Spec.GeneratedAt.Time.After(updatedPolicy.Spec.QueuedForExecutionAt.Time)
			}).Should(BeTrue())

		})
	})

	Context("When creating a new Deployment", func() {
		var deployment *appsv1.Deployment
		var createdPolicy *v1alpha1.PolicyRecommendation
		var safestPolicy, policy1, policy2 v1alpha1.Policy
		BeforeEach(func() {
			safestPolicy = v1alpha1.Policy{
				ObjectMeta: metav1.ObjectMeta{Name: "safest-policy"},
				Spec: v1alpha1.PolicySpec{
					IsDefault:               false,
					RiskIndex:               1,
					MinReplicaPercentageCut: 80,
					TargetUtilization:       10,
				},
			}
			policy1 = v1alpha1.Policy{
				ObjectMeta: metav1.ObjectMeta{Name: "policy-1"},
				Spec: v1alpha1.PolicySpec{
					IsDefault:               false,
					RiskIndex:               10,
					MinReplicaPercentageCut: 100,
					TargetUtilization:       15,
				},
			}
			policy2 = v1alpha1.Policy{
				ObjectMeta: metav1.ObjectMeta{Name: "policy-2"},
				Spec: v1alpha1.PolicySpec{
					IsDefault:               true,
					RiskIndex:               20,
					MinReplicaPercentageCut: 100,
					TargetUtilization:       20,
				},
			}
			Expect(k8sClient.Create(ctx, &safestPolicy)).Should(Succeed())
			Expect(k8sClient.Create(ctx, &policy1)).Should(Succeed())
			Expect(k8sClient.Create(ctx, &policy2)).Should(Succeed())
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, deployment)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, &safestPolicy)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, &policy1)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, &policy2)).Should(Succeed())
		})
		It("Should Create a new PolicyRecommendation", func() {
			By("By creating a new Deployment")
			ctx := context.TODO()
			deployment = &appsv1.Deployment{
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
			createdPolicy = &v1alpha1.PolicyRecommendation{}

			time.Sleep(5 * time.Second)

			Eventually(func() bool {
				err := k8sClient.Get(ctx,
					types.NamespacedName{Name: DeploymentName, Namespace: DeploymentNamespace},
					createdDeployment)
				if err != nil {
					return false
				}

				err = k8sClient.Get(ctx,
					types.NamespacedName{Name: DeploymentName, Namespace: DeploymentNamespace},
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

			fmt.Fprintf(GinkgoWriter, "PolicyReco: %v", createdPolicy)
			Expect(len(createdPolicy.Status.Conditions)).To(Equal(2))
			Expect(createdPolicy.Status.Conditions).To(ContainElement(SatisfyAll(
				HaveField("Type", Equal("Initialized")))))
			By("Testing that monitor has been queuedAllRecos")
			Eventually(Expect(queuedAllRecos).Should(BeTrue()))
		})
	})

	Context("Rollout testcases", func() {
		//	TODO(bharathguvvala): Add rollout specific test cases
	})
})

func queueRecoByPatching(name, namespace string) error {
	now := metav1.Now()
	fmt.Fprintf(GinkgoWriter, "Queuing reco at %s \n", now.String())
	TRUE := true
	patchPolicyReco := &v1alpha1.PolicyRecommendation{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PolicyRecommendation",
			APIVersion: "ottoscaler.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.PolicyRecommendationSpec{
			QueuedForExecution:   &TRUE,
			QueuedForExecutionAt: &now,
		},
	}
	err := k8sClient.Patch(ctx, patchPolicyReco, client.Apply, client.ForceOwnership, client.FieldOwner(PolicyRecoWorkflowCtrlName))
	policyString, _ := json.MarshalIndent(patchPolicyReco, "", "   ")
	fmt.Fprintf(GinkgoWriter, "Policy patch after queuing %s \n", policyString)
	return err
}

func queueRecoByUpdateOp(name, namespace string) error {
	now := metav1.Now()
	fmt.Fprintf(GinkgoWriter, "Queuing reco at %s \n", now.String())
	TRUE := true
	patchPolicyReco := &v1alpha1.PolicyRecommendation{}
	err := k8sClient.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, patchPolicyReco)
	if err != nil {
		return err
	}
	patchPolicyReco.Spec.QueuedForExecution = &TRUE
	patchPolicyReco.Spec.QueuedForExecutionAt = &now

	err = k8sClient.Update(ctx, patchPolicyReco)
	policyString, _ := json.MarshalIndent(patchPolicyReco, "", "   ")
	fmt.Fprintf(GinkgoWriter, "Policy after queuing update %s \n", policyString)
	return err
}
