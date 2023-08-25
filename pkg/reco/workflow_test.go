package reco

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("RecommendationWorkflow", func() {
	var recoWorkflowBuilder *RecoWorkflowBuilder

	BeforeEach(func() {
		recoWorkflowBuilder = NewRecommendationWorkflowBuilder()
	})
	AfterEach(func() {
		recoWorkflowBuilder = nil
	})

	Context("Test the builder", func() {

		It("Creates a reco workflow", func() {

			recoWorkflow, err := recoWorkflowBuilder.WithRecommender(&MockRecommender{
				Min:       10,
				Threshold: 50,
				Max:       20,
			}).WithPolicyIterator(&MockNoOpPI{}).WithMinRequiredReplicas(3).WithPolicyStore(store).Build()
			Expect(recoWorkflow).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			Expect(recoWorkflowBuilder.logger).NotTo(BeNil())
			Expect(recoWorkflowBuilder.recommender).NotTo(BeNil())
			Expect(recoWorkflowBuilder.policyIterators).NotTo(BeNil())
			Expect(len(recoWorkflowBuilder.policyIterators)).To(Equal(1))
			Expect(recoWorkflowBuilder.policyIterators["no-op"]).NotTo(BeNil())
			Expect(recoWorkflowBuilder.policyIterators["no-op"].GetName()).To(Equal("no-op"))
		})
	})

	Context("Test with only Recommender and no PIs", func() {
		It("Creates a reco workflow", func() {

			recoWorkflow, err := recoWorkflowBuilder.WithRecommender(&MockRecommender{
				Min:       10,
				Threshold: 18,
				Max:       20,
			}).WithMinRequiredReplicas(3).WithPolicyStore(store).Build()
			Expect(recoWorkflow).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			Expect(recoWorkflowBuilder.logger).NotTo(BeNil())
			Expect(recoWorkflowBuilder.recommender).NotTo(BeNil())
			Expect(recoWorkflowBuilder.policyIterators).To(BeNil())

			nextConfig, targetConfig, policy, err := recoWorkflow.Execute(ctx, WorkloadMeta{
				Name:      "test",
				Namespace: "test",
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(targetConfig).NotTo(BeNil())
			Expect(nextConfig).NotTo(BeNil())
			Expect(policy).To(Equal(&Policy{Name: policy1.Name, RiskIndex: policy1.Spec.RiskIndex,
				MinReplicaPercentageCut: policy1.Spec.MinReplicaPercentageCut,
				TargetUtilization:       policy1.Spec.TargetUtilization}))

			Expect(targetConfig.Max).To(Equal(20))
			Expect(targetConfig.Min).To(Equal(10))
			Expect(targetConfig.TargetMetricValue).To(Equal(18))

			Expect(nextConfig.Max).To(Equal(20))
			Expect(nextConfig.Min).To(Equal(10))
			Expect(nextConfig.TargetMetricValue).To(Equal(18))
		})
	})

	Context("Test with no Recommender and some PIs", func() {
		It("Creates a reco workflow", func() {

			recoWorkflow, err := recoWorkflowBuilder.WithPolicyIterator(&MockNoOpPI{}).WithMinRequiredReplicas(3).Build()
			Expect(recoWorkflow).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			Expect(recoWorkflowBuilder.logger).NotTo(BeNil())
			Expect(recoWorkflowBuilder.recommender).To(BeNil())
			Expect(recoWorkflowBuilder.policyIterators).NotTo(BeNil())

			_, _, _, err = recoWorkflow.Execute(ctx, WorkloadMeta{
				Name:      "test",
				Namespace: "test",
			})

			Expect(err).To(HaveOccurred())
		})
	})

	Context("Test with a valid recommender and PIs", func() {
		It("Generates recommendations", func() {

			mockPolicy = &Policy{
				Name:                    "mockPolicy",
				RiskIndex:               10,
				MinReplicaPercentageCut: 90,
				TargetUtilization:       20,
			}
			DeferCleanup(func() {
				mockPolicy = nil
			})
			recoWorkflow, err := recoWorkflowBuilder.WithRecommender(&MockRecommender{
				Min:       10,
				Threshold: 50,
				Max:       20,
			}).WithPolicyIterator(&MockPI{}).WithMinRequiredReplicas(3).WithPolicyStore(store).Build()
			Expect(recoWorkflow).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			Expect(recoWorkflowBuilder.logger).NotTo(BeNil())
			Expect(recoWorkflowBuilder.recommender).NotTo(BeNil())
			Expect(recoWorkflowBuilder.policyIterators).NotTo(BeNil())
			Expect(len(recoWorkflowBuilder.policyIterators)).To(Equal(1))
			Expect(recoWorkflowBuilder.policyIterators["mockPI"]).NotTo(BeNil())
			Expect(recoWorkflowBuilder.policyIterators["mockPI"].GetName()).To(Equal("mockPI"))

			nextConfig, targetConfig, policy, err := recoWorkflow.Execute(ctx, WorkloadMeta{
				Name:      "test",
				Namespace: "test",
			})
			Expect(err).To(BeNil())
			Expect(targetConfig.Max).To(Equal(20))
			Expect(targetConfig.Min).To(Equal(10))
			Expect(targetConfig.TargetMetricValue).To(Equal(50))

			Expect(nextConfig.Max).To(Equal(20))
			Expect(nextConfig.Min).To(Equal(11))
			Expect(nextConfig.TargetMetricValue).To(Equal(20))

			Expect(policy).NotTo(BeNil())
			Expect(policy.Name).To(Equal("mockPolicy"))
			Expect(policy.MinReplicaPercentageCut).To(Equal(90))
			Expect(policy.RiskIndex).To(Equal(10))
			Expect(policy.TargetUtilization).To(Equal(20))

		})

		Context("Test with a valid recommender and PIs with target reco safer than policy", func() {
			It("Generates recommendations", func() {

				mockPolicy = &Policy{
					Name:                    "mockPolicy",
					RiskIndex:               10,
					MinReplicaPercentageCut: 100,
					TargetUtilization:       60,
				}
				DeferCleanup(func() {
					mockPolicy = nil
				})
				recoWorkflow, err := recoWorkflowBuilder.WithRecommender(&MockRecommender{
					Min:       10,
					Threshold: 50,
					Max:       20,
				}).WithPolicyIterator(&MockPI{}).WithMinRequiredReplicas(3).WithPolicyStore(store).Build()
				Expect(recoWorkflow).NotTo(BeNil())
				Expect(err).NotTo(HaveOccurred())
				Expect(recoWorkflowBuilder.logger).NotTo(BeNil())
				Expect(recoWorkflowBuilder.recommender).NotTo(BeNil())
				Expect(recoWorkflowBuilder.policyIterators).NotTo(BeNil())
				Expect(len(recoWorkflowBuilder.policyIterators)).To(Equal(1))
				Expect(recoWorkflowBuilder.policyIterators["mockPI"]).NotTo(BeNil())
				Expect(recoWorkflowBuilder.policyIterators["mockPI"].GetName()).To(Equal("mockPI"))

				nextConfig, targetConfig, policy, err := recoWorkflow.Execute(ctx, WorkloadMeta{
					Name:      "test",
					Namespace: "test",
				})
				Expect(err).To(BeNil())
				Expect(targetConfig.Max).To(Equal(20))
				Expect(targetConfig.Min).To(Equal(10))
				Expect(targetConfig.TargetMetricValue).To(Equal(50))

				Expect(nextConfig.Max).To(Equal(20))
				Expect(nextConfig.Min).To(Equal(10))
				Expect(nextConfig.TargetMetricValue).To(Equal(50))

				Expect(policy).To(Equal(&Policy{Name: policy2.Name, RiskIndex: policy2.Spec.RiskIndex,
					MinReplicaPercentageCut: policy2.Spec.MinReplicaPercentageCut,
					TargetUtilization:       policy2.Spec.TargetUtilization}))

			})
		})

		Context("MaxReplicas and MinReplicas both less than 3", func() {
			It("should return the processed targetRecoConfig", func() {

				//Both less than 3
				mockPolicy = &Policy{
					Name:                    "mockPolicy",
					RiskIndex:               10,
					MinReplicaPercentageCut: 100,
					TargetUtilization:       60,
				}
				DeferCleanup(func() {
					mockPolicy = nil
				})
				recoWorkflow, err := recoWorkflowBuilder.WithRecommender(&MockRecommender{
					Min:       1,
					Threshold: 50,
					Max:       2,
				}).WithPolicyIterator(&MockPI{}).WithMinRequiredReplicas(3).WithPolicyStore(store).Build()
				Expect(recoWorkflow).NotTo(BeNil())
				Expect(err).NotTo(HaveOccurred())
				_, targetConfig, _, err := recoWorkflow.Execute(ctx, WorkloadMeta{
					Name:      "test",
					Namespace: "test",
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(targetConfig.Min).To(Equal(1))
				Expect(targetConfig.Max).To(Equal(2))
				Expect(targetConfig.TargetMetricValue).To(Equal(50))
			})
		})

		Context("MaxReplicas and MinReplicas both greater than or equal to 3", func() {
			It("should return the processed targetRecoConfig", func() {

				//Both greater than or equal to 3
				mockPolicy = &Policy{
					Name:                    "mockPolicy",
					RiskIndex:               10,
					MinReplicaPercentageCut: 100,
					TargetUtilization:       60,
				}
				DeferCleanup(func() {
					mockPolicy = nil
				})
				recoWorkflow, err := recoWorkflowBuilder.WithRecommender(&MockRecommender{
					Min:       6,
					Threshold: 50,
					Max:       10,
				}).WithPolicyIterator(&MockPI{}).WithMinRequiredReplicas(3).WithPolicyStore(store).Build()
				Expect(recoWorkflow).NotTo(BeNil())
				Expect(err).NotTo(HaveOccurred())
				_, targetConfig, _, err := recoWorkflow.Execute(ctx, WorkloadMeta{
					Name:      "test",
					Namespace: "test",
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(targetConfig.Min).To(Equal(6))
				Expect(targetConfig.Max).To(Equal(10))
				Expect(targetConfig.TargetMetricValue).To(Equal(50))
			})
		})

		Context("MaxReplicas greater than or equal to 3 and minReplicas less than 3", func() {
			It("should return the processed targetRecoConfig", func() {

				mockPolicy = &Policy{
					Name:                    "mockPolicy",
					RiskIndex:               10,
					MinReplicaPercentageCut: 100,
					TargetUtilization:       60,
				}
				DeferCleanup(func() {
					mockPolicy = nil
				})
				recoWorkflow, err := recoWorkflowBuilder.WithRecommender(&MockRecommender{
					Min:       1,
					Threshold: 50,
					Max:       20,
				}).WithPolicyIterator(&MockPI{}).WithMinRequiredReplicas(3).WithPolicyStore(store).Build()
				Expect(recoWorkflow).NotTo(BeNil())
				Expect(err).NotTo(HaveOccurred())
				_, targetConfig, _, err := recoWorkflow.Execute(ctx, WorkloadMeta{
					Name:      "test",
					Namespace: "test",
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(targetConfig.Min).To(Equal(3))
				Expect(targetConfig.Max).To(Equal(20))
				Expect(targetConfig.TargetMetricValue).To(Equal(50))
			})
		})

	})
})
