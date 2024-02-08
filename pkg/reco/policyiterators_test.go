package reco

import (
	"context"
	"fmt"
	"time"

	ottoscaleriov1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("PolicyIterators", func() {

	const DeploymentName = "test-deploy-tul19"
	const DeploymentNamespace = "default"
	var defaultPI, agingPI PolicyIterator
	var wm WorkloadMeta

	ctx := context.TODO()

	BeforeEach(func() {
		defaultPI = NewDefaultPolicyIterator(fakeK8SClient, clientsRegistry)

		Expect(defaultPI).NotTo(BeNil())
		Expect(defaultPI.GetName()).Should(Equal("DefaultPolicy"))
		agingPI = NewAgingPolicyIterator(fakeK8SClient, policyAge)
		Expect(agingPI).NotTo(BeNil())
		wm = WorkloadMeta{
			Name:      DeploymentName,
			Namespace: DeploymentNamespace,
		}

	})

	Context("DefaultPolicyIterator", func() {
		It("Should return only default policy", func() {
			policy, err := defaultPI.NextPolicy(ctx, wm)
			Expect(err).To(BeNil())
			Expect(policy).NotTo(BeNil())
			Expect(policy.Name).Should(Equal(policy1.Name))
		})

		It("Should return workload specific default policy if default policy annotation is set", func() {
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DeploymentName,
					Namespace: DeploymentNamespace,
					Annotations: map[string]string{
						DefaultPolicyAnnotation: policy2.Name,
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
			wm.Kind = "Deployment"
			Expect(k8sClient.Create(ctx, deployment)).Should(Succeed())
			time.Sleep(1 * time.Second)
			policy, err := defaultPI.NextPolicy(ctx, wm)
			Expect(err).To(BeNil())
			Expect(policy).NotTo(BeNil())
			Expect(policy.Name).Should(Equal(policy2.Name))
		})
	})

	Context("AgingPolicyIterator", func() {
		BeforeEach(func() {
			Expect(createPolicyReco(DeploymentName, DeploymentNamespace, "")).Should(Succeed())
		})
		AfterEach(func() {
			Expect(deletePolicyReco(DeploymentName, DeploymentNamespace)).Should(Succeed())
		})
		It("Should age policies", func() {

			policy, err := agingPI.NextPolicy(ctx, wm)
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

			By("Aging the policy once")
			time.Sleep(2 * policyAge)
			policy, err = agingPI.NextPolicy(ctx, wm)
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

			By("Aging the policy once more")
			time.Sleep(2 * policyAge)
			policy, err = agingPI.NextPolicy(ctx, wm)
			Expect(err).To(BeNil())
			Expect(policy).NotTo(BeNil())
			Expect(policy.Name).Should(Equal(policy2.Name))
			Expect(updatePolicyRecoWithPolicy(DeploymentName, DeploymentNamespace, policy2.Name)).Should(Succeed())
			Expect(func() string {
				policy, err := fetchPolicyReco(DeploymentName, DeploymentNamespace)
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "Error %s\n", err.Error())
					return ""
				}
				fmt.Fprintf(GinkgoWriter, "Fetched policyReco %v\n", policy)
				return policy.Spec.Policy
			}()).Should(Equal(policy2.Name))

			By("Aging the policy once more")
			time.Sleep(2 * policyAge)
			policy, err = agingPI.NextPolicy(ctx, wm)
			Expect(err).To(BeNil())
			Expect(policy).NotTo(BeNil())
			Expect(policy.Name).Should(Equal(policy2.Name))
		})

		It("Should update policyreco with nonexistent policy", func() {

			policy, err := agingPI.NextPolicy(ctx, wm)
			Expect(err).To(BeNil())
			Expect(policy).NotTo(BeNil())
			Expect(policy.Name).Should(Equal(safestPolicy.Name))
			Expect(updatePolicyRecoWithPolicy(DeploymentName, DeploymentNamespace, "nonexistent-policy")).Should(Succeed())
			Expect(func() string {
				policy, err := fetchPolicyReco(DeploymentName, DeploymentNamespace)
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "Error %s\n", err.Error())
					return ""
				}
				fmt.Fprintf(GinkgoWriter, "Fetched policyReco %v\n", policy)
				return policy.Spec.Policy
			}()).Should(Equal("nonexistent-policy"))

			By("Aging the policy once")
			time.Sleep(2 * policyAge)
			policy, err = agingPI.NextPolicy(ctx, wm)
			Expect(err).NotTo(HaveOccurred())
			Expect(policy).NotTo(BeNil())
			Expect(policy.Name).Should(Equal(safestPolicy.Name))
		})
	})
})

func createPolicyReco(name, namespace, policy string) error {
	now := metav1.Now()
	return fakeK8SClient.Create(ctx, &ottoscaleriov1alpha1.PolicyRecommendation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: ottoscaleriov1alpha1.PolicyRecommendationSpec{
			WorkloadMeta:            ottoscaleriov1alpha1.WorkloadMeta{},
			CurrentHPAConfiguration: ottoscaleriov1alpha1.HPAConfiguration{},
			Policy:                  policy,
			GeneratedAt:             &now,
			TransitionedAt:          &now,
		},
	})
}
func deletePolicyReco(name, namespace string) error {
	return fakeK8SClient.Delete(ctx, &ottoscaleriov1alpha1.PolicyRecommendation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	})
}

func fetchPolicyReco(name, namespace string) (ottoscaleriov1alpha1.PolicyRecommendation, error) {
	policyReco := &ottoscaleriov1alpha1.PolicyRecommendation{}
	err := fakeK8SClient.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, policyReco)
	return *policyReco, err
}

func updatePolicyRecoWithPolicy(name, namespace, policy string) error {
	policyReco := &ottoscaleriov1alpha1.PolicyRecommendation{}
	if err := fakeK8SClient.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, policyReco); err != nil {
		return err
	}
	now := metav1.Now()
	if policyReco.Spec.Policy != policy {
		policyReco.Spec.TransitionedAt = &now
		policyReco.Spec.Policy = policy
	}
	policyReco.Spec.GeneratedAt = &now
	err := fakeK8SClient.Update(ctx, policyReco)
	fmt.Fprintf(GinkgoWriter, "Update %v", policyReco)
	return err
}
