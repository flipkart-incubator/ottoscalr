package controller

import (
	"context"
	"fmt"
	"time"

	ottoscaleriov1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("PolicyWatcher controller", func() {
	const (
		timeout             = time.Second * 10
		interval            = time.Millisecond * 250
		PolicyRecoName      = "test-deployment-afgre"
		PolicyRecoNamespace = "default"
	)

	BeforeEach(func() {
		queuedAllRecos = false
	})
	var policy1, policy2, policy3 ottoscaleriov1alpha1.Policy
	Context("When updating default policy", func() {

		It("Should mark other policies as non-default and requeue all policy recommendations ", func() {
			By("Seeding all policies")
			ctx := context.TODO()

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

			time.Sleep(2 * time.Second)
			policy2 = ottoscaleriov1alpha1.Policy{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "policy2"}, &policy2)).Should(Succeed())
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

			Expect(k8sClient.Delete(ctx, &policy1)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, &policy2)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, &policy3)).Should(Succeed())
			time.Sleep(1 * time.Second)
		})
	})
	Context("When handling Add/Update/Delete on a Policy", func() {
		policyreco1 := &ottoscaleriov1alpha1.PolicyRecommendation{}
		policyreco2 := &ottoscaleriov1alpha1.PolicyRecommendation{}

		It("Should add finalizer if not present and requeue all policyrecommendations having this policy ", func() {
			By("Seeding all policies")
			ctx := context.TODO()

			policy1 = ottoscaleriov1alpha1.Policy{
				ObjectMeta: metav1.ObjectMeta{Name: "policy1", Namespace: "defualt"},
				Spec: ottoscaleriov1alpha1.PolicySpec{
					IsDefault:               false,
					RiskIndex:               10,
					MinReplicaPercentageCut: 100,
					TargetUtilization:       15,
				},
			}
			policy2 = ottoscaleriov1alpha1.Policy{
				ObjectMeta: metav1.ObjectMeta{Name: "policy2", Namespace: "defualt"},
				Spec: ottoscaleriov1alpha1.PolicySpec{
					IsDefault:               true,
					RiskIndex:               20,
					MinReplicaPercentageCut: 100,
					TargetUtilization:       20,
				},
			}
			policy3 = ottoscaleriov1alpha1.Policy{
				ObjectMeta: metav1.ObjectMeta{Name: "policy3", Namespace: "defualt"},
				Spec: ottoscaleriov1alpha1.PolicySpec{
					IsDefault:               false,
					RiskIndex:               30,
					MinReplicaPercentageCut: 100,
					TargetUtilization:       30,
				},
			}

			Expect(k8sClient.Create(ctx, &policy1)).Should(Succeed())
			Expect(k8sClient.Create(ctx, &policy2)).Should(Succeed())
			Expect(k8sClient.Create(ctx, &policy3)).Should(Succeed())

			time.Sleep(2 * time.Second)

			//check if finalizer is added
			policy1 = ottoscaleriov1alpha1.Policy{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "policy1"}, &policy1)).Should(Succeed())
			Expect(policy1.Finalizers).ShouldNot(BeNil())
			Expect(policy1.Finalizers).Should(ContainElement(policyFinalizerName))

			policy2 = ottoscaleriov1alpha1.Policy{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "policy2"}, &policy2)).Should(Succeed())
			Expect(policy2.Finalizers).ShouldNot(BeNil())
			Expect(policy2.Finalizers).Should(ContainElement(policyFinalizerName))

			policy3 = ottoscaleriov1alpha1.Policy{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "policy3"}, &policy3)).Should(Succeed())
			Expect(policy3.Finalizers).ShouldNot(BeNil())
			Expect(policy3.Finalizers).Should(ContainElement(policyFinalizerName))

			//create two policy recommendations for policy2
			now := metav1.Now()
			policyreco1 = &ottoscaleriov1alpha1.PolicyRecommendation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PolicyRecoName,
					Namespace: PolicyRecoNamespace,
				},
				Spec: ottoscaleriov1alpha1.PolicyRecommendationSpec{
					WorkloadMeta: ottoscaleriov1alpha1.WorkloadMeta{
						Name: PolicyRecoName,
					},
					Policy:             policy2.Name,
					GeneratedAt:        &now,
					TransitionedAt:     &now,
					QueuedForExecution: &falseBool,
				},
			}
			policyreco2 = &ottoscaleriov1alpha1.PolicyRecommendation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      PolicyRecoName + "2",
					Namespace: PolicyRecoNamespace,
				},
				Spec: ottoscaleriov1alpha1.PolicyRecommendationSpec{
					WorkloadMeta: ottoscaleriov1alpha1.WorkloadMeta{
						Name: PolicyRecoName + "2",
					},
					Policy:             policy2.Name,
					GeneratedAt:        &now,
					TransitionedAt:     &now,
					QueuedForExecution: &falseBool,
				},
			}

			Expect(k8sClient.Create(ctx, policyreco1)).Should(Succeed())
			Expect(k8sClient.Create(ctx, policyreco2)).Should(Succeed())

			time.Sleep(2 * time.Second)

			//update policy2 now
			policy2.Spec.RiskIndex = 4
			queuedOneReco = nil
			err := k8sClient.Update(ctx, &policy2)
			Expect(err).ToNot(HaveOccurred())

			time.Sleep(2 * time.Second)
			//check if policyreco1 and policyreco2 are queued for execution
			Expect(len(queuedOneReco)).Should(Equal(2))
			Expect(queuedOneReco[0]).Should(Equal(true))
			Expect(queuedOneReco[1]).Should(Equal(true))

			//delete policy2 now, again check if policyreco1 and policyreco2 are queued for execution
			queuedOneReco = nil
			policy2 := ottoscaleriov1alpha1.Policy{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "policy2"}, &policy2)).Should(Succeed())
			fmt.Println(policy2)
			Expect(k8sClient.Delete(ctx, &policy2)).Should(Succeed())
			time.Sleep(5 * time.Second)
			Expect(len(queuedOneReco)).Should(Equal(2))
			Expect(queuedOneReco[0]).Should(Equal(true))
			Expect(queuedOneReco[1]).Should(Equal(true))

			Expect(k8sClient.Delete(ctx, &policy1)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, &policy3)).Should(Succeed())
		})
	})
})
