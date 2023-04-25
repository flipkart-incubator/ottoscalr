package metrics

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PrometheusScraper", func() {

	Context("when querying GetAverageCPUUtilizationByWorkload", func() {
		It("should return correct data points", func() {

			By("creating a metric before queryRange window")
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

			start := time.Now()

			By("creating first metric inside queryRange window")

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

			By("creating second metric inside queryRange window")

			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-1", "test-node-1", "test-container-1").Set(5)
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-2", "test-node-2", "test-container-1").Set(4)
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-3", "test-node-2", "test-container-1").Set(12)
			cpuUsageMetric.WithLabelValues("test-ns-2", "test-pod-4", "test-node-4", "test-container-1").Set(15)

			//wait for the metric to be scraped - scraping interval is 1s
			time.Sleep(2 * time.Second)

			// data points after this should be outside the query range
			end := time.Now()

			By("creating metric after queryRange window")

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
			cpuUsageMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-1", "dep-test-node-1", "dep-test-container-1").Set(14)
			cpuUsageMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-2", "dep-test-node-2", "dep-test-container-1").Set(3)
			cpuUsageMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-3", "dep-test-node-2", "dep-test-container-1").Set(5)
			cpuUsageMetric.WithLabelValues("dep-test-ns-2", "dep-test-pod-4", "dep-test-node-4", "dep-test-container-1").Set(3)

			kubePodOwnerMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-1", "dep-1", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-2", "dep-1", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-3", "dep-2", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("dep-test-ns-2", "dep-test-pod-4", "dep-3", "deployment").Set(1)

			resourceLimitMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-1", "dep-test-node-1", "dep-test-container-1").Set(5)
			resourceLimitMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-2", "dep-test-node-2", "dep-test-container-1").Set(5)
			resourceLimitMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-3", "dep-test-node-2", "dep-test-container-1").Set(5)
			resourceLimitMetric.WithLabelValues("dep-test-ns-2", "dep-test-pod-4", "dep-test-node-4", "dep-test-container-1").Set(5)

			readyReplicasMetric.WithLabelValues("dep-test-ns-1", "dep-rs-1").Set(1)
			readyReplicasMetric.WithLabelValues("dep-test-ns-1", "dep-rs-2").Set(1)
			readyReplicasMetric.WithLabelValues("dep-test-ns-1", "dep-rs-3").Set(1)
			readyReplicasMetric.WithLabelValues("dep-test-ns-2", "dep-rs-4").Set(1)

			replicaSetOwnerMetric.WithLabelValues("dep-test-ns-1", "deployment", "dep-1", "dep-rs-1").Set(1)
			replicaSetOwnerMetric.WithLabelValues("dep-test-ns-1", "deployment", "dep-1", "dep-rs-2").Set(1)
			replicaSetOwnerMetric.WithLabelValues("dep-test-ns-1", "deployment", "dep-2", "dep-rs-3").Set(1)
			replicaSetOwnerMetric.WithLabelValues("dep-test-ns-2", "deployment", "dep-1", "dep-rs-3").Set(1)

			hpaMaxReplicasMetric.WithLabelValues("dep-test-ns-1", "dep-hpa1").Set(3)
			hpaMaxReplicasMetric.WithLabelValues("dep-test-ns-2", "dep-hpa2").Set(3)

			hpaOwnerInfoMetric.WithLabelValues("dep-test-ns-1", "dep-hpa1", "deployment", "dep-1").Set(1)
			hpaOwnerInfoMetric.WithLabelValues("dep-test-ns-1", "dep-hpa2", "deployment", "dep-2").Set(1)

			//wait for the metric to be scraped - scraping interval is 1s
			time.Sleep(2 * time.Second)

			//above data points should be outside the query range.
			start := time.Now()

			//This data point should be excluded as there are only 2 pods for dep-1. Utilization is 70%

			cpuUsageMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-1", "dep-test-node-1", "dep-test-container-1").Set(4)
			cpuUsageMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-2", "dep-test-node-2", "dep-test-container-1").Set(3)
			cpuUsageMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-3", "dep-test-node-2", "dep-test-container-1").Set(5)
			cpuUsageMetric.WithLabelValues("dep-test-ns-2", "dep-test-pod-4", "dep-test-node-4", "dep-test-container-1").Set(3)

			kubePodOwnerMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-1", "dep-1", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-2", "dep-1", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-3", "dep-2", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("dep-test-ns-2", "dep-test-pod-4", "dep-3", "deployment").Set(1)

			resourceLimitMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-1", "dep-test-node-1", "dep-test-container-1").Set(5)
			resourceLimitMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-2", "dep-test-node-2", "dep-test-container-1").Set(5)
			resourceLimitMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-3", "dep-test-node-2", "dep-test-container-1").Set(5)
			resourceLimitMetric.WithLabelValues("dep-test-ns-2", "dep-test-pod-4", "dep-test-node-4", "dep-test-container-1").Set(5)

			readyReplicasMetric.WithLabelValues("dep-test-ns-1", "dep-rs-1").Set(1)
			readyReplicasMetric.WithLabelValues("dep-test-ns-1", "dep-rs-2").Set(1)
			readyReplicasMetric.WithLabelValues("dep-test-ns-1", "dep-rs-3").Set(1)
			readyReplicasMetric.WithLabelValues("dep-test-ns-2", "dep-rs-4").Set(1)

			replicaSetOwnerMetric.WithLabelValues("dep-test-ns-1", "deployment", "dep-1", "dep-rs-1").Set(1)
			replicaSetOwnerMetric.WithLabelValues("dep-test-ns-1", "deployment", "dep-1", "dep-rs-2").Set(1)
			replicaSetOwnerMetric.WithLabelValues("dep-test-ns-1", "deployment", "dep-2", "dep-rs-3").Set(1)
			replicaSetOwnerMetric.WithLabelValues("dep-test-ns-2", "deployment", "dep-1", "dep-rs-3").Set(1)

			hpaMaxReplicasMetric.WithLabelValues("dep-test-ns-1", "dep-hpa1").Set(3)
			hpaMaxReplicasMetric.WithLabelValues("dep-test-ns-2", "dep-hpa2").Set(3)

			hpaOwnerInfoMetric.WithLabelValues("dep-test-ns-1", "dep-hpa1", "deployment", "dep-1").Set(1)
			hpaOwnerInfoMetric.WithLabelValues("dep-test-ns-1", "dep-hpa2", "deployment", "dep-2").Set(1)

			//wait for the metric to be scraped - scraping interval is 1s
			time.Sleep(2 * time.Second)

			// this data point will be excluded as utilization(80%) for dep-1 is below threshold of 85%
			cpuUsageMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-1", "dep-test-node-1", "dep-test-container-1").Set(4)
			cpuUsageMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-2", "dep-test-node-2", "dep-test-container-1").Set(4)
			cpuUsageMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-3", "dep-test-node-2", "dep-test-container-1").Set(12)
			cpuUsageMetric.WithLabelValues("dep-test-ns-2", "dep-test-pod-4", "dep-test-node-4", "dep-test-container-1").Set(15)

			//wait for the metric to be scraped - scraping interval is 1s
			time.Sleep(2 * time.Second)

			// this data point will be excluded as no of ready pods < maxReplicas(3)
			cpuUsageMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-1", "dep-test-node-1", "dep-test-container-1").Set(5)
			cpuUsageMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-2", "dep-test-node-2", "dep-test-container-1").Set(4)
			cpuUsageMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-3", "dep-test-node-2", "dep-test-container-1").Set(12)
			cpuUsageMetric.WithLabelValues("dep-test-ns-2", "dep-test-pod-4", "dep-test-node-4", "dep-test-container-1").Set(15)

			//wait for the metric to be scraped - scraping interval is 1s
			time.Sleep(2 * time.Second)

			// this data point should be included - utilization of 100% and ready replicas(1+2) = maxReplicas(3)
			cpuUsageMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-1", "dep-test-node-1", "dep-test-container-1").Set(5)
			cpuUsageMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-2", "dep-test-node-2", "dep-test-container-1").Set(5)
			cpuUsageMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-5", "dep-test-node-2", "dep-test-container-1").Set(5)
			cpuUsageMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-3", "dep-test-node-2", "dep-test-container-1").Set(12)
			cpuUsageMetric.WithLabelValues("dep-test-ns-2", "dep-test-pod-4", "dep-test-node-4", "dep-test-container-1").Set(15)

			readyReplicasMetric.WithLabelValues("dep-test-ns-1", "dep-rs-1").Set(1)
			readyReplicasMetric.WithLabelValues("dep-test-ns-1", "dep-rs-2").Set(2)
			readyReplicasMetric.WithLabelValues("dep-test-ns-1", "dep-rs-3").Set(1)
			readyReplicasMetric.WithLabelValues("dep-test-ns-2", "dep-rs-4").Set(1)

			resourceLimitMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-1", "dep-test-node-1", "dep-test-container-1").Set(5)
			resourceLimitMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-2", "dep-test-node-2", "dep-test-container-1").Set(5)
			resourceLimitMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-3", "dep-test-node-2", "dep-test-container-1").Set(5)
			resourceLimitMetric.WithLabelValues("dep-test-ns-2", "dep-test-pod-4", "dep-test-node-4", "dep-test-container-1").Set(5)
			resourceLimitMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-5", "dep-test-node-2", "dep-test-container-1").Set(5)

			//wait for the metric to be scraped - scraping interval is 1s
			time.Sleep(2 * time.Second)

			// data points after this should be outside the query range
			end := time.Now()

			cpuUsageMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-1", "dep-test-node-1", "dep-test-container-1").Set(10)
			cpuUsageMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-2", "dep-test-node-2", "dep-test-container-1").Set(10)
			cpuUsageMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-5", "dep-test-node-2", "dep-test-container-1").Set(10)
			cpuUsageMetric.WithLabelValues("dep-test-ns-1", "dep-test-pod-3", "dep-test-node-2", "dep-test-container-1").Set(12)
			cpuUsageMetric.WithLabelValues("dep-test-ns-2", "dep-test-pod-4", "dep-test-node-4", "dep-test-container-1").Set(15)

			//wait for the metric to be scraped - scraping interval is 1s
			time.Sleep(2 * time.Second)

			dataPoints, err := scraper.GetCPUUtilizationBreachDataPoints("dep-test-ns-1",
				"deployment",
				"dep-1",
				0.85, start,
				end,
				time.Second)
			Expect(err).NotTo(HaveOccurred())
			Expect(dataPoints).ToNot(BeEmpty())

			//since metrics could have been scraped multiple times, we just check the first and last value
			Expect(len(dataPoints) >= 1).To(BeTrue())

			for _, dataPoint := range dataPoints {
				Expect(dataPoint).To(Equal(1.0))
			}
		})

		It("should return correct data points when workload is a Rollout", func() {
			cpuUsageMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-1", "ro-test-node-1", "ro-test-container-1").Set(14)
			cpuUsageMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-2", "ro-test-node-2", "ro-test-container-1").Set(3)
			cpuUsageMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-3", "ro-test-node-2", "ro-test-container-1").Set(5)
			cpuUsageMetric.WithLabelValues("ro-test-ns-2", "ro-test-pod-4", "ro-test-node-4", "ro-test-container-1").Set(3)

			kubePodOwnerMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-1", "ro-1", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-2", "ro-1", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-3", "ro-2", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("ro-test-ns-2", "ro-test-pod-4", "ro-3", "deployment").Set(1)

			resourceLimitMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-1", "ro-test-node-1", "ro-test-container-1").Set(5)
			resourceLimitMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-2", "ro-test-node-2", "ro-test-container-1").Set(5)
			resourceLimitMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-3", "ro-test-node-2", "ro-test-container-1").Set(5)
			resourceLimitMetric.WithLabelValues("ro-test-ns-2", "ro-test-pod-4", "ro-test-node-4", "ro-test-container-1").Set(5)

			readyReplicasMetric.WithLabelValues("ro-test-ns-1", "ro-rs-1").Set(1)
			readyReplicasMetric.WithLabelValues("ro-test-ns-1", "ro-rs-2").Set(1)
			readyReplicasMetric.WithLabelValues("ro-test-ns-1", "ro-rs-3").Set(1)
			readyReplicasMetric.WithLabelValues("ro-test-ns-2", "ro-rs-4").Set(1)

			replicaSetOwnerMetric.WithLabelValues("ro-test-ns-1", "Rollout", "ro-1", "ro-rs-1").Set(1)
			replicaSetOwnerMetric.WithLabelValues("ro-test-ns-1", "Rollout", "ro-1", "ro-rs-2").Set(1)
			replicaSetOwnerMetric.WithLabelValues("ro-test-ns-1", "Rollout", "ro-2", "ro-rs-3").Set(1)
			replicaSetOwnerMetric.WithLabelValues("ro-test-ns-2", "Rollout", "ro-1", "ro-rs-3").Set(1)

			hpaMaxReplicasMetric.WithLabelValues("ro-test-ns-1", "ro-hpa1").Set(3)
			hpaMaxReplicasMetric.WithLabelValues("ro-test-ns-2", "ro-hpa2").Set(3)

			hpaOwnerInfoMetric.WithLabelValues("ro-test-ns-1", "ro-hpa1", "Rollout", "ro-1").Set(1)
			hpaOwnerInfoMetric.WithLabelValues("ro-test-ns-1", "ro-hpa2", "Rollout", "ro-2").Set(1)

			//wait for the metric to be scraped - scraping interval is 1s
			time.Sleep(2 * time.Second)

			//above data points should be outside the query range.
			start := time.Now()

			//This data point should be excluded as there are only 2 pods for ro-1. Utilization is 70%

			cpuUsageMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-1", "ro-test-node-1", "ro-test-container-1").Set(4)
			cpuUsageMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-2", "ro-test-node-2", "ro-test-container-1").Set(3)
			cpuUsageMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-3", "ro-test-node-2", "ro-test-container-1").Set(5)
			cpuUsageMetric.WithLabelValues("ro-test-ns-2", "ro-test-pod-4", "ro-test-node-4", "ro-test-container-1").Set(3)

			kubePodOwnerMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-1", "ro-1", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-2", "ro-1", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-3", "ro-2", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("ro-test-ns-2", "ro-test-pod-4", "ro-3", "deployment").Set(1)

			resourceLimitMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-1", "ro-test-node-1", "ro-test-container-1").Set(5)
			resourceLimitMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-2", "ro-test-node-2", "ro-test-container-1").Set(5)
			resourceLimitMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-3", "ro-test-node-2", "ro-test-container-1").Set(5)
			resourceLimitMetric.WithLabelValues("ro-test-ns-2", "ro-test-pod-4", "ro-test-node-4", "ro-test-container-1").Set(5)

			readyReplicasMetric.WithLabelValues("ro-test-ns-1", "ro-rs-1").Set(1)
			readyReplicasMetric.WithLabelValues("ro-test-ns-1", "ro-rs-2").Set(1)
			readyReplicasMetric.WithLabelValues("ro-test-ns-1", "ro-rs-3").Set(1)
			readyReplicasMetric.WithLabelValues("ro-test-ns-2", "ro-rs-4").Set(1)

			replicaSetOwnerMetric.WithLabelValues("ro-test-ns-1", "Rollout", "ro-1", "ro-rs-1").Set(1)
			replicaSetOwnerMetric.WithLabelValues("ro-test-ns-1", "Rollout", "ro-1", "ro-rs-2").Set(1)
			replicaSetOwnerMetric.WithLabelValues("ro-test-ns-1", "Rollout", "ro-2", "ro-rs-3").Set(1)
			replicaSetOwnerMetric.WithLabelValues("ro-test-ns-2", "Rollout", "ro-1", "ro-rs-3").Set(1)

			hpaMaxReplicasMetric.WithLabelValues("ro-test-ns-1", "ro-hpa1").Set(3)
			hpaMaxReplicasMetric.WithLabelValues("ro-test-ns-2", "ro-hpa2").Set(3)

			hpaOwnerInfoMetric.WithLabelValues("ro-test-ns-1", "ro-hpa1", "Rollout", "ro-1").Set(1)
			hpaOwnerInfoMetric.WithLabelValues("ro-test-ns-1", "ro-hpa2", "Rollout", "ro-2").Set(1)

			//wait for the metric to be scraped - scraping interval is 1s
			time.Sleep(2 * time.Second)

			// this data point will be excluded as utilization(80%) for ro-1 is below threshold of 85%
			cpuUsageMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-1", "ro-test-node-1", "ro-test-container-1").Set(4)
			cpuUsageMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-2", "ro-test-node-2", "ro-test-container-1").Set(4)
			cpuUsageMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-3", "ro-test-node-2", "ro-test-container-1").Set(12)
			cpuUsageMetric.WithLabelValues("ro-test-ns-2", "ro-test-pod-4", "ro-test-node-4", "ro-test-container-1").Set(15)

			//wait for the metric to be scraped - scraping interval is 1s
			time.Sleep(2 * time.Second)

			// this data point will be excluded as no of ready pods < maxReplicas(3)
			cpuUsageMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-1", "ro-test-node-1", "ro-test-container-1").Set(5)
			cpuUsageMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-2", "ro-test-node-2", "ro-test-container-1").Set(4)
			cpuUsageMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-3", "ro-test-node-2", "ro-test-container-1").Set(12)
			cpuUsageMetric.WithLabelValues("ro-test-ns-2", "ro-test-pod-4", "ro-test-node-4", "ro-test-container-1").Set(15)

			//wait for the metric to be scraped - scraping interval is 1s
			time.Sleep(2 * time.Second)

			// this data point should be included - utilization of 100% and ready replicas(1+2) = maxReplicas(3)
			cpuUsageMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-1", "ro-test-node-1", "ro-test-container-1").Set(5)
			cpuUsageMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-2", "ro-test-node-2", "ro-test-container-1").Set(5)
			cpuUsageMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-5", "ro-test-node-2", "ro-test-container-1").Set(5)
			cpuUsageMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-3", "ro-test-node-2", "ro-test-container-1").Set(12)
			cpuUsageMetric.WithLabelValues("ro-test-ns-2", "ro-test-pod-4", "ro-test-node-4", "ro-test-container-1").Set(15)

			readyReplicasMetric.WithLabelValues("ro-test-ns-1", "ro-rs-1").Set(1)
			readyReplicasMetric.WithLabelValues("ro-test-ns-1", "ro-rs-2").Set(2)
			readyReplicasMetric.WithLabelValues("ro-test-ns-1", "ro-rs-3").Set(1)
			readyReplicasMetric.WithLabelValues("ro-test-ns-2", "ro-rs-4").Set(1)

			resourceLimitMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-1", "ro-test-node-1", "ro-test-container-1").Set(5)
			resourceLimitMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-2", "ro-test-node-2", "ro-test-container-1").Set(5)
			resourceLimitMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-3", "ro-test-node-2", "ro-test-container-1").Set(5)
			resourceLimitMetric.WithLabelValues("ro-test-ns-2", "ro-test-pod-4", "ro-test-node-4", "ro-test-container-1").Set(5)
			resourceLimitMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-5", "ro-test-node-2", "ro-test-container-1").Set(5)

			//wait for the metric to be scraped - scraping interval is 1s
			time.Sleep(2 * time.Second)

			// data points after this should be outside the query range
			end := time.Now()

			cpuUsageMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-1", "ro-test-node-1", "ro-test-container-1").Set(10)
			cpuUsageMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-2", "ro-test-node-2", "ro-test-container-1").Set(10)
			cpuUsageMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-5", "ro-test-node-2", "ro-test-container-1").Set(10)
			cpuUsageMetric.WithLabelValues("ro-test-ns-1", "ro-test-pod-3", "ro-test-node-2", "ro-test-container-1").Set(12)
			cpuUsageMetric.WithLabelValues("ro-test-ns-2", "ro-test-pod-4", "ro-test-node-4", "ro-test-container-1").Set(15)

			//wait for the metric to be scraped - scraping interval is 1s
			time.Sleep(2 * time.Second)

			dataPoints, err := scraper.GetCPUUtilizationBreachDataPoints("ro-test-ns-1",
				"Rollout",
				"ro-1",
				0.85, start,
				end,
				time.Second)
			Expect(err).NotTo(HaveOccurred())
			Expect(dataPoints).ToNot(BeEmpty())

			//since metrics could have been scraped multiple times, we just check the first and last value
			Expect(len(dataPoints) >= 1).To(BeTrue())

			for _, dataPoint := range dataPoints {
				Expect(dataPoint).To(Equal(1.0))
			}
		})
	})

})
