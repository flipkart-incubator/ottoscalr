package metrics

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PrometheusScraper", func() {

	Context("when querying average CPU utilization by workload", func() {
		It("should return correct data points if the query is valid", func() {

			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-1", "test-node-1", "test-container-1").Set(4)
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-2", "test-node-2", "test-container-1").Set(3)
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-3", "test-node-2", "test-container-1").Set(5)
			cpuUsageMetric.WithLabelValues("test-ns-2", "test-pod-4", "test-node-4", "test-container-1").Set(20)

			kubePodOwnerMetric.WithLabelValues("test-ns-1", "test-pod-1", "test-workload-1", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("test-ns-1", "test-pod-2", "test-workload-1", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("test-ns-1", "test-pod-3", "test-workload-2", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("test-ns-2", "test-pod-4", "test-workload-3", "deployment").Set(1)

			//wait for the metric to be scraped - scraping interval is 1s
			time.Sleep(10 * time.Second)

			//wait for the metric to be scraped - scraping interval is 1s
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-1", "test-node-1", "test-container-1").Set(5)
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-2", "test-node-2", "test-container-1").Set(4)
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-3", "test-node-2", "test-container-1").Set(12)
			cpuUsageMetric.WithLabelValues("test-ns-2", "test-pod-4", "test-node-4", "test-container-1").Set(15)
			time.Sleep(10 * time.Second)

			dataPoints, err := scraper.GetAverageCPUUtilizationByWorkload("test-ns-1",
				"test-workload-1", time.Now().Add(-1*time.Minute), time.Now(), 100*time.Millisecond)
			Expect(err).NotTo(HaveOccurred())
			Expect(dataPoints).ToNot(BeEmpty())

			//since metrics could have been scraped multiple times, we just check the first and last value
			Expect(len(dataPoints) >= 2).To(BeTrue())
			Expect(dataPoints[0]).To(Equal(7.0))
			Expect(dataPoints[len(dataPoints)-1]).To(Equal(9.0))

		})
	})

})
