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
	GetAverageCPUUtilizationByWorkload(namespace,
		workload string,
		start time.Time,
		end time.Time,
		step time.Duration) ([]float64, error)

	GetCPUUtilizationBreachDataPoints(namespace,
		workloadType,
		workload string,
		redLineUtilization float32,
		start time.Time,
		end time.Time,
		step time.Duration) ([]float64, error)

	GetPodReadyLatencyByWorkload(namespace,
		workload string,
	) (float64, error)
}

// PrometheusScraper is a Scraper implementation that scrapes metrics data from Prometheus.
type PrometheusScraper struct {
	api            v1.API
	metricRegistry *MetricNameRegistry
	queryTimeout   time.Duration
}

type MetricNameRegistry struct {
	utilizationMetric     string
	podOwnerMetric        string
	resourceLimitMetric   string
	readyReplicasMetric   string
	replicaSetOwnerMetric string
	hpaMaxReplicasMetric  string
	hpaOwnerInfoMetric    string
	podCreatedTimeMetric  string
	podReadyTimeMetric    string
}

type ACLComponents struct {
	scraper             *PrometheusScraper
	metricIngestionTime float64
	metricProbeTime     float64
}

func NewACLComponent(ps *PrometheusScraper, metricIngestionTime float64, metricProbeTime float64) *ACLComponents {
	return &ACLComponents{
		scraper:             ps,
		metricIngestionTime: metricIngestionTime,
		metricProbeTime:     metricProbeTime,
	}

}

func (ac *ACLComponents) GetACLByWorkload(namespace string, workload string) (float64, error) {
	podBootStrapTime, err := ac.scraper.GetPodReadyLatencyByWorkload(namespace, workload)
	if err != nil {
		return 0.0, fmt.Errorf("error getting pod bootstrap time: %v", err)
	}
	totalACL := ac.metricIngestionTime + ac.metricProbeTime + podBootStrapTime
	return totalACL, nil
}

func NewKubePrometheusMetricNameRegistry() *MetricNameRegistry {
	cpuUtilizationMetric := "node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate"
	podOwnerMetric := "namespace_workload_pod:kube_pod_owner:relabel"
	resourceLimitMetric := "cluster:namespace:pod_cpu:active:kube_pod_container_resource_limits"
	readyReplicasMetric := "kube_replicaset_status_ready_replicas"
	replicaSetOwnerMetric := "kube_replicaset_owner"
	hpaMaxReplicasMetric := "kube_horizontalpodautoscaler_spec_max_replicas"
	hpaOwnerInfoMetric := "kube_horizontalpodautoscaler_info"
	podCreatedTimeMetric := "kube_pod_created"
	podReadyTimeMetric := "alm_kube_pod_ready_time"

	return &MetricNameRegistry{utilizationMetric: cpuUtilizationMetric,
		podOwnerMetric:        podOwnerMetric,
		resourceLimitMetric:   resourceLimitMetric,
		readyReplicasMetric:   readyReplicasMetric,
		replicaSetOwnerMetric: replicaSetOwnerMetric,
		hpaMaxReplicasMetric:  hpaMaxReplicasMetric,
		hpaOwnerInfoMetric:    hpaOwnerInfoMetric,
		podCreatedTimeMetric:  podCreatedTimeMetric,
		podReadyTimeMetric:    podReadyTimeMetric,
	}
}

// NewPrometheusScraper returns a new PrometheusScraper instance.

func NewPrometheusScraper(apiURL string, timeout time.Duration) (*PrometheusScraper, error) {

	client, err := api.NewClient(api.Config{
		Address: apiURL,
	})

	if err != nil {
		return nil, fmt.Errorf("error creating Prometheus client: %v", err)
	}

	return &PrometheusScraper{api: v1.NewAPI(client),
			metricRegistry: NewKubePrometheusMetricNameRegistry(),
			queryTimeout:   timeout},
		nil
}

