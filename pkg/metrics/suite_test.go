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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

func TestMetrics(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metrics Suite")
}

var (
	registry       *prometheus.Registry
	metricsServer  *http.Server
	metricsAddress string

	cpuUsageMetric *prometheus.GaugeVec

	kubePodOwnerMetric *prometheus.GaugeVec

	resourceLimitMetric   *prometheus.GaugeVec
	readyReplicasMetric   *prometheus.GaugeVec
	replicaSetOwnerMetric *prometheus.GaugeVec
	hpaMaxReplicasMetric  *prometheus.GaugeVec
	hpaOwnerInfoMetric    *prometheus.GaugeVec
	podCreatedTimeMetric  *prometheus.GaugeVec
	podReadyTimeMetric    *prometheus.GaugeVec

	scraper *PrometheusScraper

	acl *ACLComponents
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
	resourceLimitMetric := "cluster_namespace_pod_cpu_active_kube_pod_container_resource_limits"
	readyReplicasMetric := "kube_replicaset_status_ready_replicas"
	replicaSetOwnerMetric := "kube_replicaset_owner"
	hpaMaxReplicasMetric := "kube_horizontalpodautoscaler_spec_max_replicas"
	hpaOwnerInfoMetric := "kube_horizontalpodautoscaler_info"
	podCreatedTimeMetric := "kube_pod_created"
	podReadyTimeMetric := "alm_kube_pod_ready_time"

	scraper = &PrometheusScraper{api: v1.NewAPI(client),
		metricRegistry: &MetricNameRegistry{
			utilizationMetric:     utilizationMetric,
			podOwnerMetric:        podOwnerMetric,
			resourceLimitMetric:   resourceLimitMetric,
			readyReplicasMetric:   readyReplicasMetric,
			replicaSetOwnerMetric: replicaSetOwnerMetric,
			hpaMaxReplicasMetric:  hpaMaxReplicasMetric,
			hpaOwnerInfoMetric:    hpaOwnerInfoMetric,
			podCreatedTimeMetric:  podCreatedTimeMetric,
			podReadyTimeMetric:    podReadyTimeMetric,
		},
		queryTimeout: 30 * time.Second}

	metricIngestionTime := 15.0
	metricProbeTime := 15.0
	acl = &ACLComponents{scraper: scraper,
		metricIngestionTime: metricIngestionTime,
		metricProbeTime:     metricProbeTime}

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

	resourceLimitMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "cluster_namespace_pod_cpu_active_kube_pod_container_resource_limits",
		Help: "Test metric for container resource limits",
	}, []string{"namespace", "pod", "node", "container"})

	readyReplicasMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "kube_replicaset_status_ready_replicas",
		Help: "Test metric for ready replicas in a replicaSet",
	}, []string{"namespace", "replicaset"})

	replicaSetOwnerMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "kube_replicaset_owner",
		Help: "Test metric for replicaset owner",
	}, []string{"namespace", "owner_kind", "owner_name", "replicaset"})

	hpaMaxReplicasMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "kube_horizontalpodautoscaler_spec_max_replicas",
		Help: "Test metric for hpa max replicas",
	}, []string{"namespace", "horizontalpodautoscaler"})

	hpaOwnerInfoMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "kube_horizontalpodautoscaler_info",
		Help: "Test metric for hpa owner",
	}, []string{"namespace", "horizontalpodautoscaler", "scaletargetref_kind", "scaletargetref_name"})

	podCreatedTimeMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "kube_pod_created",
		Help: "Test metric pod created",
	}, []string{"namespace", "pod"})

	podReadyTimeMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "alm_kube_pod_ready_time",
		Help: "Test metric pod ready",
	}, []string{"namespace", "pod"})

	registry.MustRegister(cpuUsageMetric)
	registry.MustRegister(kubePodOwnerMetric)
	registry.MustRegister(resourceLimitMetric)
	registry.MustRegister(readyReplicasMetric)
	registry.MustRegister(replicaSetOwnerMetric)
	registry.MustRegister(hpaMaxReplicasMetric)
	registry.MustRegister(hpaOwnerInfoMetric)
	registry.MustRegister(podCreatedTimeMetric)
	registry.MustRegister(podReadyTimeMetric)
}
