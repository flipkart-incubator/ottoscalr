package metrics

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PrometheusScraper", func() {

	Context("when querying GetAverageCPUUtilizationByWorkload", func() {
		It("should return correct data points", func() {
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-1", "test-node-1", "test-container-1").Set(4)
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-2", "test-node-2", "test-container-1").Set(3)
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-3", "test-node-2", "test-container-1").Set(5)
			cpuUsageMetric.WithLabelValues("test-ns-2", "test-pod-4", "test-node-4", "test-container-1").Set(20)

			kubePodOwnerMetric.WithLabelValues("test-ns-1", "test-pod-1", "test-workload-1", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("test-ns-1", "test-pod-2", "test-workload-1", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("test-ns-1", "test-pod-3", "test-workload-2", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("test-ns-2", "test-pod-4", "test-workload-3", "deployment").Set(1)
			//wait for the metric to be scraped - scraping interval is 1s
			time.Sleep(2 * time.Second)

			//above data points should be outside the query range.
			start := time.Now()

			kubePodOwnerMetric.WithLabelValues("test-ns-1", "test-pod-1", "test-workload-1", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("test-ns-1", "test-pod-2", "test-workload-1", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("test-ns-1", "test-pod-3", "test-workload-2", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("test-ns-2", "test-pod-4", "test-workload-3", "deployment").Set(1)

			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-1", "test-node-1", "test-container-1").Set(12)
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-2", "test-node-2", "test-container-1").Set(14)
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-3", "test-node-2", "test-container-1").Set(3)
			cpuUsageMetric.WithLabelValues("test-ns-2", "test-pod-4", "test-node-4", "test-container-1").Set(16)

			//wait for the metric to be scraped - scraping interval is 1s
			time.Sleep(2 * time.Second)

			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-1", "test-node-1", "test-container-1").Set(5)
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-2", "test-node-2", "test-container-1").Set(4)
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-3", "test-node-2", "test-container-1").Set(12)
			cpuUsageMetric.WithLabelValues("test-ns-2", "test-pod-4", "test-node-4", "test-container-1").Set(15)

			//wait for the metric to be scraped - scraping interval is 1s
			time.Sleep(2 * time.Second)

			// data points after this should be outside the query range
			end := time.Now()

			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-1", "test-node-1", "test-container-1").Set(23)
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-2", "test-node-2", "test-container-1").Set(12)
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-3", "test-node-2", "test-container-1").Set(12)
			cpuUsageMetric.WithLabelValues("test-ns-2", "test-pod-4", "test-node-4", "test-container-1").Set(15)

			//wait for the metric to be scraped - scraping interval is 1s
			time.Sleep(2 * time.Second)

			dataPoints, err := scraper.GetAverageCPUUtilizationByWorkload("test-ns-1",
				"test-workload-1", start, end, time.Second)
			Expect(err).NotTo(HaveOccurred())
			Expect(dataPoints).ToNot(BeEmpty())

			//since metrics could have been scraped multiple times, we just check the first and last value
			Expect(len(dataPoints) >= 2).To(BeTrue())
			Expect(dataPoints[0]).To(Equal(26.0))
			Expect(dataPoints[len(dataPoints)-1]).To(Equal(9.0))
		})
	})

	Context("when querying GetCPUUtilizationBreachDataPoints", func() {
		It("should return correct data points when workload is a deployment", func() {
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-1", "test-node-1", "test-container-1").Set(4)
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-2", "test-node-2", "test-container-1").Set(3)
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-3", "test-node-2", "test-container-1").Set(5)
			cpuUsageMetric.WithLabelValues("test-ns-2", "test-pod-4", "test-node-4", "test-container-1").Set(3)

			kubePodOwnerMetric.WithLabelValues("test-ns-1", "test-pod-1", "dep-1", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("test-ns-1", "test-pod-2", "dep-1", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("test-ns-1", "test-pod-3", "dep-2", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("test-ns-2", "test-pod-4", "dep-3", "deployment").Set(1)

			resourceLimitMetric.WithLabelValues("test-ns-1", "test-pod-1", "test-node-1", "test-container-1").Set(5)
			resourceLimitMetric.WithLabelValues("test-ns-1", "test-pod-1", "test-node-1", "test-container-1").Set(5)
			resourceLimitMetric.WithLabelValues("test-ns-1", "test-pod-1", "test-node-1", "test-container-1").Set(5)
			resourceLimitMetric.WithLabelValues("test-ns-1", "test-pod-1", "test-node-1", "test-container-1").Set(5)

			readyReplicasMetric.WithLabelValues("test-ns-1", "rs-1").Set(1)
			readyReplicasMetric.WithLabelValues("test-ns-1", "rs-2").Set(1)
			readyReplicasMetric.WithLabelValues("test-ns-1", "rs-3").Set(1)
			readyReplicasMetric.WithLabelValues("test-ns-2", "rs-4").Set(1)

			replicaSetOwnerMetric.WithLabelValues("test-ns-1", "deployment", "dep-1", "rs-1").Set(1)
			replicaSetOwnerMetric.WithLabelValues("test-ns-1", "deployment", "dep-1", "rs-2").Set(1)
			replicaSetOwnerMetric.WithLabelValues("test-ns-1", "deployment", "dep-2", "rs-3").Set(1)
			replicaSetOwnerMetric.WithLabelValues("test-ns-2", "deployment", "dep-1", "rs-3").Set(1)

			hpaMaxReplicasMetric.WithLabelValues("test-ns-1", "hpa1").Set(3)
			hpaMaxReplicasMetric.WithLabelValues("test-ns-2", "hpa2").Set(3)

			hpaOwnerInfoMetric.WithLabelValues("test-ns-1", "hpa1", "deployment", "dep-1").Set(1)
			hpaOwnerInfoMetric.WithLabelValues("test-ns-1", "hpa2", "deployment", "dep-2").Set(1)

			//wait for the metric to be scraped - scraping interval is 1s
			time.Sleep(2 * time.Second)

			//above data points should be outside the query range.
			start := time.Now()

			//This data point should be excluded as there are only 2 pods for dep-1. Utilization is 70%

			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-1", "test-node-1", "test-container-1").Set(4)
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-2", "test-node-2", "test-container-1").Set(3)
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-3", "test-node-2", "test-container-1").Set(5)
			cpuUsageMetric.WithLabelValues("test-ns-2", "test-pod-4", "test-node-4", "test-container-1").Set(3)

			kubePodOwnerMetric.WithLabelValues("test-ns-1", "test-pod-1", "dep-1", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("test-ns-1", "test-pod-2", "dep-1", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("test-ns-1", "test-pod-3", "dep-2", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("test-ns-2", "test-pod-4", "dep-3", "deployment").Set(1)

			resourceLimitMetric.WithLabelValues("test-ns-1", "test-pod-1", "test-node-1", "test-container-1").Set(5)
			resourceLimitMetric.WithLabelValues("test-ns-1", "test-pod-2", "test-node-2", "test-container-1").Set(5)
			resourceLimitMetric.WithLabelValues("test-ns-1", "test-pod-3", "test-node-2", "test-container-1").Set(5)
			resourceLimitMetric.WithLabelValues("test-ns-2", "test-pod-4", "test-node-4", "test-container-1").Set(5)

			readyReplicasMetric.WithLabelValues("test-ns-1", "rs-1").Set(1)
			readyReplicasMetric.WithLabelValues("test-ns-1", "rs-2").Set(1)
			readyReplicasMetric.WithLabelValues("test-ns-1", "rs-3").Set(1)
			readyReplicasMetric.WithLabelValues("test-ns-2", "rs-4").Set(1)

			replicaSetOwnerMetric.WithLabelValues("test-ns-1", "deployment", "dep-1", "rs-1").Set(1)
			replicaSetOwnerMetric.WithLabelValues("test-ns-1", "deployment", "dep-1", "rs-2").Set(1)
			replicaSetOwnerMetric.WithLabelValues("test-ns-1", "deployment", "dep-2", "rs-3").Set(1)
			replicaSetOwnerMetric.WithLabelValues("test-ns-2", "deployment", "dep-1", "rs-3").Set(1)

			hpaMaxReplicasMetric.WithLabelValues("test-ns-1", "hpa1").Set(3)
			hpaMaxReplicasMetric.WithLabelValues("test-ns-2", "hpa2").Set(3)

			hpaOwnerInfoMetric.WithLabelValues("test-ns-1", "hpa1", "deployment", "dep-1").Set(1)
			hpaOwnerInfoMetric.WithLabelValues("test-ns-1", "hpa2", "deployment", "dep-2").Set(1)

			//wait for the metric to be scraped - scraping interval is 1s
			time.Sleep(2 * time.Second)

			// this data point will be excluded as utilization(80%) for dep-1 is below threshold of 85%
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-1", "test-node-1", "test-container-1").Set(4)
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-2", "test-node-2", "test-container-1").Set(4)
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-3", "test-node-2", "test-container-1").Set(12)
			cpuUsageMetric.WithLabelValues("test-ns-2", "test-pod-4", "test-node-4", "test-container-1").Set(15)

			//wait for the metric to be scraped - scraping interval is 1s
			time.Sleep(2 * time.Second)

			// this data point will be excluded as no of ready pods < maxReplicas(3)
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-1", "test-node-1", "test-container-1").Set(5)
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-2", "test-node-2", "test-container-1").Set(4)
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-3", "test-node-2", "test-container-1").Set(12)
			cpuUsageMetric.WithLabelValues("test-ns-2", "test-pod-4", "test-node-4", "test-container-1").Set(15)

			//wait for the metric to be scraped - scraping interval is 1s
			time.Sleep(2 * time.Second)

			// this data point should be included - utilization of 100% and ready replicas(1+2) = maxReplicas(3)
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-1", "test-node-1", "test-container-1").Set(5)
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-2", "test-node-2", "test-container-1").Set(5)
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-5", "test-node-2", "test-container-1").Set(5)
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-3", "test-node-2", "test-container-1").Set(12)
			cpuUsageMetric.WithLabelValues("test-ns-2", "test-pod-4", "test-node-4", "test-container-1").Set(15)

			readyReplicasMetric.WithLabelValues("test-ns-1", "rs-1").Set(1)
			readyReplicasMetric.WithLabelValues("test-ns-1", "rs-2").Set(2)
			readyReplicasMetric.WithLabelValues("test-ns-1", "rs-3").Set(1)
			readyReplicasMetric.WithLabelValues("test-ns-2", "rs-4").Set(1)

			resourceLimitMetric.WithLabelValues("test-ns-1", "test-pod-1", "test-node-1", "test-container-1").Set(5)
			resourceLimitMetric.WithLabelValues("test-ns-1", "test-pod-2", "test-node-2", "test-container-1").Set(5)
			resourceLimitMetric.WithLabelValues("test-ns-1", "test-pod-3", "test-node-2", "test-container-1").Set(5)
			resourceLimitMetric.WithLabelValues("test-ns-2", "test-pod-4", "test-node-4", "test-container-1").Set(5)
			resourceLimitMetric.WithLabelValues("test-ns-1", "test-pod-5", "test-node-2", "test-container-1").Set(5)

			//wait for the metric to be scraped - scraping interval is 1s
			time.Sleep(2 * time.Second)

			// data points after this should be outside the query range
			end := time.Now()

			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-1", "test-node-1", "test-container-1").Set(10)
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-2", "test-node-2", "test-container-1").Set(10)
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-5", "test-node-2", "test-container-1").Set(10)
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-3", "test-node-2", "test-container-1").Set(12)
			cpuUsageMetric.WithLabelValues("test-ns-2", "test-pod-4", "test-node-4", "test-container-1").Set(15)

			//wait for the metric to be scraped - scraping interval is 1s
			time.Sleep(2 * time.Second)

			dataPoints, err := scraper.GetCPUUtilizationBreachDataPoints("test-ns-1",
				"deployment",
				"dep-1",
				0.85, start,
				end,
				time.Second)
			Expect(err).NotTo(HaveOccurred())
			Expect(dataPoints).ToNot(BeEmpty())

			//since metrics could have been scraped multiple times, we just check the first and last value
			Expect(len(dataPoints) >= 1).To(BeTrue())

			fmt.Println(dataPoints)

			for _, dataPoint := range dataPoints {
				Expect(dataPoint).To(Equal(1.0))
			}
		})
	})

})
