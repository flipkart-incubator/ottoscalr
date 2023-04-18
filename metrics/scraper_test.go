package metrics

import (
	"fmt"
	"net/http"
	"strings"
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
			scraper, err := NewPrometheusScraper("http://localhost:9090")
			Expect(err).NotTo(HaveOccurred())

			// Register your metrics with the testRegistry, or use dummy data
			t1 := time.Now()
			//t2 := t1.Add(15 * time.Second)
			//t3 := t1.Add(30 * time.Second)

			err = pushTimeSeriesData("namespace_workload_pod:kube_pod_owner:relabel", "1",
				map[string]string{"namespace": "test-ns",
					"pod":           "test-pod-1",
					"workload":      "test-workload",
					"workload_type": "deployment"})
			Expect(err).NotTo(HaveOccurred())

			err = pushTimeSeriesData("node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate",
				"13.2",
				map[string]string{"namespace": "test-ns",
					"pod":       "test-pod-1",
					"node":      "test-node-1",
					"container": "test-container-1"})
			Expect(err).NotTo(HaveOccurred())
			time.Sleep(10 * time.Second)

			dataPoints, err := scraper.GetAverageCPUUtilizationByWorkload("test-ns",
				"test-workload", t1.Add(-5*time.Minute), time.Now())
			Expect(err).NotTo(HaveOccurred())

			// Check the data points
			Expect(dataPoints).ToNot(BeEmpty())
			// Add more assertions to check the content of dataPoints
		})
	})

})

func pushTimeSeriesData(metricName, metricValue string, customLabels map[string]string) error {
	data := fmt.Sprintf("%s %s\n", metricName, metricValue)
	gatewayURL := fmt.Sprintf("http://localhost:9091/metrics/job/test-job")

	// Append custom labels to the URL
	for labelName, labelValue := range customLabels {
		gatewayURL += fmt.Sprintf("/%s/%s", labelName, labelValue)
	}

	resp, err := http.Post(gatewayURL, "text/plain; version=0.0.4", strings.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to push time series data, status code: %d", resp.StatusCode)
	}
	return nil
}
