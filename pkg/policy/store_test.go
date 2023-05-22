package policy

import (
	"github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("PolicyStore", func() {

	var policies []v1alpha1.Policy
	AfterEach(func() {
		for _, policy := range policies {
			Expect(k8sClient.Delete(ctx, &policy)).Should(Succeed())
		}
	})
	It("should get the safest policy and next policy", func() {
		By("creating policies")
		policies = []v1alpha1.Policy{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "policy1",
				},
				Spec: v1alpha1.PolicySpec{
					RiskIndex:               1,
					MinReplicaPercentageCut: 1,
					TargetUtilization:       60,
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "policy2",
				},
				Spec: v1alpha1.PolicySpec{
					RiskIndex:               2,
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
		nextPolicy, err := store.GetNextPolicyByName(policies[0].Name)
		Expect(err).NotTo(HaveOccurred())
		Expect(nextPolicy).NotTo(BeNil())
		Expect(nextPolicy.Name).To(Equal("policy2"))

		By("getting the next policy when there is no next policy")
		nextPolicy, err = store.GetNextPolicyByName(policies[1].Name)
		Expect(err).To(HaveOccurred())
		Expect(err).To(Equal(NoNextPolicyFoundErr))
		Expect(nextPolicy).To(BeNil())
	})

	It("should get the safest policy and previous policy", func() {
		By("creating policies")
		policies = []v1alpha1.Policy{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "policy1",
				},
				Spec: v1alpha1.PolicySpec{
					RiskIndex:               1,
					MinReplicaPercentageCut: 1,
					TargetUtilization:       60,
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "policy2",
				},
				Spec: v1alpha1.PolicySpec{
					RiskIndex:               2,
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
		nextPolicy, err := store.GetPreviousPolicyByName(policies[0].Name)
		Expect(err).To(HaveOccurred())
		Expect(err).To(Equal(NoPrevPolicyFoundErr))
		Expect(nextPolicy).To(BeNil())

	})

	It("should get the previous policy", func() {
		By("creating policies")
		policies = []v1alpha1.Policy{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "policy1",
				},
				Spec: v1alpha1.PolicySpec{
					RiskIndex:               1,
					MinReplicaPercentageCut: 1,
					TargetUtilization:       60,
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "policy2",
				},
				Spec: v1alpha1.PolicySpec{
					RiskIndex:               2,
					MinReplicaPercentageCut: 2,
					TargetUtilization:       80,
				},
			},
		}

		for _, p := range policies {
			Expect(k8sClient.Create(ctx, &p)).Should(Succeed())
		}

		By("getting policy2")
		policy2, err := store.GetPolicyByName(policies[1].Name)
		Expect(err).NotTo(HaveOccurred())
		Expect(policy2).NotTo(BeNil())
		Expect(policy2.Name).To(Equal("policy2"))

		By("getting the previous policy")
		prevPolicy, err := store.GetPreviousPolicyByName(policy2.Name)
		Expect(err).NotTo(HaveOccurred())
		Expect(err).To(BeNil())
		Expect(prevPolicy).NotTo(BeNil())
		Expect(prevPolicy.Name).To(Equal(policies[0].Name))

	})

	It("should get the default policy", func() {
		By("creating policies")
		policies = []v1alpha1.Policy{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "policy1",
				},
				Spec: v1alpha1.PolicySpec{
					RiskIndex:               1,
					MinReplicaPercentageCut: 1,
					TargetUtilization:       60,
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "policy2",
				},
				Spec: v1alpha1.PolicySpec{
					IsDefault:               true,
					RiskIndex:               2,
					MinReplicaPercentageCut: 2,
					TargetUtilization:       80,
				},
			},
		}

		for _, p := range policies {
			Expect(k8sClient.Create(ctx, &p)).Should(Succeed())
		}

		By("getting default policy")
		policy2, err := store.GetDefaultPolicy()
		Expect(err).NotTo(HaveOccurred())
		Expect(policy2).NotTo(BeNil())
		Expect(policy2.Name).To(Equal("policy2"))

	})

	It("should get the sorted list of policies", func() {
		By("creating policies")
		policies = []v1alpha1.Policy{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "policy1",
				},
				Spec: v1alpha1.PolicySpec{
					RiskIndex:               3,
					MinReplicaPercentageCut: 1,
					TargetUtilization:       60,
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "policy2",
				},
				Spec: v1alpha1.PolicySpec{
					IsDefault:               true,
					RiskIndex:               1,
					MinReplicaPercentageCut: 2,
					TargetUtilization:       80,
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "policy3",
				},
				Spec: v1alpha1.PolicySpec{
					RiskIndex:               2,
					MinReplicaPercentageCut: 20,
					TargetUtilization:       60,
				},
			},
		}

		for _, p := range policies {
			Expect(k8sClient.Create(ctx, &p)).Should(Succeed())
		}

		By("getting sorted list of policies")
		sortedPolicies, err := store.GetSortedPolicies()
		Expect(err).NotTo(HaveOccurred())
		Expect(sortedPolicies).NotTo(BeNil())

		Expect(sortedPolicies.Items).NotTo(BeNil())
		Expect(sortedPolicies.Items[0].Name).To(Equal(policies[1].Name))
		Expect(sortedPolicies.Items[1].Name).To(Equal(policies[2].Name))
		Expect(sortedPolicies.Items[2].Name).To(Equal(policies[0].Name))

	})
})
