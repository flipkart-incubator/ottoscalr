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
	GetAverageCPUUtilizationByWorkload(ctx context.Context,
		namespace,
		workloadType,
		workload string, start time.Time) ([]float64, error)
}

// PrometheusScraper is a Scraper implementation that scrapes metrics data from Prometheus.
type PrometheusScraper struct {
	api v1.API
}

// NewPrometheusScraper returns a new PrometheusScraper instance.

func NewPrometheusScraper(apiURL string) (*PrometheusScraper, error) {
	client, err := api.NewClient(api.Config{
		Address: apiURL,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating Prometheus client: %v", err)
	}

	promAPI := v1.NewAPI(client)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, _, err = promAPI.LabelValues(ctx, "__name__", []string{}, time.Now(), time.Now())
	if err != nil {
		return nil, fmt.Errorf("error connecting to Prometheus: %v", err)
	}

	return &PrometheusScraper{api: promAPI}, nil
}

// GetAverageCPUUtilizationByWorkload returns the average CPU utilization for the given workload type and name in the
// specified namespace, starting from the given time.
func (ps *PrometheusScraper) GetAverageCPUUtilizationByWorkload(ctx context.Context,
	namespace string,
	workloadType string,
	workloadName string,
	start time.Time) ([]float64, error) {
	query := fmt.Sprintf("sum(node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate) "+
		"by (workload, workload_type) * on(namespace, pod) "+
		"group_left(workload, workload_type) namespace_workload_pod:kube_pod_owner:relabel namespace=\"%s\", "+
		"workload=\"%s\", workload_type=\"%s\"", namespace, workloadName, workloadType)
	result, _, err := ps.api.Query(ctx, query, start)
	if err != nil {
		return nil, fmt.Errorf("failed to execute Prometheus query: %v", err)
	}
	if result.Type() != model.ValVector {
		return nil, fmt.Errorf("unexpected result type: %v", result.Type())
	}
	vector := *result.(*model.Vector)
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
