package metrics

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/api"
	"github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/common/model"
	"math"
	p8smetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	"sort"
	"sync"
	"time"
)

const (
	CPUUtilizationDataPointsQuery = "cpuUtilizationDataPointsQuery"
	BreachDataPointsQuery         = "breachDataPointsQuery"
)

var (
	prometheusQueryLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{Name: "prometheus_scraper_query_latency",
			Help: "Time to execute prometheus scraper query in seconds"}, []string{"namespace", "query", "instance", "workload"},
	)

	dataPointsFetched = promauto.NewGaugeVec(
		prometheus.GaugeOpts{Name: "datapoints_fetched_by_p8s_instance",
			Help: "Number of Datapoints fetched by quering p8s instance"}, []string{"namespace", "query", "instance", "workload"},
	)

	totalDataPointsFetched = promauto.NewGaugeVec(
		prometheus.GaugeOpts{Name: "total_datapoints_fetched",
			Help: "Total Number of Datapoints fetched by quering p8s instances"}, []string{"namespace", "query", "workload"},
	)

	p8sInstanceQueried = promauto.NewGaugeVec(
		prometheus.GaugeOpts{Name: "p8s_instance_queried",
			Help: "Number of P8s instances which returned datapoints"}, []string{"namespace", "query", "instance", "workload"},
	)
)

func init() {
	p8smetrics.Registry.MustRegister(prometheusQueryLatency, dataPointsFetched, totalDataPointsFetched, p8sInstanceQueried)
}

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
	api                 []PrometheusInstance
	metricRegistry      *MetricNameRegistry
	queryTimeout        time.Duration
	rangeQuerySplitter  *RangeQuerySplitter
	metricIngestionTime float64
	metricProbeTime     float64
	logger              logr.Logger
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

type PrometheusInstance struct {
	apiUrl  v1.API
	address string
}

// NewPrometheusScraper returns a new PrometheusScraper instance.

func NewPrometheusScraper(apiUrls []string,
	timeout time.Duration,
	splitInterval time.Duration,
	metricIngestionTime float64,
	metricProbeTime float64,
	logger logr.Logger) (*PrometheusScraper, error) {

	var prometheusInstances []PrometheusInstance
	for _, pi := range apiUrls {
		logger.Info("prometheus instance ", "endpoint", pi)
		client, err := api.NewClient(api.Config{
			Address: pi,
		})

		if err != nil {
			return nil, fmt.Errorf("error creating Prometheus client: %v", err)
		}

		prometheusInstances = append(prometheusInstances, PrometheusInstance{
			apiUrl:  v1.NewAPI(client),
			address: pi,
		})
	}

	return &PrometheusScraper{api: prometheusInstances,
		metricRegistry:      NewKubePrometheusMetricNameRegistry(),
		queryTimeout:        timeout,
		rangeQuerySplitter:  NewRangeQuerySplitter(splitInterval),
		metricProbeTime:     metricProbeTime,
		metricIngestionTime: metricIngestionTime,
		logger:              logger}, nil
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

	var totalDataPoints []DataPoint
	if ps.api == nil {
		return nil, fmt.Errorf("no apiurl for executing prometheus query")
	}

	resultChanLength := len(ps.api) + 5 //Added some buffer
	resultChan := make(chan []DataPoint, resultChanLength)
	var wg sync.WaitGroup
	for _, pi := range ps.api {

		wg.Add(1)
		go func(pi PrometheusInstance) {
			defer wg.Done()

			p8sQueryStartTime := time.Now()
			result, err := ps.rangeQuerySplitter.QueryRangeByInterval(ctx, pi.apiUrl, query, start, end, step)

			if err != nil {
				ps.logger.Error(err, "failed to execute Prometheus query", "Instance", pi.address)
				logP8sMetrics(p8sQueryStartTime, namespace, CPUUtilizationDataPointsQuery, pi.address, workload, -1, 0)
				resultChan <- nil
				return
			}
			if result.Type() != model.ValMatrix {
				ps.logger.Error(fmt.Errorf("unexpected result type: %v", result.Type()), "Result Type Error", "Instance", pi.address)
				logP8sMetrics(p8sQueryStartTime, namespace, CPUUtilizationDataPointsQuery, pi.address, workload, -1, 1)
				resultChan <- nil
				return
			}

			matrix := result.(model.Matrix)
			if len(matrix) != 1 {
				ps.logger.Error(fmt.Errorf("unexpected no of time series: %v", len(matrix)), "Zero Datapoints Error", "Instance", pi.address)
				logP8sMetrics(p8sQueryStartTime, namespace, CPUUtilizationDataPointsQuery, pi.address, workload, 0, 1)
				resultChan <- nil
				return
			}
			var dataPoints []DataPoint
			for _, sample := range matrix[0].Values {
				datapoint := DataPoint{sample.Timestamp.Time(), float64(sample.Value)}
				if !sample.Timestamp.Time().IsZero() {
					dataPoints = append(dataPoints, datapoint)
				}
			}
			logP8sMetrics(p8sQueryStartTime, namespace, CPUUtilizationDataPointsQuery, pi.address, workload, len(dataPoints), 1)

			sort.SliceStable(dataPoints, func(i, j int) bool {
				return dataPoints[i].Timestamp.Before(dataPoints[j].Timestamp)
			})
			resultChan <- dataPoints
		}(pi)
	}
	wg.Wait()
	close(resultChan)

	for p8sQueryResult := range resultChan {
		totalDataPoints = aggregateMetrics(totalDataPoints, p8sQueryResult)
	}

	totalDataPointsFetched.WithLabelValues(namespace, CPUUtilizationDataPointsQuery, workload).Set(float64(len(totalDataPoints)))
	if totalDataPoints == nil {
		return nil, fmt.Errorf("unable to getCPUUtlizationDataPoints metrics from any of the prometheus instances")
	}
	totalDataPoints = ps.interpolateMissingDataPoints(totalDataPoints, step)
	return totalDataPoints, nil
}