// GetAverageCPUUtilizationByWorkload returns the average CPU utilization for the given workload type and name in the
// specified namespace, in the given time range.
func (ps *PrometheusScraper) GetAverageCPUUtilizationByWorkload(namespace string,
	workload string,
	start time.Time,
	end time.Time,
	step time.Duration) ([]float64, error) {

	ctx, cancel := context.WithTimeout(context.Background(), ps.queryTimeout)
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

	queryRange := v1.Range{Start: start, End: end, Step: step}
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
	redLineUtilization float32,
	start time.Time,
	end time.Time,
	step time.Duration) ([]float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ps.queryTimeout)
	defer cancel()

	query := fmt.Sprintf("(sum(%s{"+
		"namespace=\"%s\"} * on(namespace,pod) group_left(workload, workload_type) "+
		"%s{namespace=\"%s\", workload=\"%s\", workload_type=\"deployment\"})"+
		" by (namespace, workload, workload_type)/ on (namespace, workload, workload_type) "+
		"group_left sum(%s{"+
		"namespace=\"%s\"} * on(namespace,pod) group_left(workload, workload_type)"+
		"%s{namespace=\"%s\", workload=\"%s\", workload_type=\"deployment\"}) "+
		"by (namespace, workload, workload_type) > %.2f) and on(namespace, workload) "+
		"label_replace(sum(%s{namespace=\"%s\"} * on(replicaset)"+
		" group_left(namespace, owner_kind, owner_name) %s{namespace=\"%s\", owner_kind=\"%s\", owner_name=\"%s\"}) by"+
		" (namespace, owner_kind, owner_name) >= on(namespace, owner_kind, owner_name) "+
		"(%s{namespace=\"%s\"} * on(namespace, horizontalpodautoscaler) "+
		"group_left(owner_kind, owner_name) label_replace(label_replace(%s{"+
		"namespace=\"%s\", scaletargetref_kind=\"%s\", scaletargetref_name=\"%s\"},\"owner_kind\", \"$1\", "+
		"\"scaletargetref_kind\", \"(.*)\"), \"owner_name\", \"$1\", \"scaletargetref_name\", \"(.*)\")),"+
		"\"workload\", \"$1\", \"owner_name\", \"(.*)\")",
		ps.metricRegistry.utilizationMetric,
		namespace,
		ps.metricRegistry.podOwnerMetric,
		namespace,
		workload,
		ps.metricRegistry.resourceLimitMetric,
		namespace,
		ps.metricRegistry.podOwnerMetric,
		namespace,
		workload,
		redLineUtilization,
		ps.metricRegistry.readyReplicasMetric,
		namespace,
		ps.metricRegistry.replicaSetOwnerMetric,
		namespace,
		workloadType,
		workload,
		ps.metricRegistry.hpaMaxReplicasMetric,
		namespace,
		ps.metricRegistry.hpaOwnerInfoMetric,
		namespace,
		workloadType,
		workload)

	queryRange := v1.Range{Start: start, End: end, Step: step}

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

func (ps *PrometheusScraper) GetPodReadyLatencyByWorkload(namespace string,
	workload string,
) (float64, error) {

	ctx, cancel := context.WithTimeout(context.Background(), ps.queryTimeout)
	defer cancel()

	query := fmt.Sprintf("min((%s"+
		"{namespace=\"%s\"} - on (namespace,pod) (%s{namespace=\"%s\"}))  * on (namespace,pod) group_left(workload, workload_type)"+
		"(%s{namespace=\"%s\", workload=\"%s\","+
		" workload_type=\"deployment\"}))",
		ps.metricRegistry.podReadyTimeMetric,
		namespace,
		ps.metricRegistry.podCreatedTimeMetric,
		namespace,
		ps.metricRegistry.podOwnerMetric,
		namespace,
		workload)

	result, _, err := ps.api.Query(ctx, query, time.Now())

	if err != nil {
		return 0.0, fmt.Errorf("failed to execute Prometheus query: %v", err)
	}
	if result.Type() != model.ValVector {
		return 0.0, fmt.Errorf("unexpected result type: %v", result.Type())
	}
	matrix := result.(model.Vector)

	if len(matrix) != 1 {
		return 0.0, fmt.Errorf("unexpected no of time series: %v", len(matrix))
	}

	podBootstrapTime := float64(matrix[0].Value)
	if err != nil {
		return 0.0, fmt.Errorf("")
	}

	return podBootstrapTime, nil
}
