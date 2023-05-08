package trigger

import (
	"context"
	ottoscaleriov1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"time"
)

var _ = Describe("K8sTriggerHandler", func() {
	var (
		handler *K8sTriggerHandler
		ctx     context.Context
	)

	BeforeEach(func() {
		handler = NewK8sTriggerHandler(k8sClient, zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
		ctx = context.TODO()
	})

	Context("For QueueForExecution", func() {
		It("should update PolicyRecommendation's QueuedForExecution field", func() {
			// Create a new PolicyRecommendation
			policyRecommendation := &ottoscaleriov1alpha1.PolicyRecommendation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy-recommendation",
					Namespace: "default",
				},
				Spec: ottoscaleriov1alpha1.PolicyRecommendationSpec{
					QueuedForExecution: false,
				},
			}
			err := k8sClient.Create(ctx, policyRecommendation)
			Expect(err).ToNot(HaveOccurred())

			// Start the handler
			handler.Start()

			// Queue the PolicyRecommendation for execution
			handler.QueueForExecution(types.NamespacedName{Name: policyRecommendation.Name, Namespace: "default"})

			// Allow time for the handler to process the update
			time.Sleep(1 * time.Second)

			// Retrieve the updated PolicyRecommendation
			updatedPolicyRecommendation := &ottoscaleriov1alpha1.PolicyRecommendation{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: policyRecommendation.Name, Namespace: "default"},
				updatedPolicyRecommendation)
			Expect(err).ToNot(HaveOccurred())

			// Check if the QueuedForExecution field was updated
			Expect(updatedPolicyRecommendation.Spec.QueuedForExecution).Should(BeTrue())

			// Clean up
			err = k8sClient.Delete(ctx, policyRecommendation)
		})
	})

	Context("For QueueAllForExecution", func() {
		It("should update PolicyRecommendation's QueuedForExecution field", func() {
			// Create 2 new PolicyRecommendation
			policyRecommendation1 := &ottoscaleriov1alpha1.PolicyRecommendation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy-recommendation-1",
					Namespace: "default",
				},
				Spec: ottoscaleriov1alpha1.PolicyRecommendationSpec{
					QueuedForExecution: false,
				},
			}
			err := k8sClient.Create(ctx, policyRecommendation1)
			Expect(err).ToNot(HaveOccurred())

			policyRecommendation2 := &ottoscaleriov1alpha1.PolicyRecommendation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy-recommendation-2",
					Namespace: "default",
				},
				Spec: ottoscaleriov1alpha1.PolicyRecommendationSpec{
					QueuedForExecution: false,
				},
			}
			err = k8sClient.Create(ctx, policyRecommendation2)
			Expect(err).ToNot(HaveOccurred())

			// Start the handler
			handler.Start()

			// Queue the PolicyRecommendation for execution
			handler.QueueAllForExecution()

			// Allow time for the handler to process the update
			time.Sleep(1 * time.Second)

			// Retrieve the updated PolicyRecommendation
			recommendations := ottoscaleriov1alpha1.PolicyRecommendationList{}
			err = k8sClient.List(ctx, &recommendations)
			Expect(err).ToNot(HaveOccurred())

			// Check if the QueuedForExecution field was updated
			for _, reco := range recommendations.Items {
				Expect(reco.Spec.QueuedForExecution).Should(BeTrue())

			}

			// Clean up
			err = k8sClient.Delete(ctx, policyRecommendation1)
			err = k8sClient.Delete(ctx, policyRecommendation2)
		})
	})

})
