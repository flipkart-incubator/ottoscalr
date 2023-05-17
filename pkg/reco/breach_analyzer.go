package reco

import (
	"context"
	"github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	"github.com/flipkart-incubator/ottoscalr/pkg/metrics"
	"github.com/flipkart-incubator/ottoscalr/pkg/policy"
	"github.com/flipkart-incubator/ottoscalr/pkg/trigger"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

type BreachAnalyzer struct {
	store    policy.Store
	scraper  metrics.Scraper
	breachFn func(ctx context.Context, start, end time.Time, workloadType string,
		workload types.NamespacedName,
		metricScraper metrics.Scraper,
		cpuRedLine float64,
		metricStep time.Duration) (bool, error)
	client     client.Client
	cpuRedline float64
	metricStep time.Duration
}

func NewBreachAnalyzer(k8sClient client.Client, scraper metrics.Scraper, cpuRedline float64, metricStep time.Duration) (*BreachAnalyzer, error) {
	return &BreachAnalyzer{
		store:      policy.NewPolicyStore(k8sClient),
		scraper:    scraper,
		breachFn:   trigger.HasBreached,
		client:     k8sClient,
		cpuRedline: cpuRedline,
		metricStep: metricStep,
	}, nil
}

func (pi *BreachAnalyzer) NextPolicy(ctx context.Context, wm WorkloadMeta) (*Policy, error) {
	logger := log.FromContext(ctx)
	currentPolicyReco := &v1alpha1.PolicyRecommendation{}
	if err := pi.client.Get(ctx, types.NamespacedName{Name: wm.Name, Namespace: wm.Namespace}, currentPolicyReco); err != nil {
		logger.V(0).Error(err, "Error while fetching policy reco", "workload", wm)
		return nil, err
	}

	if len(currentPolicyReco.Spec.Policy) == 0 {
		logger.V(0).Info("Empty policy in policy reco. Falling back to no-op.")
		return nil, nil
	}
	end := time.Now()
	start := currentPolicyReco.Spec.GeneratedAt.Time
	breached, err := pi.breachFn(ctx, start, end, wm.Kind, types.NamespacedName{
		Namespace: wm.Namespace,
		Name:      wm.Name,
	}, pi.scraper, pi.cpuRedline, pi.metricStep)
	if err != nil {
		logger.V(0).Error(err, "Error running breach detector")
		return nil, err
	}
	if breached {
		currentPolicyReco := &v1alpha1.PolicyRecommendation{}
		if err2 := pi.client.Get(ctx, types.NamespacedName{Name: wm.Name, Namespace: wm.Namespace}, currentPolicyReco); client.IgnoreNotFound(err2) != nil {
			logger.V(0).Error(err2, "Error while fetching policy reco", "workload", wm)
			return nil, err2
		}
		policy, err3 := pi.store.GetPreviousPolicyByName(currentPolicyReco.Spec.Policy)
		if err3 != nil {
			logger.V(0).Error(err3, "Error fetching the previous policy. Falling back to no-op.")
			return nil, nil
		}
		return PolicyFromCR(policy), nil
	}
	return nil, nil
}

func (pi *BreachAnalyzer) GetName() string {
	return "BreachAnalyzer"
}
