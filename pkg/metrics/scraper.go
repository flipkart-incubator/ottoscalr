package metrics

import (
	"context"
	"fmt"
	"github.com/prometheus/client_golang/api"
	"github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"sort"
	"sync"
	"time"
)

type DataPoint struct {
	Timestamp time.Time
	Value     float64
}

// Scraper is an interface for scraping metrics data.
type Scraper interface {
	GetAverageCPUUtilizationByWorkload(namespace,
		workload string,
		start time.Time,
		end time.Time,
		step time.Duration) ([]DataPoint, error)

	GetCPUUtilizationBreachDataPoints(namespace,
		workloadType,
		workload string,
		redLineUtilization float64,
		start time.Time,
		end time.Time,
		step time.Duration) ([]DataPoint, error)

	GetACLByWorkload(namespace,
		workload string) (time.Duration, error)
}

// PrometheusScraper is a Scraper implementation that scrapes metrics data from Prometheus.
type PrometheusScraper struct {
	api                 v1.API
	metricRegistry      *MetricNameRegistry
	queryTimeout        time.Duration
	rangeQuerySplitter  *RangeQuerySplitter
	metricIngestionTime float64
	metricProbeTime     float64
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

type PrometheusQueryResult struct {
	result model.Matrix
	err    error
}

func (ps *PrometheusScraper) GetACLByWorkload(namespace string, workload string) (time.Duration, error) {
	podBootStrapTime, err := ps.getPodReadyLatencyByWorkload(namespace, workload)
	if err != nil {
		return 0.0, fmt.Errorf("error getting pod bootstrap time: %v", err)
	}
	totalACL := ps.metricIngestionTime + ps.metricProbeTime + podBootStrapTime
	return time.Duration(totalACL) * time.Second, nil
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

func NewPrometheusScraper(apiURL string,
	timeout time.Duration,
	splitInterval time.Duration,
	metricIngestionTime float64,
	metricProbeTime float64) (*PrometheusScraper, error) {

	client, err := api.NewClient(api.Config{
		Address: apiURL,
	})

	if err != nil {
		return nil, fmt.Errorf("error creating Prometheus client: %v", err)
	}

	v1Api := v1.NewAPI(client)
	return &PrometheusScraper{api: v1Api,
		metricRegistry:      NewKubePrometheusMetricNameRegistry(),
		queryTimeout:        timeout,
		rangeQuerySplitter:  NewRangeQuerySplitter(v1Api, splitInterval),
		metricProbeTime:     metricProbeTime,
		metricIngestionTime: metricIngestionTime}, nil
}

// GetAverageCPUUtilizationByWorkload returns the average CPU utilization for the given workload type and name in the
// specified namespace, in the given time range.
func (ps *PrometheusScraper) GetAverageCPUUtilizationByWorkload(namespace string,
	workload string,
	start time.Time,
	end time.Time,
	step time.Duration) ([]DataPoint, error) {

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

	result, err := ps.rangeQuerySplitter.QueryRangeByInterval(ctx, query, start, end, step)

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

	var dataPoints []DataPoint
	for _, sample := range matrix[0].Values {
		datapoint := DataPoint{sample.Timestamp.Time(), float64(sample.Value)}
		if !sample.Timestamp.Time().IsZero() {
			dataPoints = append(dataPoints, datapoint)
		}
	}

	sort.SliceStable(dataPoints, func(i, j int) bool {
		return dataPoints[i].Timestamp.Before(dataPoints[j].Timestamp)
	})
	dataPoints = ps.interpolateMissingDataPoints(dataPoints, step)
	return dataPoints, nil
}

// GetCPUUtilizationBreachDataPoints returns the data points where avg CPU utilization for a workload goes above the
// redLineUtilization while no of ready pods for the workload were < maxReplicas defined in the HPA.
func (ps *PrometheusScraper) GetCPUUtilizationBreachDataPoints(namespace,
	workloadType,
	workload string,
	redLineUtilization float64,
	start time.Time,
	end time.Time,
	step time.Duration) ([]DataPoint, error) {
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
		" (namespace, owner_kind, owner_name) < on(namespace, owner_kind, owner_name) "+
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

	result, err := ps.rangeQuerySplitter.QueryRangeByInterval(ctx, query, start, end, step)
	if err != nil {
		return nil, fmt.Errorf("failed to execute Prometheus query: %v", err)
	}
	if result.Type() != model.ValMatrix {
		return nil, fmt.Errorf("unexpected result type: %v", result.Type())
	}
	matrix := result.(model.Matrix)

	if len(matrix) != 1 {
		// if no datapoints are returned which satisfy the query it can be considered that there's no breach to redLineUtilization
		return nil, nil
	}

	var dataPoints []DataPoint
	for _, sample := range matrix[0].Values {
		datapoint := DataPoint{sample.Timestamp.Time(), float64(sample.Value)}
		if !sample.Timestamp.Time().IsZero() {
			dataPoints = append(dataPoints, datapoint)
		}
	}

	sort.SliceStable(dataPoints, func(i, j int) bool {
		return dataPoints[i].Timestamp.Before(dataPoints[j].Timestamp)
	})
	return dataPoints, nil
}

// RangeQuerySplitter splits a given queryRange into multiple range queries of width splitInterval. This is done to
// avoid loading too many samples into P8s memory.
type RangeQuerySplitter struct {
	api           v1.API
	splitInterval time.Duration
}

func NewRangeQuerySplitter(api v1.API, splitInterval time.Duration) *RangeQuerySplitter {
	return &RangeQuerySplitter{api: api, splitInterval: splitInterval}
}
func (rqs *RangeQuerySplitter) QueryRangeByInterval(ctx context.Context,
	query string,
	start, end time.Time,
	step time.Duration) (model.Value, error) {

	var resultMatrix model.Matrix

	resultChanLength := int(end.Sub(start).Hours()/rqs.splitInterval.Hours()) + 50 //Added some buffer
	resultChan := make(chan PrometheusQueryResult, resultChanLength)
	var wg sync.WaitGroup

	for start.Before(end) {
		splitEnd := start.Add(rqs.splitInterval)
		if splitEnd.After(end) {
			splitEnd = end
		}
		splitRange := v1.Range{
			Start: start,
			End:   splitEnd,
			Step:  step,
		}

		wg.Add(1)
		go func(splitRange v1.Range) {
			defer wg.Done()
			partialResult, _, err := rqs.api.QueryRange(ctx, query, splitRange)
			if err != nil {
				resultChan <- PrometheusQueryResult{nil, fmt.Errorf("failed to execute Prometheus query: %v", err)}
				return
			}

			if partialResult.Type() != model.ValMatrix {
				resultChan <- PrometheusQueryResult{nil, fmt.Errorf("unexpected result type: %v", partialResult.Type())}
				return
			}

			partialMatrix := partialResult.(model.Matrix)
			resultChan <- PrometheusQueryResult{partialMatrix, nil}

		}(splitRange)

		start = splitEnd
	}

	wg.Wait()
	close(resultChan)

	for p8sQueryResult := range resultChan {
		if p8sQueryResult.err != nil {
			return nil, p8sQueryResult.err
		}
		resultMatrix = mergeMatrices(resultMatrix, p8sQueryResult.result)
	}
	return resultMatrix, nil
}

func mergeMatrices(matrixA, matrixB model.Matrix) model.Matrix {
	if len(matrixA) == 0 {
		return matrixB
	}

	if len(matrixB) == 0 {
		return matrixA
	}

	resultMatrix := make(model.Matrix, len(matrixA))

	for i, seriesA := range matrixA {
		seriesB := matrixB[i]
		mergedSeries := model.SampleStream{
			Metric: seriesA.Metric,
			Values: append(seriesA.Values, seriesB.Values...),
		}
		resultMatrix[i] = &mergedSeries
	}

	return resultMatrix
}
func (ps *PrometheusScraper) getPodReadyLatencyByWorkload(namespace string, workload string) (float64, error) {

	ctx, cancel := context.WithTimeout(context.Background(), ps.queryTimeout)
	defer cancel()

	query := fmt.Sprintf("quantile(0.5,(%s"+
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

	return podBootstrapTime, nil
}

func (ps *PrometheusScraper) interpolateMissingDataPoints(dataPoints []DataPoint, step time.Duration) []DataPoint {
	var interpolatedData []DataPoint
	prevTimestamp := dataPoints[0].Timestamp
	prevValue := dataPoints[0].Value

	interpolatedData = append(interpolatedData, dataPoints[0])

	for i := 1; i < len(dataPoints); i++ {
		currTimestamp := dataPoints[i].Timestamp
		currValue := dataPoints[i].Value

		//Find Missing time intervals
		diff := currTimestamp.Sub(prevTimestamp)
		missingIntervals := int(diff / step)
		if missingIntervals > 1 {
			stepSize := (currValue - prevValue) / float64(missingIntervals)
			for j := 1; j < missingIntervals; j++ {
				interpolatedTimestamp := prevTimestamp.Add(step * time.Duration(j))
				interpolatedValue := prevValue + float64(j)*stepSize
				interpolatedData = append(interpolatedData, DataPoint{Timestamp: interpolatedTimestamp, Value: interpolatedValue})
			}
		}

		interpolatedData = append(interpolatedData, dataPoints[i])
		prevTimestamp = currTimestamp
		prevValue = currValue
	}

	return interpolatedData
}
