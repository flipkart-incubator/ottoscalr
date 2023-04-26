package reco

import (
	"context"
	"errors"
	v1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"log"
	"math"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type RecommendationWorkflow interface {
	Execute(ctx context.Context, wm WorkloadMeta) (*v1alpha1.HPAConfiguration, *v1alpha1.HPAConfiguration, *Policy, error)
}

type Recommender interface {
	Recommend(wm WorkloadMeta) (*v1alpha1.HPAConfiguration, error)
}

type RecommendationWorkflowImpl struct {
	Recommender     Recommender
	PolicyIterators map[string]PolicyIterator
	logger          logr.Logger
}

type WorkloadMeta struct {
	metav1.TypeMeta
	Name      string
	Namespace string
}

type RecoWorkflowBuilder RecommendationWorkflowImpl

func (b *RecoWorkflowBuilder) AddRecommender(r Recommender) *RecoWorkflowBuilder {
	if b.Recommender == nil {
		b.Recommender = r
		return b
	}
	log.Println("Only one recommender must be added. There's already one configured so ignoring this one.")
	return b
}

func (b *RecoWorkflowBuilder) AddPolicyIterator(name string, p PolicyIterator) *RecoWorkflowBuilder {
	if b.PolicyIterators == nil {
		b.PolicyIterators = make(map[string]PolicyIterator)
		b.PolicyIterators[name] = p
	} else if _, ok := b.PolicyIterators[name]; !ok {
		b.PolicyIterators[name] = p
	}
	return b
}

func (b *RecoWorkflowBuilder) WithLogger(log logr.Logger) *RecoWorkflowBuilder {
	var zeroValLogger logr.Logger
	if b.logger == zeroValLogger {
		b.logger = log
	}
	return b
}

func (b *RecoWorkflowBuilder) Build() RecommendationWorkflow {
	var zeroValLogger logr.Logger
	if b.logger == zeroValLogger {
		b.logger = zap.New()
	}
	return &RecommendationWorkflowImpl{
		Recommender:     b.Recommender,
		PolicyIterators: b.PolicyIterators,
		logger:          b.logger,
	}
}

func NewRecommendationWorkflowBuilder() *RecoWorkflowBuilder {
	return &RecoWorkflowBuilder{}
}

type MockRecommender struct {
	Min       int
	Threshold int
	Max       int
}

func (r *MockRecommender) Recommend(wm WorkloadMeta) (*v1alpha1.HPAConfiguration, error) {
	return &v1alpha1.HPAConfiguration{
		Min:               r.Min,
		Max:               r.Max,
		TargetMetricValue: r.Threshold,
	}, nil
}

func (rw *RecommendationWorkflowImpl) Execute(ctx context.Context, wm WorkloadMeta) (*v1alpha1.HPAConfiguration, *v1alpha1.HPAConfiguration, *Policy, error) {
	if rw.Recommender == nil {
		return nil, nil, nil, errors.New("No recommenders configured in the workflow.")
	}
	recoConfig, err := rw.Recommender.Recommend(wm)
	if err != nil {
		rw.logger.Error(err, "Error while generating recommendation")
		return nil, nil, nil, errors.New("Unable to generate recommendation")
	}
	var nextPolicy *Policy
	for i, pi := range rw.PolicyIterators {
		rw.logger.V(0).Info("Running policy iterator", "iterator", i)
		p, err := pi.NextPolicy(wm)
		if err != nil {
			rw.logger.Error(err, "Error while generating recommendation")
			return nil, nil, nil, err
		}
		rw.logger.V(0).Info("Next Policy recommended by PI", "iterator", i, "policy", p)

		nextPolicy = pickSafestPolicy(nextPolicy, p)
		rw.logger.V(0).Info("Next Policy after applying PI", "iterator", i, "policy", nextPolicy)

	}

	nextConfig := generateNextRecoConfig(recoConfig, nextPolicy, wm)
	return nextConfig, recoConfig, nextPolicy, nil
}

func generateNextRecoConfig(config *v1alpha1.HPAConfiguration, policy *Policy, wm WorkloadMeta) *v1alpha1.HPAConfiguration {
	if shouldApplyReco(config, policy) {
		return config
	} else {
		recoConfig, _ := createRecoConfigFromPolicy(policy, config, wm)
		return recoConfig
	}
}

func createRecoConfigFromPolicy(policy *Policy, recoConfig *v1alpha1.HPAConfiguration, wm WorkloadMeta) (*v1alpha1.HPAConfiguration, error) {
	return &v1alpha1.HPAConfiguration{
		Min:               recoConfig.Max - int(math.Ceil(float64(policy.MinReplicaPercentageCut*(recoConfig.Max-recoConfig.Min)/100))),
		Max:               recoConfig.Max,
		TargetMetricValue: policy.TargetUtilization,
	}, nil
}

// Determines whether the recommendation should take precedence over the nextPolicy
func shouldApplyReco(config *v1alpha1.HPAConfiguration, policy *Policy) bool {
	if policy == nil {
		return true
	}
	// Returns true if the reco is safer than the policy
	if policy.MinReplicaPercentageCut == 100 && config.TargetMetricValue < policy.TargetUtilization {
		return true
	} else {
		return false
	}
}

func pickSafestPolicy(p1, p2 *Policy) *Policy {
	// if either or both of the policies are nil
	if p1 == nil && p2 != nil {
		return p2
	} else if p2 == nil && p1 != nil {
		return p1
	} else if p1 == nil && p2 == nil {
		return nil
	}

	if p1.RiskIndex <= p2.RiskIndex {
		return p1
	} else {
		return p2
	}
}
