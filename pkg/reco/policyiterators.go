package reco

import (
	"context"
	"errors"
	"github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	"github.com/flipkart-incubator/ottoscalr/pkg/policy"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

type Policy struct {
	Name                    string `json:"name"`
	RiskIndex               int    `json:"riskIndex"`
	MinReplicaPercentageCut int    `json:"minReplicaPercentageCut"`
	TargetUtilization       int    `json:"targetUtilization"`
}

type PolicyIterator interface {
	NextPolicy(ctx context.Context, wm WorkloadMeta) (*Policy, error)
	GetName() string
}

type PolicyIteratorImpl struct {
	store policy.Store
}

type DefaultPolicyIterator PolicyIteratorImpl

func NewDefaultPolicyIterator(k8sClient client.Client) *DefaultPolicyIterator {
	return &DefaultPolicyIterator{
		store: policy.NewPolicyStore(k8sClient),
	}
}

func (pi *DefaultPolicyIterator) NextPolicy(ctx context.Context, wm WorkloadMeta) (*Policy, error) {
	logger := log.FromContext(ctx)
	policy, err := pi.store.GetDefaultPolicy()
	if err != nil {
		logger.V(0).Error(err, "Error fetching default policy.")
		return nil, err
	}
	return &Policy{
		Name:                    policy.Name,
		RiskIndex:               policy.Spec.RiskIndex,
		MinReplicaPercentageCut: policy.Spec.MinReplicaPercentageCut,
		TargetUtilization:       policy.Spec.TargetUtilization,
	}, nil
}

func (pi *DefaultPolicyIterator) GetName() string {
	return "DefaultPolicy"
}

type AgingPolicyIterator struct {
	store  policy.Store
	client client.Client
	Age    time.Duration
}

func NewAgingPolicyIterator(k8sClient client.Client, age time.Duration) *AgingPolicyIterator {
	return &AgingPolicyIterator{
		store:  policy.NewPolicyStore(k8sClient),
		client: k8sClient,
		Age:    age,
	}
}

func (pi *AgingPolicyIterator) NextPolicy(ctx context.Context, wm WorkloadMeta) (*Policy, error) {
	logger := log.FromContext(ctx)
	policyreco := &v1alpha1.PolicyRecommendation{}
	pi.client.Get(context.TODO(), types.NamespacedName{
		Namespace: wm.Namespace,
		Name:      wm.Name,
	}, policyreco)

	expired, err := isAgeBeyondExpiry(policyreco, pi.Age)
	if err != nil {
		return nil, err
	}

	// If the current policy reco is not set return the safest policy
	if len(policyreco.Spec.Policy) == 0 {

		safestPolicy, err := pi.store.GetSafestPolicy()
		if err != nil {
			return nil, err
		}
		logger.V(0).Error(err, "No policy has been configured. Returning safest policy.", "policy", safestPolicy.Name)
		return PolicyFromCR(safestPolicy), nil
	}

	currentAppliedPolicy, err := pi.store.GetPolicyByName(policyreco.Spec.Policy)
	if err != nil {
		return nil, err
	}

	if !expired {
		logger.V(0).Info("Policy hasn't expired yet")
		return PolicyFromCR(currentAppliedPolicy), nil
	}

	nextPolicy, err := pi.store.GetNextPolicyByName(policyreco.Spec.Policy)
	if err != nil {
		if IsLastPolicy(err) {
			return PolicyFromCR(currentAppliedPolicy), nil
		}
		return nil, err
	}

	return PolicyFromCR(nextPolicy), nil
}

func (pi *AgingPolicyIterator) GetName() string {
	return "Aging"
}

func IsLastPolicy(err error) bool {
	return errors.Is(err, policy.NO_NEXT_POLICY_FOUND_ERR)
}

func PolicyFromCR(policy *v1alpha1.Policy) *Policy {
	if policy == nil {
		return nil
	}
	return &Policy{
		Name:                    policy.Name,
		RiskIndex:               policy.Spec.RiskIndex,
		MinReplicaPercentageCut: policy.Spec.MinReplicaPercentageCut,
		TargetUtilization:       policy.Spec.TargetUtilization,
	}
}

func isAgeBeyondExpiry(policyreco *v1alpha1.PolicyRecommendation, age time.Duration) (bool, error) {
	if policyreco == nil || policyreco.Spec.TransitionedAt.IsZero() {
		// For new policyreco which haven't been touched by the registrar return false
		return false, nil
	}
	// if now() is still before last reco transitionedAt + expiry age
	if policyreco.Spec.TransitionedAt.Add(age).After(metav1.Now().Time) {
		return false, nil
	} else {
		return true, nil
	}
}

type BreachAnalyzer struct {
}

func NewBreachAnalayzer() (*BreachAnalyzer, error) {
	return nil, nil
}

func (pi *BreachAnalyzer) NextPolicy(ctx context.Context, wm WorkloadMeta) (*Policy, error) {
	//	TODO: if breach then currenPolicy - 1; else currentPolicy
	return nil, nil
}

func (pi *BreachAnalyzer) GetName() string {
	return "BreachAnalyzer"
}
