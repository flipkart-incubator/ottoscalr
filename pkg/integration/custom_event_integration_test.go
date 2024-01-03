package integration

import (
	"fmt"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	customEventIntegration *CustomEventDataFetcher
	logger                 logr.Logger
)

var _ = Describe("GetDesiredEvents", func() {

	var configmap corev1.ConfigMap
	var namespace corev1.Namespace

	BeforeEach(func() {
		configmap = corev1.ConfigMap{
			TypeMeta:   metav1.TypeMeta{Kind: "ConfigMap", APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{Name: "custom-event-data-config", Namespace: "ottoscalr"},
			Data: map[string]string{
				"430cda1d-84d7-4022-8dd7-2e951e837bca": fmt.Sprintf(`{"eventId":"430cda1d-84d7-4022-8dd7-2e951e837bca","eventName":"event1","startTime":"2023-06-07 14:00","endTime":"2023-06-07 23:59"}`),
				"7f8b9c84-3533-4c64-9598-008db4c67974": fmt.Sprintf(`{"eventId":"7f8b9c84-3533-4c64-9598-008db4c67974","eventName":"event2","startTime":"2023-06-08 14:00","endTime":"2023-06-08 23:59"}`),
				"cf6edadc-e2fe-4d8a-92d1-de3c57053994": fmt.Sprintf(`{"eventId":"cf6edadc-e2fe-4d8a-92d1-de3c57053994","eventName":"event3","startTime":"2023-06-09 14:00","endTime":"2023-06-09 23:59"}`),
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
	It("Should return the custom outlier intervals by fetching the event metadata and parsing it", func() {
		time1 := time.Now().Add(-20 * time.Minute)
		time2 := time.Now().Add(-14 * time.Minute)
		customEventIntegration, _ = NewCustomEventDataFetcher(k8sClient, "ottoscalr", "custom-event-data-config", logger)
		events, err := customEventIntegration.GetDesiredEvents(time1, time2)
		Expect(err).NotTo(HaveOccurred())
		fmt.Fprintf(GinkgoWriter, "Events: %v\n", events)
		Expect(len(events)).To(Equal(3))
	})
})
