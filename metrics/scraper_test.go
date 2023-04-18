package metrics

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PrometheusScraper", func() {

	BeforeEach(func() {

	})

	AfterEach(func() {

	})

	Context("when querying average CPU utilization by workload", func() {
		It("should return correct data points if the query is valid", func() {

			cpuUsageMetric.WithLabelValues("test-ns", "test-pod-1", "test-node-1", "test-container-1").Set(13.2)
			kubePodOwnerMetric.WithLabelValues("test-ns", "test-pod-1", "test-workload", "deployment").Set(1)

			time.Sleep(10 * time.Second)

			dataPoints, err := scraper.GetAverageCPUUtilizationByWorkload("test-ns",
				"test-workload", time.Now().Add(-15*time.Minute), time.Now())
			Expect(err).NotTo(HaveOccurred())

			Expect(dataPoints).ToNot(BeEmpty())
		})
	})

})
