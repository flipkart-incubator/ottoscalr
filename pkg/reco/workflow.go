package reco

import (
	"context"
	"errors"
	v1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	golog "log"
	"math"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type RecommendationWorkflow interface {
	Execute(ctx context.Context, wm WorkloadMeta) (*v1alpha1.HPAConfiguration, *v1alpha1.HPAConfiguration, *Policy, error)
}

type Recommender interface {
	Recommend(ctx context.Context, wm WorkloadMeta) (*v1alpha1.HPAConfiguration, error)
}

type RecommendationWorkflowImpl struct {
	recommender     Recommender
	policyIterators map[string]PolicyIterator
	logger          logr.Logger
}

type WorkloadMeta struct {
	metav1.TypeMeta
	Name      string
	Namespace string
}

type RecoWorkflowBuilder RecommendationWorkflowImpl

func (b *RecoWorkflowBuilder) WithRecommender(r Recommender) *RecoWorkflowBuilder {
	if b.recommender == nil {
		b.recommender = r
		return b
	}
	golog.Println("Only one recommender must be added. There's already one configured so ignoring this one.")
	return b
}

func (b *RecoWorkflowBuilder) WithPolicyIterator(p PolicyIterator) *RecoWorkflowBuilder {
	if b.policyIterators == nil {
		b.policyIterators = make(map[string]PolicyIterator)
		b.policyIterators[p.GetName()] = p
	} else if _, ok := b.policyIterators[p.GetName()]; !ok {
		b.policyIterators[p.GetName()] = p
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

func (b *RecoWorkflowBuilder) Build() (RecommendationWorkflow, error) {
	var zeroValLogger logr.Logger
	if b.logger == zeroValLogger {
		b.logger = zap.New()
	}
	if b.recommender == nil && b.policyIterators == nil {
		return nil, errors.New("Both recommender and policy iterators can't be nil")
	}
	return &RecommendationWorkflowImpl{
		recommender:     b.recommender,
		policyIterators: b.policyIterators,
		logger:          b.logger,
	}, nil
}

func NewRecommendationWorkflowBuilder() *RecoWorkflowBuilder {
	return &RecoWorkflowBuilder{}
}

func (rw *RecommendationWorkflowImpl) Execute(ctx context.Context, wm WorkloadMeta) (*v1alpha1.HPAConfiguration, *v1alpha1.HPAConfiguration, *Policy, error) {
	ctx = log.IntoContext(ctx, rw.logger)
	rw.logger.V(0).Info("Workload Meta", "workload", wm)
	if rw.recommender == nil {
		return nil, nil, nil, errors.New("No recommenders configured in the workflow.")
	}
	targetRecoConfig, err := rw.recommender.Recommend(ctx, wm)
	if err != nil {
		rw.logger.Error(err, "Error while generating recommendation")
		return nil, nil, nil, errors.New("Unable to generate recommendation")
	}
	var nextPolicy *Policy
	for i, pi := range rw.policyIterators {
		rw.logger.V(0).Info("Running policy iterator", "iterator", i)
		p, err := pi.NextPolicy(ctx, wm)
		if err != nil {
			rw.logger.Error(err, "Error while generating recommendation")
			return nil, nil, nil, err
		}

		if p == nil {
			rw.logger.V(0).Info("Skipping this PI since it has recommended nil policy (no-op)", "iterator", i)
			continue
		}

		rw.logger.V(0).Info("Next Policy recommended by PI", "iterator", i, "policy", p)
		nextPolicy = pickSafestPolicy(nextPolicy, p)
		rw.logger.V(0).Info("Next Policy after applying PI", "iterator", i, "policy", nextPolicy)

	}

	nextConfig, policyToApply := generateNextRecoConfig(targetRecoConfig, nextPolicy, wm)
	return nextConfig, targetRecoConfig, policyToApply, nil
}

func generateNextRecoConfig(config *v1alpha1.HPAConfiguration, policy *Policy, wm WorkloadMeta) (*v1alpha1.HPAConfiguration, *Policy) {
	if shouldApplyReco(config, policy) {
		return config, nil
	} else {
		recoConfig, _ := createRecoConfigFromPolicy(policy, config, wm)
		return recoConfig, policy
	}
}

func createRecoConfigFromPolicy(policy *Policy, recoConfig *v1alpha1.HPAConfiguration, wm WorkloadMeta) (*v1alpha1.HPAConfiguration, error) {
	if policy == nil || recoConfig == nil {
		return nil, errors.New("Policy or reco config supplied is nil")
	}
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
	if config == nil {
		return false
	}

	// Returns true if the reco is safer than the policy
	if policy.MinReplicaPercentageCut == 100 && config.TargetMetricValue < policy.TargetUtilization {
		return true
	} else { // Policy is safer than the reco or we've exhausted the list of policies and the last (most aggressive) policy is safer than the reco config
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
