package metrics

import (
	"context"
	"errors"
	"fmt"
	"github.com/golang/mock/gomock"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"ottoscalr/mocks"
	"testing"
	"time"
)

func TestGetAverageCPUUtilizationByWorkload(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock API client
	mockAPI := mocks.NewMockAPI(ctrl)

	// Create a PrometheusScraper with the mock API client
	ps := &PrometheusScraper{
		api: mockAPI,
	}

	// Set up the test case
	ctx := context.Background()
	namespace := "test-namespace"
	workloadType := "test-type"
	workloadName := "test-name"
	start := time.Now()

	// Create a mock query result
	mockResult := &model.Vector{
		&model.Sample{
			Metric: model.Metric{
				"workload":     "test-workload",
				"workloadType": "test-type",
			},
			Value:     123.45,
			Timestamp: model.TimeFromUnix((time.Now().Add(1 * time.Hour)).Unix()),
		},
	}
	query := fmt.Sprintf("sum(node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate) "+
		"by (workload, workload_type) * on(namespace, pod) "+
		"group_left(workload, workload_type) namespace_workload_pod:kube_pod_owner:relabel namespace=\"%s\", "+
		"workload=\"%s\", workload_type=\"%s\"", namespace, workloadName, workloadType)

	// Set up the mock API to return the mock query result
	mockAPI.EXPECT().Query(gomock.Any(), query, gomock.Any()).Return(mockResult, nil, nil)

	// Call the GetAverageCPUUtilizationByWorkload method
	dataPoints, err := ps.GetAverageCPUUtilizationByWorkload(ctx, namespace, workloadType, workloadName, start)

	// Check that the result is as expected
	assert.NoError(t, err)
	assert.Equal(t, []float64{123.45}, dataPoints)
}

func TestGetAverageCPUUtilizationByWorkload_Negative(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock API client
	mockAPI := mocks.NewMockAPI(ctrl)

	// Create a PrometheusScraper with the mock API client
	ps := &PrometheusScraper{
		api: mockAPI,
	}

	// Set up the test case
	ctx := context.Background()
	namespace := "test-namespace"
	workloadType := "test-type"
	workloadName := "test-name"
	start := time.Now()

	query := fmt.Sprintf("sum(node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate) "+
		"by (workload, workload_type) * on(namespace, pod) "+
		"group_left(workload, workload_type) namespace_workload_pod:kube_pod_owner:relabel namespace=\"%s\", "+
		"workload=\"%s\", workload_type=\"%s\"", namespace, workloadName, workloadType)

	// Set up the mock API to return the mock query result
	mockAPI.EXPECT().Query(gomock.Any(), query, gomock.Any()).Return(nil, nil, errors.New("test error"))

	// Call the GetAverageCPUUtilizationByWorkload method
	dataPoints, err := ps.GetAverageCPUUtilizationByWorkload(ctx, namespace, workloadType, workloadName, start)

	// Check that the error is as expected
	assert.Error(t, err)
	assert.Nil(t, dataPoints)
}
