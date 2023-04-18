/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package metrics

import (
	"fmt"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metrics Suite")
}

var (
	registry       *prometheus.Registry
	metricsServer  *http.Server
	metricsAddress string

	cpuUsageMetric *prometheus.GaugeVec

	kubePodOwnerMetric *prometheus.GaugeVec

	scraper *PrometheusScraper
)

var _ = BeforeSuite(func() {
	registry = prometheus.NewRegistry()
	registerMetrics()

	client, err := api.NewClient(api.Config{
		Address: "http://localhost:9090",
	})

	Expect(err).NotTo(HaveOccurred())

	utilizationMetric := "node_namespace_pod_container_container_cpu_usage_seconds_total_sum_irate"
	podOwnerMetric := "namespace_workload_pod_kube_pod_owner_relabel"
	scraper = &PrometheusScraper{api: v1.NewAPI(client),
		metricRegistry: &MetricRegistry{utilizationMetric: utilizationMetric, podOwnerMetric: podOwnerMetric}}

	go func() {
		metricsAddress = "localhost:9091"
		metricsServer = startMetricsServer(registry, metricsAddress)
		defer GinkgoRecover()
	}()
})

var _ = AfterSuite(func() {
	_ = metricsServer.Close()
})

func startMetricsServer(registry *prometheus.Registry, addr string) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(fmt.Sprintf("Failed to start metrics server: %s", err))
		}
	}()

	return server
}

func registerMetrics() {
	cpuUsageMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "node_namespace_pod_container_container_cpu_usage_seconds_total_sum_irate",
		Help: "Test metric for container CPU usage",
	}, []string{"namespace", "pod", "node", "container"})

	kubePodOwnerMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "namespace_workload_pod_kube_pod_owner_relabel",
		Help: "Test metric for Kubernetes pod owner",
	}, []string{"namespace", "pod", "workload", "workload_type"})

	registry.MustRegister(cpuUsageMetric)
	registry.MustRegister(kubePodOwnerMetric)
}
