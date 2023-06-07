package reco

import (
	"context"
	"fmt"
	ottoscaleriov1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	"github.com/flipkart-incubator/ottoscalr/pkg/metrics"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"time"
)

var _ = Describe("BreachAnalyzer policy iterator", func() {

	const DeploymentName = "test-deploy-9u91l"
	const DeploymentNamespace = "test-namespace"
	var breachAnalyzer PolicyIterator
	var wm WorkloadMeta
	var cpuRedline float64
	var fakeP8sScraper *FakeScraper

	ctx := context.TODO()

	BeforeEach(func() {
		cpuRedline = 85.0
		var err error

		Expect(err).To(BeNil())
		wm = WorkloadMeta{
			Name:      DeploymentName,
			Namespace: DeploymentNamespace,
		}
	})

	Context("When BreachAnalyzer PI is invoked", func() {
		BeforeEach(func() {
			Expect(createPolicyReco(DeploymentName, DeploymentNamespace, "policy-2")).Should(Succeed())
			var cpuUtil, breaches []metrics.DataPoint
			acl := 5 * time.Minute
			fakeP8sScraper = newFakeScraper(cpuUtil, breaches, acl)
			Expect(fakeP8sScraper).NotTo(BeNil())
			var err error
			breachAnalyzer, err = NewBreachAnalyzer(fakeK8SClient, fakeP8sScraper, cpuRedline, metricStep)
			Expect(breachAnalyzer).NotTo(BeNil())
			Expect(err).To(BeNil())
		})
		AfterEach(func() {
			Expect(deletePolicyReco(DeploymentName, DeploymentNamespace)).Should(Succeed())
		})

		It("Should downgrade upon breach", func() {
			// enable breaches
			fakeP8sScraper.BreachDataPoints = []metrics.DataPoint{{Timestamp: time.Now(), Value: 1.3}}

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

		It("Should not downgrade when there's no breach", func() {
			policy, err := breachAnalyzer.NextPolicy(ctx, wm)
			Expect(err).To(BeNil())
			Expect(policy).To(BeNil())

			policy, err = breachAnalyzer.NextPolicy(ctx, wm)
			Expect(err).To(BeNil())
			Expect(policy).To(BeNil())

		})

		It("Should fallback to no op if there's no generatedAt field", func() {
			updatePolicyRecoGeneratedAtFieldWithNil(DeploymentName, DeploymentNamespace)
			policy, err := breachAnalyzer.NextPolicy(ctx, wm)
			Expect(err).To(BeNil())
			Expect(policy).To(BeNil())
		})
	})
})

func updatePolicyRecoGeneratedAtFieldWithNil(name, namespace string) error {
	policyReco := &ottoscaleriov1alpha1.PolicyRecommendation{}
	if err := fakeK8SClient.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, policyReco); err != nil {
		return err
	}
	policyReco.Spec.GeneratedAt = nil
	err := fakeK8SClient.Update(ctx, policyReco)
	fmt.Fprintf(GinkgoWriter, "Update %v", policyReco)
	return err
}
