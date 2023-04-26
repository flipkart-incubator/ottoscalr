package reco

import (
	"context"
	"errors"
	"fmt"
	v1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"log"
	"math"
)

type RecommendationWorkflow interface {
	Execute(ctx context.Context, wm WorkloadMeta) (*v1alpha1.HPAConfiguration, error)
}

type Recommender interface {
	Recommend(wm WorkloadMeta) (*v1alpha1.HPAConfiguration, error)
}

// TODO(bharathguvvala): make metric scraper part of this struct
type RecommendationWorkflowImpl struct {
	Recommender     Recommender
	PolicyIterators map[string]PolicyIterator
}

type SerialRecomendationWorkflow struct {
	RecommendationWorkflowImpl
}

type WorkloadMeta struct {
	metav1.TypeMeta
	Name      string
	Namespace string
}

func NewRecommendationWorkflow() (*SerialRecomendationWorkflow, error) {
	return &SerialRecomendationWorkflow{
		RecommendationWorkflowImpl: RecommendationWorkflowImpl{
			Recommender: MockRecommender{
				min:       1,
				threshold: 20,
				max:       10,
			},
			PolicyIterators: nil,
		},
	}, nil
}

type MockRecommender struct {
	min       int
	threshold int
	max       int
}

func (r MockRecommender) Recommend(wm WorkloadMeta) (*v1alpha1.HPAConfiguration, error) {
	return &v1alpha1.HPAConfiguration{
		Min:               r.min,
		Max:               r.max,
		TargetMetricValue: r.threshold,
	}, nil

}

func (w WorkloadMeta) GetReplicas() (int, error) {
	//	TODO: query the k8s apiserver fetch the replicas; return a constant for now
	return 10, nil
}

func (rw *SerialRecomendationWorkflow) Execute(ctx context.Context, wm WorkloadMeta) (*v1alpha1.HPAConfiguration, *v1alpha1.HPAConfiguration, *Policy, error) {
	if rw.Recommender == nil {
		return nil, nil, nil, errors.New("No recommenders configured in the workflow.")
	}
	recoConfig, err := rw.Recommender.Recommend(wm)
	if err != nil {
		log.Println("Error while generating recommendation")
		// TODO: fallback
		return nil, nil, nil, errors.New("Unable to generate recommendation")
	}
	var nextPolicy *Policy
	for name, pi := range rw.PolicyIterators {
		p, err := pi.NextPolicy(wm)
		if err != nil {
			log.Println("Error while generating recommendation")
			return nil, nil, nil, errors.New(fmt.Sprintf("Unable to generate next policy from policy iterator %s", name))
		}
		nextPolicy = pickSafestPolicy(nextPolicy, p)
	}

	nextConfig := generateNextRecoConfig(recoConfig, nextPolicy, wm)
	return nextConfig, recoConfig, nextPolicy, nil
}

func generateNextRecoConfig(config *v1alpha1.HPAConfiguration, policy *Policy, wm WorkloadMeta) *v1alpha1.HPAConfiguration {
	if shouldApplyReco(config, policy) {
		return config
	} else {
		recoConfig, _ := createRecoConfigFromPolicy(policy, wm)
		return recoConfig
	}
}

func createRecoConfigFromPolicy(policy *Policy, wm WorkloadMeta) (*v1alpha1.HPAConfiguration, error) {
	replicas, err := wm.GetReplicas()
	if err != nil {
		return nil, errors.New("Error fetching replicas for workload")
	}
	return &v1alpha1.HPAConfiguration{
		Min:               int(math.Ceil(float64(policy.MinReplicaPercentageCut * replicas / 100))),
		Max:               replicas,
		TargetMetricValue: policy.TargetUtilization,
	}, nil
}

// Determines whether the recommendation should take precedence over the nextPolicy
func shouldApplyReco(config *v1alpha1.HPAConfiguration, policy *Policy) bool {
	if policy == nil {
		return true
	}
	// Returns true if the reco is safer than the next policy
	if policy.MinReplicaPercentageCut == 100 && config.TargetMetricValue < policy.TargetUtilization {
		return true
	} else {
		return false
	}
}

func pickSafestPolicy(p1, p2 *Policy) *Policy {
	if p1.RiskIndex <= p2.RiskIndex {
		return p1
	} else {
		return p2
	}
}