func aggregateMetrics(dataPoints1 []DataPoint, dataPoints2 []DataPoint) []DataPoint {
	var mergedDatapoints []DataPoint
	index1, index2 := 0, 0

	for index1 < len(dataPoints1) && index2 < len(dataPoints2) {
		dp1 := dataPoints1[index1]
		dp2 := dataPoints2[index2]

		if dp1.Timestamp.Before(dp2.Timestamp) {
			mergedDatapoints = append(mergedDatapoints, dp1)
			index1++
		} else if dp2.Timestamp.Before(dp1.Timestamp) {
			mergedDatapoints = append(mergedDatapoints, dp2)
			index2++
		} else {
			mergedDatapoints = append(mergedDatapoints, DataPoint{
				Timestamp: dp1.Timestamp,
				Value:     math.Max(dp1.Value, dp2.Value),
			})
			index1++
			index2++
		}
	}
	if index1 < len(dataPoints1) {
		mergedDatapoints = append(mergedDatapoints, dataPoints1[index1:]...)
	}

	if index2 < len(dataPoints2) {
		mergedDatapoints = append(mergedDatapoints, dataPoints2[index2:]...)
	}

	return mergedDatapoints
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

	resultChanLength := len(ps.api) + 5 //Added some buffer
	resultChan := make(chan []DataPoint, resultChanLength)
	var wg sync.WaitGroup

	var totalDataPoints []DataPoint
	if ps.api == nil {
		return nil, fmt.Errorf("no apiurl for executing prometheus query")
	}
	for _, pi := range ps.api {

		wg.Add(1)
		go func(pi PrometheusInstance) {
			defer wg.Done()
			p8sQueryStartTime := time.Now()
			result, err := ps.rangeQuerySplitter.QueryRangeByInterval(ctx, pi.apiUrl, query, start, end, step)
			if err != nil {
				ps.logger.Error(err, "failed to execute Prometheus query", "Instance", pi.address)
				logP8sMetrics(p8sQueryStartTime, namespace, BreachDataPointsQuery, pi.address, workload, -1, 0)
				resultChan <- nil
				return
			}
			if result.Type() != model.ValMatrix {
				ps.logger.Error(fmt.Errorf("unexpected result type: %v", result.Type()), "Result Type Error", "Instance", pi.address)
				logP8sMetrics(p8sQueryStartTime, namespace, BreachDataPointsQuery, pi.address, workload, -1, 1)
				resultChan <- nil
				return
			}
			matrix := result.(model.Matrix)
			if len(matrix) != 1 {
				// if no datapoints are returned which satisfy the query it can be considered that there's no breach to redLineUtilization
				ps.logger.V(2).Info("no Breach dataPoints found with the p8s instance", "Instance", pi.address)
				logP8sMetrics(p8sQueryStartTime, namespace, BreachDataPointsQuery, pi.address, workload, 0, 1)
				resultChan <- nil
				return
			}
			var dataPoints []DataPoint
			for _, sample := range matrix[0].Values {
				datapoint := DataPoint{sample.Timestamp.Time(), float64(sample.Value)}
				if !sample.Timestamp.Time().IsZero() {
					dataPoints = append(dataPoints, datapoint)
				}
			}
			logP8sMetrics(p8sQueryStartTime, namespace, BreachDataPointsQuery, pi.address, workload, len(dataPoints), 1)
			sort.SliceStable(dataPoints, func(i, j int) bool {
				return dataPoints[i].Timestamp.Before(dataPoints[j].Timestamp)
			})

			resultChan <- dataPoints
		}(pi)
	}

	wg.Wait()
	close(resultChan)

	for p8sQueryResult := range resultChan {
		totalDataPoints = aggregateMetrics(totalDataPoints, p8sQueryResult)
	}

	totalDataPointsFetched.WithLabelValues(namespace, BreachDataPointsQuery, workload).Set(float64(len(totalDataPoints)))
	if totalDataPoints == nil {
		// if no datapoints are returned which satisfy the query it can be considered that there's no breach to redLineUtilization
		ps.logger.Info("no Breach dataPoints found in any of the p8s instance", "Namespace", namespace, "Workload", workload)
		return nil, nil
	}
	ps.logger.Info("Breach dataPoints found..", "Namespace", namespace, "Workload", workload)
	return totalDataPoints, nil
}

