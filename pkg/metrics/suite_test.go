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
	"net/http"
	"testing"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

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
	registry        *prometheus.Registry
	registry1       *prometheus.Registry
	metricsServer   *http.Server
	metricsServer1  *http.Server
	metricsAddress  string
	metricsAddress1 string

	cpuUsageMetric *prometheus.GaugeVec

	kubePodOwnerMetric *prometheus.GaugeVec

	resourceLimitMetric   *prometheus.GaugeVec
	readyReplicasMetric   *prometheus.GaugeVec
	replicaSetOwnerMetric *prometheus.GaugeVec
	hpaMaxReplicasMetric  *prometheus.GaugeVec
	hpaOwnerInfoMetric    *prometheus.GaugeVec
	podCreatedTimeMetric  *prometheus.GaugeVec
	podReadyTimeMetric    *prometheus.GaugeVec

	cpuUsageMetric1 *prometheus.GaugeVec

	resourceLimitMetric1   *prometheus.GaugeVec
	readyReplicasMetric1   *prometheus.GaugeVec
	replicaSetOwnerMetric1 *prometheus.GaugeVec
	hpaMaxReplicasMetric1  *prometheus.GaugeVec
	hpaOwnerInfoMetric1    *prometheus.GaugeVec
	podCreatedTimeMetric1  *prometheus.GaugeVec
	podReadyTimeMetric1    *prometheus.GaugeVec

	scraper *PrometheusScraper
)

var _ = BeforeSuite(func() {
	registry = prometheus.NewRegistry()
	registerMetrics()

	registry1 = prometheus.NewRegistry()
	registerMetrics1()

	client, err := api.NewClient(api.Config{
		Address: "http://localhost:9090",
	})
	Expect(err).NotTo(HaveOccurred())

	client1, err := api.NewClient(api.Config{
		Address: "http://localhost:8080",
	})

	Expect(err).NotTo(HaveOccurred())

	api := v1.NewAPI(client)
	api1 := v1.NewAPI(client1)
	metricIngestionTime := 15.0
	metricProbeTime := 15.0

	var v1Api []PrometheusInstance
	v1Api = append(v1Api, PrometheusInstance{
		apiUrl:  api,
		address: "http://localhost:9090",
	}, PrometheusInstance{
		apiUrl:  api1,
		address: "http://localhost:8080",
	})

	compositeQuery := NewPrometheusCompositeQueries()

	scraper = &PrometheusScraper{api: v1Api,
		queryTimeout:              30 * time.Second,
		rangeQuerySplitter:        NewRangeQuerySplitter(1 * time.Second),
		metricIngestionTime:       metricIngestionTime,
		metricProbeTime:           metricProbeTime,
		CPUUtilizationQuery:       (*CPUUtilizationQuery)(compositeQuery),
		CPUUtilizationBreachQuery: (*CPUUtilizationBreachQuery)(compositeQuery),
		PodReadyLatencyQuery:      (*PodReadyLatencyQuery)(compositeQuery),
	}

	go func() {
		metricsAddress = "localhost:9091"
		metricsServer = startMetricsServer(registry, metricsAddress)
		metricsAddress1 = "localhost:9092"
		metricsServer1 = startMetricsServer(registry1, metricsAddress1)
		defer GinkgoRecover()
	}()
})

var _ = AfterSuite(func() {
	_ = metricsServer.Close()
	_ = metricsServer1.Close()
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
		Name: "node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate",
		Help: "Test metric for container CPU usage",
	}, []string{"namespace", "pod", "node", "container"})

	kubePodOwnerMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "namespace_workload_pod:kube_pod_owner:relabel",
		Help: "Test metric for Kubernetes pod owner",
	}, []string{"namespace", "pod", "workload", "workload_type"})

	resourceLimitMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "cluster:namespace:pod_cpu:active:kube_pod_container_resource_limits",
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
		Name: "kube_pod_status_ready_time",
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

func registerMetrics1() {
	cpuUsageMetric1 = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate",
		Help: "Test metric for container CPU usage",
	}, []string{"namespace", "pod", "node", "container"})

	resourceLimitMetric1 = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "cluster:namespace:pod_cpu:active:kube_pod_container_resource_limits",
		Help: "Test metric for container resource limits",
	}, []string{"namespace", "pod", "node", "container"})

	readyReplicasMetric1 = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "kube_replicaset_status_ready_replicas",
		Help: "Test metric for ready replicas in a replicaSet",
	}, []string{"namespace", "replicaset"})

	replicaSetOwnerMetric1 = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "kube_replicaset_owner",
		Help: "Test metric for replicaset owner",
	}, []string{"namespace", "owner_kind", "owner_name", "replicaset"})

	hpaMaxReplicasMetric1 = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "kube_horizontalpodautoscaler_spec_max_replicas",
		Help: "Test metric for hpa max replicas",
	}, []string{"namespace", "horizontalpodautoscaler"})

	hpaOwnerInfoMetric1 = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "kube_horizontalpodautoscaler_info",
		Help: "Test metric for hpa owner",
	}, []string{"namespace", "horizontalpodautoscaler", "scaletargetref_kind", "scaletargetref_name"})

	podCreatedTimeMetric1 = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "kube_pod_created",
		Help: "Test metric pod created",
	}, []string{"namespace", "pod"})

	podReadyTimeMetric1 = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "kube_pod_status_ready_time",
		Help: "Test metric pod ready",
	}, []string{"namespace", "pod"})

	registry1.MustRegister(cpuUsageMetric1)
	registry1.MustRegister(kubePodOwnerMetric)
	registry1.MustRegister(resourceLimitMetric1)
	registry1.MustRegister(readyReplicasMetric1)
	registry1.MustRegister(replicaSetOwnerMetric1)
	registry1.MustRegister(hpaMaxReplicasMetric1)
	registry1.MustRegister(hpaOwnerInfoMetric1)
	registry1.MustRegister(podCreatedTimeMetric1)
	registry1.MustRegister(podReadyTimeMetric1)
}
