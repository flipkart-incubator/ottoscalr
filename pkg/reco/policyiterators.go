package reco

import (
	"context"
	"errors"
	"time"

	"github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	"github.com/flipkart-incubator/ottoscalr/pkg/policy"
	"github.com/flipkart-incubator/ottoscalr/pkg/registry"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	p8smetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	DefaultPolicyAnnotation = "ottoscalr.io/default-policy"
)

var (
	agedPolicyCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{Name: "policyage_expired_counter",
			Help: "Number of policyrecos reconcile errored counter"}, []string{"namespace", "policyreco", "workloadKind", "workload"},
	)
)

func init() {
	p8smetrics.Registry.MustRegister(agedPolicyCounter)
}

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

type DefaultPolicyIterator struct {
	store           policy.Store
	client          client.Client
	clientsRegistry registry.DeploymentClientRegistry
}

func NewDefaultPolicyIterator(k8sClient client.Client, clientsRegistry registry.DeploymentClientRegistry) *DefaultPolicyIterator {
	return &DefaultPolicyIterator{
		store:           policy.NewPolicyStore(k8sClient),
		client:          k8sClient,
		clientsRegistry: clientsRegistry,
	}
}

func (pi *DefaultPolicyIterator) NextPolicy(ctx context.Context, wm WorkloadMeta) (*Policy, error) {
	logger := log.FromContext(ctx)

	//Check if the workload has a default policy annotation set.
	//If yes, return that policy as default policy, else return global default policy.
	deploymentClient, err := pi.clientsRegistry.GetObjectClient(wm.Kind)
	if err != nil {
		logger.V(0).Error(err, "Error fetching deployment client for this workload", "workload", wm)
		return pi.GetGlobalDefaultPolicy(ctx)
	}
	workload, err := deploymentClient.GetObject(wm.Namespace, wm.Name)
	if err != nil {
		logger.V(0).Error(err, "Error fetching this workload", "workload", wm)
		return pi.GetGlobalDefaultPolicy(ctx)
	}
	defaultPolicyName, ok := workload.GetAnnotations()[DefaultPolicyAnnotation]
	if ok {
		logger.V(0).Info("Workload has default policy annotation set", "workload", wm, "defaultPolicy", defaultPolicyName)
		policy, err := pi.store.GetPolicyByName(defaultPolicyName)
		if err == nil {
			return &Policy{
				Name:                    policy.Name,
				RiskIndex:               policy.Spec.RiskIndex,
				MinReplicaPercentageCut: policy.Spec.MinReplicaPercentageCut,
				TargetUtilization:       policy.Spec.TargetUtilization,
			}, nil
		}
		logger.V(0).Error(err, "Error fetching this policy.", "workload", wm, "defaultPolicy", defaultPolicyName)
		logger.V(0).Info("Falling back to global default policy")
	}
	return pi.GetGlobalDefaultPolicy(ctx)
}

func (pi *DefaultPolicyIterator) GetGlobalDefaultPolicy(ctx context.Context) (*Policy, error) {
	logger := log.FromContext(ctx)
	policy, err := pi.store.GetDefaultPolicy()
	if err != nil {
		logger.V(0).Error(err, "Error fetching default policy.")
		return nil, nil
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

	logger.V(0).Info("Workload Meta", "workload", wm)
	logger.V(0).Info("Policy Reco CR", "policyreco", policyreco)
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
		if errors.Is(err, policy.NoPolicyFoundErr) {
			defaultPolicy, err2 := pi.store.GetSafestPolicy()
			if err2 != nil {
				return nil, err2
			}
			return PolicyFromCR(defaultPolicy), nil
		}
		return nil, err
	}

	if !expired {
		logger.V(0).Info("Policy hasn't expired yet")
		return PolicyFromCR(currentAppliedPolicy), nil
	}

	agedPolicyCounter.WithLabelValues(wm.Namespace, policyreco.Name, wm.Kind, wm.Name).Inc()
	nextPolicy, err := pi.store.GetNextPolicyByName(policyreco.Spec.Policy)
	if err != nil {
		if policy.IsLastPolicy(err) {
			return PolicyFromCR(currentAppliedPolicy), nil
		}
		return nil, err
	}

	return PolicyFromCR(nextPolicy), nil
}

func (pi *AgingPolicyIterator) GetName() string {
	return "Aging"
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
