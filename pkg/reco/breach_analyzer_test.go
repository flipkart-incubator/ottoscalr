package reco

import (
	"context"
	"fmt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("BreachAnalyzer policy iterator", func() {

	const DeploymentName = "test-deploy-9u91l"
	const DeploymentNamespace = "test-namespace"
	var breachAnalyzer PolicyIterator
	var wm WorkloadMeta
	var cpuRedline float64

	ctx := context.TODO()

	BeforeEach(func() {
		cpuRedline = 85.0
		var err error
		breachAnalyzer, err = NewBreachAnalyzer(fakeK8SClient, fakeScraper, cpuRedline, metricStep)
		Expect(err).To(BeNil())
		wm = WorkloadMeta{
			Name:      DeploymentName,
			Namespace: DeploymentNamespace,
		}
	})

	Context("When BreachAnalyzer PI is invoked", func() {
		BeforeEach(func() {
			Expect(createPolicyReco(DeploymentName, DeploymentNamespace, "policy-2")).Should(Succeed())
		})
		AfterEach(func() {
			Expect(deletePolicyReco(DeploymentName, DeploymentNamespace)).Should(Succeed())
		})

		It("Should downgrade upon breach", func() {

			policy, err := breachAnalyzer.NextPolicy(ctx, wm)
			Expect(err).To(BeNil())
			Expect(policy).NotTo(BeNil())
			Expect(policy.Name).Should(Equal(policy1.Name))
			Expect(updatePolicyRecoWithPolicy(DeploymentName, DeploymentNamespace, policy1.Name)).Should(Succeed())
			Expect(func() string {
				policy, err := fetchPolicyReco(DeploymentName, DeploymentNamespace)
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "Error %s\n", err.Error())
					return ""
				}
				fmt.Fprintf(GinkgoWriter, "Fetched policyReco %v\n", policy)
				return policy.Spec.Policy
			}()).Should(Equal(policy1.Name))

			policy, err = breachAnalyzer.NextPolicy(ctx, wm)
			Expect(err).To(BeNil())
			Expect(policy).NotTo(BeNil())
			Expect(policy.Name).Should(Equal(safestPolicy.Name))
			Expect(updatePolicyRecoWithPolicy(DeploymentName, DeploymentNamespace, safestPolicy.Name)).Should(Succeed())
			Expect(func() string {
				policy, err := fetchPolicyReco(DeploymentName, DeploymentNamespace)
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "Error %s\n", err.Error())
					return ""
				}
				fmt.Fprintf(GinkgoWriter, "Fetched policyReco %v\n", policy)
				return policy.Spec.Policy
			}()).Should(Equal(safestPolicy.Name))

			policy, err = breachAnalyzer.NextPolicy(ctx, wm)
			Expect(err).To(BeNil())
			Expect(policy).To(BeNil())

		})
	})
})