// RangeQuerySplitter splits a given queryRange into multiple range queries of width splitInterval. This is done to
// avoid loading too many samples into P8s memory.
type RangeQuerySplitter struct {
	splitInterval time.Duration
}

func NewRangeQuerySplitter(splitInterval time.Duration) *RangeQuerySplitter {
	return &RangeQuerySplitter{splitInterval: splitInterval}
}
func (rqs *RangeQuerySplitter) QueryRangeByInterval(ctx context.Context,
	api v1.API,
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
			partialResult, _, err := api.QueryRange(ctx, query, splitRange)
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

	podBootstrapTime := 0.0
	if ps.api == nil {
		return 0.0, fmt.Errorf("no apiurl for executing prometheus query")
	}
	for _, pi := range ps.api {
		result, _, err := pi.apiUrl.Query(ctx, query, time.Now())

		if err != nil {
			ps.logger.Error(err, "failed to execute Prometheus query", "Instance", pi.address)
			continue
		}
		if result.Type() != model.ValVector {
			ps.logger.Error(fmt.Errorf("unexpected result type: %v", result.Type()), "Result Type Error", "Instance", pi.address)
			continue
		}

		matrix := result.(model.Vector)
		if len(matrix) != 1 {
			ps.logger.Error(fmt.Errorf("unexpected no of time series: %v", len(matrix)), "Zero Datapoints Error", "Instance", pi.address)
			continue
		}

		podBootstrapTime = math.Max(podBootstrapTime, float64(matrix[0].Value))
	}
	if podBootstrapTime == 0.0 {
		return 0.0, fmt.Errorf("unable to getPodReadyLatency metrics from any of the prometheus instances")
	}
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

func logP8sMetrics(p8sQueryStartTime time.Time, namespace string, query string, address string, workload string, dataPointsLength int, p8sInstanceSuccessfullyQueried int) {
	p8sQueryLatency := time.Since(p8sQueryStartTime).Seconds()
	prometheusQueryLatency.WithLabelValues(namespace, query, address, workload).Observe(p8sQueryLatency)
	dataPointsFetched.WithLabelValues(namespace, query, address, workload).Set(float64(dataPointsLength))
	p8sInstanceQueried.WithLabelValues(namespace, query, address, workload).Set(float64(p8sInstanceSuccessfullyQueried))
}
