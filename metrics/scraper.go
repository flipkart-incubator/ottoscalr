package metrics

import (
	"context"
	"fmt"
	"github.com/prometheus/client_golang/api"
	"time"

	"github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// Scraper is an interface for scraping metrics data.
type Scraper interface {
	GetAverageCPUUtilizationByWorkload(namespace, workloadType, workload string, start time.Time) ([]float64, error)

	GetCPUUtilizationBreachDataPoints(namespace, workloadType, workload string, start time.Time) ([]float64, error)
}

// PrometheusScraper is a Scraper implementation that scrapes metrics data from Prometheus.
type PrometheusScraper struct {
	api            v1.API
	metricRegistry *MetricRegistry
}

type MetricRegistry struct {
	utilizationMetric string
	podOwnerMetric    string
}

// NewPrometheusScraper returns a new PrometheusScraper instance.

func NewPrometheusScraper(apiURL string) (*PrometheusScraper, error) {

	client, err := api.NewClient(api.Config{
		Address: apiURL,
	})

	if err != nil {
		return nil, fmt.Errorf("error creating Prometheus client: %v", err)
	}
	utilizationMetric := "node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate"
	podOwnerMetric := "namespace_workload_pod:kube_pod_owner:relabel"
	return &PrometheusScraper{api: v1.NewAPI(client),
			metricRegistry: &MetricRegistry{utilizationMetric: utilizationMetric, podOwnerMetric: podOwnerMetric}},
		nil
}

// GetAverageCPUUtilizationByWorkload returns the average CPU utilization for the given workload type and name in the
// specified namespace, in the given time range.
func (ps *PrometheusScraper) GetAverageCPUUtilizationByWorkload(namespace string,
	workload string,
	start time.Time,
	end time.Time) ([]float64, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := fmt.Sprintf("sum(%s"+
		"{namespace=\"%s\"} * on (namespace,pod) group_left(workload, workload_type)"+
		"%s{namespace=\"%s\", workload=\"%s\","+
		" workload_type=\"deployment\"}) by(namespace, workload, workload_type)",
		ps.metricRegistry.utilizationMetric,
		namespace,
		ps.metricRegistry.podOwnerMetric,
		namespace,
		workload)

	queryRange := v1.Range{Start: start, End: end, Step: time.Minute}
	result, _, err := ps.api.QueryRange(ctx, query, queryRange)

	if err != nil {
		return nil, fmt.Errorf("failed to execute Prometheus query: %v", err)
	}
	if result.Type() != model.ValMatrix {
		return nil, fmt.Errorf("unexpected result type: %v", result.Type())
	}

	matrix := result.(model.Matrix)
	if len(matrix) != 1 {
		return nil, fmt.Errorf("unexpected no of time series: %v", len(matrix))
	}

	var dataPoints []float64
	for _, sample := range matrix[0].Values {
		cpuUtilization := float64(sample.Value)
		if !sample.Timestamp.Time().IsZero() {
			dataPoints = append(dataPoints, cpuUtilization)
		}
	}
	return dataPoints, nil
}

// GetCPUUtilizationBreachDataPoints returns the data points where avg CPU utilization for a workload goes above the
// redLineUtilization while no of ready pods for the workload were < maxReplicas defined in the HPA.
func (ps *PrometheusScraper) GetCPUUtilizationBreachDataPoints(namespace,
	workloadType,
	workload string,
	redLineUtilization float64,
	start time.Time) ([]float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := fmt.Sprintf("(sum(node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate{"+
		"namespace=\"%s\"} * on(namespace,pod) group_left(workload, workload_type) "+
		"namespace_workload_pod:kube_pod_owner:relabel{namespace=\"%s\", workload=\"%s\", workload_type=\"deployment\"})"+
		" by (namespace, workload, workload_type)/ on (namespace, workload, workload_type) "+
		"group_left sum(cluster:namespace:pod_cpu:active:kube_pod_container_resource_limits{"+
		"namespace=\"%s\"} * on(namespace,pod) group_left(workload, workload_type)"+
		"namespace_workload_pod:kube_pod_owner:relabel{namespace=\"%s\", workload=\"%s\", workload_type=\"%s\"}) "+
		"by (namespace, workload, workload_type) > %.2f) and on(namespace, workload) "+
		"label_replace(sum(kube_replicaset_status_ready_replicas{namespace=\"%s\"} * on(replicaset)"+
		" group_left(namespace, owner_kind, owner_name) kube_replicaset_owner{owner_kind=\"%s\", owner_name=\"%s\"}) by"+
		" (namespace, owner_kind, owner_name) < on(namespace, owner_kind, owner_name) "+
		"(kube_horizontalpodautoscaler_spec_max_replicas{namespace=\"%s\"} * on(namespace, horizontalpodautoscaler) "+
		"group_left(owner_kind, owner_name) label_replace(label_replace(kube_horizontalpodautoscaler_info{"+
		"namespace=\"%s\", scaletargetref_kind=\"%s\", scaletargetref_name=\"%s\"},\"owner_kind\", \"$1\", "+
		"\"scaletargetref_kind\", \"(.*)\"), \"owner_name\", \"$1\", \"scaletargetref_name\", \"(.*)\")),"+
		"\"workload\", \"$1\", \"owner_name\", \"(.*)\")", namespace, namespace, workload, namespace, namespace,
		workload, workloadType, redLineUtilization, namespace, workloadType, workload, namespace, namespace,
		workloadType, workload)

	result, _, err := ps.api.Query(ctx, query, start)
	if err != nil {
		return nil, fmt.Errorf("failed to execute Prometheus query: %v", err)
	}
	if result.Type() != model.ValVector {
		return nil, fmt.Errorf("unexpected result type: %v", result.Type())
	}
	vector := *result.(*model.Vector)

	fmt.Println(query)
	if len(vector) == 0 {
		return nil, nil
	}

	var dataPoints []float64
	for _, sample := range vector {
		cpuUtilization := float64(sample.Value)
		if !sample.Timestamp.Time().IsZero() {
			dataPoints = append(dataPoints, cpuUtilization)
		}
	}
	return dataPoints, nil
}
