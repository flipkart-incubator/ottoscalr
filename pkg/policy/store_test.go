package policy

import (
	"github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("PolicyStore", func() {

	It("should get the safest policy and next policy", func() {
		By("creating policies")
		policies := []v1alpha1.Policy{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "policy1",
				},
				Spec: v1alpha1.PolicySpec{
					RiskIndex:               "1",
					MinReplicaPercentageCut: 1,
					TargetUtilization:       60,
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "policy2",
				},
				Spec: v1alpha1.PolicySpec{
					RiskIndex:               "2",
					MinReplicaPercentageCut: 2,
					TargetUtilization:       80,
				},
			},
		}

		for _, p := range policies {
			Expect(k8sClient.Create(ctx, &p)).Should(Succeed())
		}

		By("getting the safest policy")
		safestPolicy, err := store.GetSafestPolicy()
		Expect(err).NotTo(HaveOccurred())
		Expect(safestPolicy).NotTo(BeNil())
		Expect(safestPolicy.Name).To(Equal("policy1"))

		By("getting the next policy")
		nextPolicy, err := store.GetNextPolicy(&policies[0])
		Expect(err).NotTo(HaveOccurred())
		Expect(nextPolicy).NotTo(BeNil())
		Expect(nextPolicy.Name).To(Equal("policy2"))

		By("getting the next policy when there is no next policy")
		nextPolicy, err = store.GetNextPolicy(&policies[1])
		Expect(err).To(HaveOccurred())
		Expect(nextPolicy).To(BeNil())
	})
})
