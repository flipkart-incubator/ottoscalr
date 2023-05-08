package controller

import (
	"context"
	ottoscaleriov1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"time"
)

var _ = Describe("PolicyWatcher controller", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	BeforeEach(func() {
		queuedAllRecos = false
	})
	var policy1, policy2, policy3 ottoscaleriov1alpha1.Policy
	Context("When updating default policy", func() {
		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, &policy1)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, &policy2)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, &policy3)).Should(Succeed())
		})
		It("Should mark other policies as non-degault and requeue all policy recommendations ", func() {
			By("Seeding all policies")
			ctx := context.Background()

			policy1 = ottoscaleriov1alpha1.Policy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "policy1",
					Namespace: "default",
				},
				Spec: ottoscaleriov1alpha1.PolicySpec{
					IsDefault: true,
				},
			}
			policy2 = ottoscaleriov1alpha1.Policy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "policy2",
					Namespace: "default",
				},
				Spec: ottoscaleriov1alpha1.PolicySpec{
					IsDefault: false,
				},
			}
			policy3 = ottoscaleriov1alpha1.Policy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "policy3",
					Namespace: "default",
				},
				Spec: ottoscaleriov1alpha1.PolicySpec{
					IsDefault: false,
				},
			}

			Expect(k8sClient.Create(ctx, &policy1)).Should(Succeed())
			Expect(k8sClient.Create(ctx, &policy2)).Should(Succeed())
			Expect(k8sClient.Create(ctx, &policy3)).Should(Succeed())

			policy2.Spec.IsDefault = true
			err := k8sClient.Update(ctx, &policy2)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() bool {
				updatedPolicy1 := ottoscaleriov1alpha1.Policy{}
				updatedPolicy2 := ottoscaleriov1alpha1.Policy{}
				updatedPolicy3 := ottoscaleriov1alpha1.Policy{}
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: "default",
					Name: "policy1"}, &updatedPolicy1)
				if err != nil {
					return false
				}

				err = k8sClient.Get(ctx, types.NamespacedName{Namespace: "default",
					Name: "policy2"}, &updatedPolicy2)
				if err != nil {
					return false
				}

				err = k8sClient.Get(ctx, types.NamespacedName{Namespace: "default",
					Name: "policy3"}, &updatedPolicy3)
				if err != nil {
					return false
				}
				if updatedPolicy1.Spec.IsDefault || !updatedPolicy2.Spec.IsDefault || updatedPolicy3.Spec.IsDefault {
					return false
				}

				return true
			}, timeout, interval).Should(BeTrue())

			By("Testing that queuedAllRecos was called")
			Eventually(Expect(queuedAllRecos).Should(BeTrue()))
		})
	})
})
