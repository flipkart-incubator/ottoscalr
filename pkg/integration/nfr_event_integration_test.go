package integration

import (
	"fmt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

var (
	nfrEventIntegration *NFREventDataFetcher
)

var _ = Describe("GetDesiredEvents", func() {

	var configmap corev1.ConfigMap
	var namespace corev1.Namespace

	BeforeEach(func() {
		configmap = corev1.ConfigMap{
			TypeMeta:   metav1.TypeMeta{Kind: "ConfigMap", APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{Name: "nfr-data-config", Namespace: "ottoscalr"},
			Data: map[string]string{
				"430cda1d-84d7-4022-8dd7-2e951e837bca": fmt.Sprintf(`{"nfrEventKey":"430cda1d-84d7-4022-8dd7-2e951e837bca","startDateTime":"2023-06-07T14:00:00","endDateTime":"2023-06-07T23:59:00","notes":""}`),
				"7f8b9c84-3533-4c64-9598-008db4c67974": fmt.Sprintf(`{"nfrEventKey":"7f8b9c84-3533-4c64-9598-008db4c67974","startDateTime":"2023-06-08T14:00:00","endDateTime":"2023-06-08T23:59:00","notes":""}`),
				"cf6edadc-e2fe-4d8a-92d1-de3c57053994": fmt.Sprintf(`{"nfrEventKey":"cf6edadc-e2fe-4d8a-92d1-de3c57053994","startDateTime":"2023-06-09T14:00:00","endDateTime":"2023-06-09T23:59:00","notes":""}`),
			},
		}

		namespace = corev1.Namespace{
			TypeMeta:   metav1.TypeMeta{Kind: "Namespace", APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{Name: "ottoscalr"},
		}
		Expect(k8sClient.Create(ctx, &namespace)).Should(Succeed())
		time.Sleep(2 * time.Second)
		Expect(k8sClient.Create(ctx, &configmap)).Should(Succeed())
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(ctx, &configmap)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, &namespace)).Should(Succeed())
	})
	It("Should return the nfr outlier intervals by fetching the event metadata and parsing it", func() {
		time1 := time.Now().Add(-20 * time.Minute)
		time2 := time.Now().Add(-14 * time.Minute)
		nfrEventIntegration, _ = NewNFREventDataFetcher(k8sClient, "ottoscalr")
		events, err := nfrEventIntegration.GetDesiredEvents(time1, time2)
		Expect(err).NotTo(HaveOccurred())
		fmt.Fprintf(GinkgoWriter, "Events: %v\n", events)
		Expect(len(events)).To(Equal(3))

	})
})
