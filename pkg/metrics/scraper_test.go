package metrics

import (
	"context"
	"fmt"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"math"
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
			time.Sleep(5 * time.Second)

			start := time.Now().Add(1 * time.Second)

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
			time.Sleep(5 * time.Second)

			By("creating second metric inside queryRange window")

			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-1", "test-node-1", "test-container-1").Set(5)
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-2", "test-node-2", "test-container-1").Set(4)
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-3", "test-node-2", "test-container-1").Set(12)
			cpuUsageMetric.WithLabelValues("test-ns-2", "test-pod-4", "test-node-4", "test-container-1").Set(15)

			//wait for the metric to be scraped - scraping interval is 1s
			time.Sleep(5 * time.Second)

			// data points after this should be outside the query range
			end := time.Now()

			By("creating metric after queryRange window")

			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-1", "test-node-1", "test-container-1").Set(23)
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-2", "test-node-2", "test-container-1").Set(12)
			cpuUsageMetric.WithLabelValues("test-ns-1", "test-pod-3", "test-node-2", "test-container-1").Set(12)
			cpuUsageMetric.WithLabelValues("test-ns-2", "test-pod-4", "test-node-4", "test-container-1").Set(15)

			//wait for the metric to be scraped - scraping interval is 1s
			time.Sleep(5 * time.Second)

			dataPoints, err := scraper.GetAverageCPUUtilizationByWorkload("test-ns-1",
				"test-workload-1", start, end, time.Second)
			Expect(err).NotTo(HaveOccurred())
			Expect(dataPoints).ToNot(BeEmpty())

			//since metrics could have been scraped multiple times, we just check the first and last value
			Expect(len(dataPoints) >= 2).To(BeTrue())

			Expect(dataPoints[0].Value).To(Equal(26.0))
			Expect(dataPoints[len(dataPoints)-1].Value).To(Equal(9.0))
		})
	})

	Context("when querying GetACLByWorkload", func() {
		It("should return correct ACL", func() {

			By("creating a metric before queryRange window")

			podCreatedTimeMetric.WithLabelValues("test-ns-1", "test-pod-1").Set(45)
			podCreatedTimeMetric.WithLabelValues("test-ns-1", "test-pod-2").Set(55)
			podCreatedTimeMetric.WithLabelValues("test-ns-2", "test-pod-3").Set(65)
			podCreatedTimeMetric.WithLabelValues("test-ns-2", "test-pod-4").Set(75)
			podCreatedTimeMetric.WithLabelValues("test-ns-2", "test-pod-5").Set(80)

			podReadyTimeMetric.WithLabelValues("test-ns-1", "test-pod-1").Set(50)
			podReadyTimeMetric.WithLabelValues("test-ns-1", "test-pod-2").Set(70)
			podReadyTimeMetric.WithLabelValues("test-ns-2", "test-pod-3").Set(80)
			podReadyTimeMetric.WithLabelValues("test-ns-2", "test-pod-4").Set(110)
			podReadyTimeMetric.WithLabelValues("test-ns-2", "test-pod-5").Set(120)

			kubePodOwnerMetric.WithLabelValues("test-ns-1", "test-pod-1", "test-workload-1", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("test-ns-1", "test-pod-2", "test-workload-1", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("test-ns-2", "test-pod-3", "test-workload-3", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("test-ns-2", "test-pod-4", "test-workload-3", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("test-ns-2", "test-pod-5", "test-workload-3", "deployment").Set(1)

			podCreatedTimeMetric1.WithLabelValues("test-ns-1", "test-pod-1").Set(45)
			podCreatedTimeMetric1.WithLabelValues("test-ns-1", "test-pod-2").Set(55)
			podCreatedTimeMetric1.WithLabelValues("test-ns-2", "test-pod-3").Set(65)
			podCreatedTimeMetric1.WithLabelValues("test-ns-2", "test-pod-4").Set(75)
			podCreatedTimeMetric1.WithLabelValues("test-ns-2", "test-pod-5").Set(80)

			podReadyTimeMetric1.WithLabelValues("test-ns-1", "test-pod-1").Set(60)
			podReadyTimeMetric1.WithLabelValues("test-ns-1", "test-pod-2").Set(70)
			podReadyTimeMetric1.WithLabelValues("test-ns-2", "test-pod-3").Set(80)
			podReadyTimeMetric1.WithLabelValues("test-ns-2", "test-pod-4").Set(100)
			podReadyTimeMetric1.WithLabelValues("test-ns-2", "test-pod-5").Set(120)

			//wait for the metric to be scraped - scraping interval is 1s
			time.Sleep(2 * time.Second)

			autoscalingLag1, err := scraper.GetACLByWorkload("test-ns-1", "test-workload-1")
			Expect(err).NotTo(HaveOccurred())
			Expect(autoscalingLag1).To(Equal(45.0 * time.Second))

			autoscalingLag2, err := scraper.GetACLByWorkload("test-ns-2", "test-workload-3")
			Expect(err).NotTo(HaveOccurred())
			Expect(autoscalingLag2).To(Equal(65.0 * time.Second))
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
			for _, datapoint := range dataPoints {
				Expect(datapoint.Value).To(Or(Equal(1.7), Equal(0.9)))
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
			for _, datapoint := range dataPoints {
				Expect(datapoint.Value).To(Or(Equal(1.7), Equal(0.9)))
			}
		})
	})

	Context("when querying GetAverageCPUUtilizationByWorkload with two prometheus instances", func() {
		It("should return correct data points", func() {

			By("creating a metric before queryRange window")
			cpuUsageMetric.WithLabelValues("test-nsp-1", "test-pod-1", "test-node-1", "test-container-1").Set(4)
			cpuUsageMetric.WithLabelValues("test-nsp-1", "test-pod-2", "test-node-2", "test-container-1").Set(3)
			cpuUsageMetric.WithLabelValues("test-nsp-1", "test-pod-3", "test-node-2", "test-container-1").Set(5)
			cpuUsageMetric.WithLabelValues("test-nsp-2", "test-pod-4", "test-node-4", "test-container-1").Set(20)

			cpuUsageMetric1.WithLabelValues("test-nsp-1", "test-pod-1", "test-node-1", "test-container-1").Set(4)
			cpuUsageMetric1.WithLabelValues("test-nsp-1", "test-pod-2", "test-node-2", "test-container-1").Set(3)
			cpuUsageMetric1.WithLabelValues("test-nsp-1", "test-pod-3", "test-node-2", "test-container-1").Set(5)
			cpuUsageMetric1.WithLabelValues("test-nsp-2", "test-pod-4", "test-node-4", "test-container-1").Set(20)

			kubePodOwnerMetric.WithLabelValues("test-nsp-1", "test-pod-1", "test-workload-1", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("test-nsp-1", "test-pod-2", "test-workload-1", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("test-nsp-1", "test-pod-3", "test-workload-2", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("test-nsp-2", "test-pod-4", "test-workload-3", "deployment").Set(1)

			//wait for the metric to be scraped - scraping interval is 1s
			time.Sleep(5 * time.Second)

			start := time.Now().Add(1 * time.Second)

			By("creating first metric inside queryRange window")

			kubePodOwnerMetric.WithLabelValues("test-nsp-1", "test-pod-1", "test-workload-1", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("test-nsp-1", "test-pod-2", "test-workload-1", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("test-nsp-1", "test-pod-3", "test-workload-2", "deployment").Set(1)
			kubePodOwnerMetric.WithLabelValues("test-nsp-2", "test-pod-4", "test-workload-3", "deployment").Set(1)

			cpuUsageMetric.WithLabelValues("test-nsp-1", "test-pod-1", "test-node-1", "test-container-1").Set(12)
			cpuUsageMetric.WithLabelValues("test-nsp-1", "test-pod-2", "test-node-2", "test-container-1").Set(14)
			cpuUsageMetric.WithLabelValues("test-nsp-1", "test-pod-3", "test-node-2", "test-container-1").Set(3)
			cpuUsageMetric.WithLabelValues("test-nsp-2", "test-pod-4", "test-node-4", "test-container-1").Set(16)

			cpuUsageMetric1.WithLabelValues("test-nsp-1", "test-pod-1", "test-node-1", "test-container-1").Set(12)
			cpuUsageMetric1.WithLabelValues("test-nsp-1", "test-pod-2", "test-node-2", "test-container-1").Set(14)
			cpuUsageMetric1.WithLabelValues("test-nsp-1", "test-pod-3", "test-node-2", "test-container-1").Set(3)
			cpuUsageMetric1.WithLabelValues("test-nsp-2", "test-pod-4", "test-node-4", "test-container-1").Set(16)

			//wait for the metric to be scraped - scraping interval is 1s
			time.Sleep(5 * time.Second)

			By("creating second metric inside queryRange window")

			cpuUsageMetric.WithLabelValues("test-nsp-1", "test-pod-1", "test-node-1", "test-container-1").Set(5)
			cpuUsageMetric.WithLabelValues("test-nsp-1", "test-pod-2", "test-node-2", "test-container-1").Set(4)
			cpuUsageMetric.WithLabelValues("test-nsp-1", "test-pod-3", "test-node-2", "test-container-1").Set(12)
			cpuUsageMetric.WithLabelValues("test-nsp-2", "test-pod-4", "test-node-4", "test-container-1").Set(15)

			cpuUsageMetric1.WithLabelValues("test-nsp-1", "test-pod-1", "test-node-1", "test-container-1").Set(5)
			cpuUsageMetric1.WithLabelValues("test-nsp-1", "test-pod-2", "test-node-2", "test-container-1").Set(5)
			cpuUsageMetric1.WithLabelValues("test-nsp-1", "test-pod-3", "test-node-2", "test-container-1").Set(12)
			cpuUsageMetric1.WithLabelValues("test-nsp-2", "test-pod-4", "test-node-4", "test-container-1").Set(15)

			//wait for the metric to be scraped - scraping interval is 1s
			time.Sleep(5 * time.Second)

			// data points after this should be outside the query range
			end := time.Now()

			By("creating metric after queryRange window")

			cpuUsageMetric.WithLabelValues("test-nsp-1", "test-pod-1", "test-node-1", "test-container-1").Set(23)
			cpuUsageMetric.WithLabelValues("test-nsp-1", "test-pod-2", "test-node-2", "test-container-1").Set(12)
			cpuUsageMetric.WithLabelValues("test-nsp-1", "test-pod-3", "test-node-2", "test-container-1").Set(12)
			cpuUsageMetric.WithLabelValues("test-nsp-2", "test-pod-4", "test-node-4", "test-container-1").Set(15)

			//wait for the metric to be scraped - scraping interval is 1s
			time.Sleep(5 * time.Second)

			dataPoints, err := scraper.GetAverageCPUUtilizationByWorkload("test-nsp-1",
				"test-workload-1", start, end, time.Second)
			fmt.Println(dataPoints)
			Expect(err).NotTo(HaveOccurred())
			Expect(dataPoints).ToNot(BeEmpty())

			//since metrics could have been scraped multiple times, we just check the first and last value
			Expect(len(dataPoints) >= 2).To(BeTrue())

			Expect(dataPoints[0].Value).To(Equal(26.0))
			Expect(dataPoints[len(dataPoints)-1].Value).To(Equal(10.0))
		})
	})
})

var _ = Describe("mergeMatrices", func() {
	It("should correctly merge two matrices", func() {
		matrix1 := model.Matrix{
			&model.SampleStream{
				Metric: model.Metric{"label": "test"},
				Values: []model.SamplePair{
					{Timestamp: 100, Value: 1},
					{Timestamp: 200, Value: 2},
				},
			},
		}

		matrix2 := model.Matrix{
			&model.SampleStream{
				Metric: model.Metric{"label": "test"},
				Values: []model.SamplePair{
					{Timestamp: 300, Value: 3},
					{Timestamp: 400, Value: 4},
				},
			},
		}

		expectedMergedMatrix := model.Matrix{
			&model.SampleStream{
				Metric: model.Metric{"label": "test"},
				Values: []model.SamplePair{
					{Timestamp: 100, Value: 1},
					{Timestamp: 200, Value: 2},
					{Timestamp: 300, Value: 3},
					{Timestamp: 400, Value: 4},
				},
			},
		}

		mergedMatrix := mergeMatrices(matrix1, matrix2)
		Expect(mergedMatrix).To(Equal(expectedMergedMatrix))
	})
})

type mockAPI struct {
	v1.API
	queryRangeFunc func(ctx context.Context, query string, r v1.Range, options ...v1.Option) (model.Value,
		v1.Warnings, error)
}

func (m *mockAPI) QueryRange(ctx context.Context, query string, r v1.Range, options ...v1.Option) (model.Value,
	v1.Warnings, error) {
	return m.queryRangeFunc(ctx, query, r)
}

var _ = Describe("RangeQuerySplitter", func() {
	It("should split and query correctly by duration", func() {
		query := "test_query"
		start := time.Now().Add(-5 * time.Minute)
		end := time.Now()
		step := 1 * time.Minute
		splitDuration := 2 * time.Minute

		mockApi := &mockAPI{
			queryRangeFunc: func(ctx context.Context, query string, r v1.Range, options ...v1.Option) (model.Value,
				v1.Warnings, error) {
				matrix := model.Matrix{
					&model.SampleStream{
						Metric: model.Metric{"label": "test"},
						Values: []model.SamplePair{
							{Timestamp: model.TimeFromUnix(r.Start.Unix()), Value: 1},
							{Timestamp: model.TimeFromUnix(r.End.Unix()), Value: 2},
						},
					},
				}
				return matrix, nil, nil
			},
		}

		splitter := NewRangeQuerySplitter(splitDuration)
		pi := PrometheusInstance{apiUrl: mockApi, address: ""}
		result, err := splitter.QueryRangeByInterval(context.TODO(), pi, query, start, end, step)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Type()).To(Equal(model.ValMatrix))

		matrix := result.(model.Matrix)
		Expect(len(matrix)).To(Equal(1))
		Expect(len(matrix[0].Values)).To(Equal(6))
	})
})

var _ = Describe("interpolateMissingDataPoints", func() {
	It("should interpolate the missing data", func() {
		dataPoints := []DataPoint{
			{Timestamp: time.Now().Add(-30 * time.Minute), Value: 60},
			{Timestamp: time.Now().Add(-29 * time.Minute), Value: 80},
			{Timestamp: time.Now().Add(-28 * time.Minute), Value: 100},
			{Timestamp: time.Now().Add(-27 * time.Minute), Value: 50},
			{Timestamp: time.Now().Add(-26 * time.Minute), Value: 30},
			{Timestamp: time.Now().Add(-25 * time.Minute), Value: 60},
			{Timestamp: time.Now().Add(-24 * time.Minute), Value: 80},
			{Timestamp: time.Now().Add(-20 * time.Minute), Value: 60},
			{Timestamp: time.Now().Add(-19 * time.Minute), Value: 80},
			{Timestamp: time.Now().Add(-18 * time.Minute), Value: 100},
			{Timestamp: time.Now().Add(-17 * time.Minute), Value: 50},
			{Timestamp: time.Now().Add(-16 * time.Minute), Value: 30},
			{Timestamp: time.Now().Add(-9 * time.Minute), Value: 80},
			{Timestamp: time.Now().Add(-8 * time.Minute), Value: 100},
			{Timestamp: time.Now().Add(-7 * time.Minute), Value: 50},
			{Timestamp: time.Now().Add(-6 * time.Minute), Value: 30},
		}
		dataPoints = scraper.interpolateMissingDataPoints(dataPoints, time.Minute)
		Expect(len(dataPoints)).To(Equal(25))
		Expect(dataPoints[7].Value).To(Equal(75.0))
		Expect(dataPoints[8].Value).To(Equal(70.0))
		Expect(math.Floor(dataPoints[15].Value*100) / 100).To(Equal(37.14))
		Expect(math.Floor(dataPoints[20].Value*100) / 100).To(Equal(72.85))
	})
})

var _ = Describe("aggregateMetrics", func() {
	It("should aggregate the metrics from different sources", func() {
		time1 := time.Now().Add(-30 * time.Minute)
		time2 := time.Now().Add(-29 * time.Minute)
		time3 := time.Now().Add(-28 * time.Minute)
		time4 := time.Now().Add(-27 * time.Minute)
		time5 := time.Now().Add(-26 * time.Minute)
		time6 := time.Now().Add(-25 * time.Minute)
		time7 := time.Now().Add(-24 * time.Minute)
		time8 := time.Now().Add(-20 * time.Minute)
		time9 := time.Now().Add(-19 * time.Minute)
		time10 := time.Now().Add(-18 * time.Minute)
		time11 := time.Now().Add(-17 * time.Minute)
		time12 := time.Now().Add(-16 * time.Minute)
		dataPoints1 := []DataPoint{
			{Timestamp: time1, Value: 60},
			{Timestamp: time2, Value: 80},
			{Timestamp: time3, Value: 100},
			{Timestamp: time4, Value: 50},
			{Timestamp: time5, Value: 30},
			{Timestamp: time6, Value: 60},
			{Timestamp: time7, Value: 80},
			{Timestamp: time8, Value: 60},
			{Timestamp: time10, Value: 80},
			{Timestamp: time11, Value: 100},
		}
		dataPoints2 := []DataPoint{
			{Timestamp: time1, Value: 30},
			{Timestamp: time2, Value: 80},
			{Timestamp: time3, Value: 100},
			{Timestamp: time5, Value: 30},
			{Timestamp: time6, Value: 60},
			{Timestamp: time7, Value: 100},
			{Timestamp: time9, Value: 80},
			{Timestamp: time10, Value: 80},
			{Timestamp: time11, Value: 100},
			{Timestamp: time12, Value: 100},
		}
		dataPoints := aggregateMetrics(dataPoints1, dataPoints2)
		fmt.Println(dataPoints)
		Expect(len(dataPoints)).To(Equal(12))
		Expect(dataPoints[0].Value).To(Equal(60.0))
		Expect(dataPoints[6].Value).To(Equal(100.0))
		Expect(dataPoints[11].Value).To(Equal(100.0))
	})
})
