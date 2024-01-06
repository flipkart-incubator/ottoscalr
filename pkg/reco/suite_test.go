package reco

import (
	"context"
	"testing"
	"time"

	rolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	ottoscaleriov1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	"github.com/flipkart-incubator/ottoscalr/pkg/autoscaler"
	"github.com/flipkart-incubator/ottoscalr/pkg/metrics"
	"github.com/flipkart-incubator/ottoscalr/pkg/policy"
	"github.com/flipkart-incubator/ottoscalr/pkg/registry"
	"github.com/flipkart-incubator/ottoscalr/pkg/testutil"
	"github.com/go-logr/logr"
	kedaapi "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	cfg           *rest.Config
	k8sClient     client.Client
	fakeK8SClient client.Client
	ctx           context.Context
	cancel        context.CancelFunc

	logger                       logr.Logger
	redLineUtil                  = 0.85
	metricWindow                 = 1 * time.Hour
	metricStep                   = 5 * time.Minute
	minTarget                    = 10
	maxTarget                    = 60
	minPercentageMetricsRequired = 5
	fakeScraper                  metrics.Scraper
	fakeScraper1                 metrics.Scraper
	clientsRegistry              registry.DeploymentClientRegistry
	recommender                  *CpuUtilizationBasedRecommender
	recommender1                 *CpuUtilizationBasedRecommender
	recommender2                 *CpuUtilizationBasedRecommender
	recommender3                 *CpuUtilizationBasedRecommender
	fakeMetricsTransformer       []metrics.MetricsTransformer
	store                        *policy.PolicyStore
	policyAge                    = 1 * time.Second
	mockRecommender              *Recommender
	mockPolicyIterator           *PolicyIterator
	mockPolicy                   *Policy
)

var safestPolicy, policy1, policy2 *ottoscaleriov1alpha1.Policy

type FakeScraper struct {
	CPUDataPoints    []metrics.DataPoint
	BreachDataPoints []metrics.DataPoint
	WorkloadACL      time.Duration
}

func newFakeScraper(cpuDataPoints, breaches []metrics.DataPoint, acl time.Duration) *FakeScraper {
	return &FakeScraper{
		CPUDataPoints:    cpuDataPoints,
		BreachDataPoints: breaches,
		WorkloadACL:      acl,
	}
}

type MockRecommender struct {
	Min       int
	Threshold int
	Max       int
}

func (r *MockRecommender) Recommend(ctx context.Context, wm WorkloadMeta) (*ottoscaleriov1alpha1.HPAConfiguration, error) {
	return &ottoscaleriov1alpha1.HPAConfiguration{
		Min:               r.Min,
		Max:               r.Max,
		TargetMetricValue: r.Threshold,
	}, nil
}

type MockNoOpPI struct{}
type MockPI struct{}

func (pi *MockNoOpPI) NextPolicy(ctx context.Context, wm WorkloadMeta) (*Policy, error) {
	return nil, nil
}

func (pi *MockNoOpPI) GetName() string {
	return "no-op"
}

func (pi *MockPI) NextPolicy(ctx context.Context, wm WorkloadMeta) (*Policy, error) {
	return mockPolicy, nil
}

func (pi *MockPI) GetName() string {
	return "mockPI"
}

type FakeMetricsTransformer struct{}

func (fs *FakeScraper) GetAverageCPUUtilizationByWorkload(namespace,
	workload string,
	start time.Time,
	end time.Time,
	step time.Duration) ([]metrics.DataPoint, error) {
	return fs.CPUDataPoints, nil
}

func (fs *FakeScraper) GetCPUUtilizationBreachDataPoints(namespace,
	workloadType,
	workload string,
	redLineUtilization float64,
	start time.Time,
	end time.Time,
	step time.Duration) ([]metrics.DataPoint, error) {
	return fs.BreachDataPoints, nil
}
func (fs *FakeScraper) GetACLByWorkload(namespace,
	workload string) (time.Duration, error) {
	return fs.WorkloadACL, nil
}

func (fm *FakeMetricsTransformer) Transform(
	startTime time.Time, endTime time.Time, dataPoints []metrics.DataPoint) ([]metrics.DataPoint, error) {
	return dataPoints, nil
}

func TestPolicies(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Policy Suite")
}

var _ = BeforeSuite(func() {
	logger = zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true))
	logf.SetLogger(logger)
	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	var err error
	// cfg is defined in this file globally.
	cfg, ctx, cancel = testutil.SetupSingletonEnvironment()
	Expect(cfg).NotTo(BeNil())

	err = rolloutv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = ottoscaleriov1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = kedaapi.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:Scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0.0.0.0:0",
	})
	Expect(err).ToNot(HaveOccurred())

	err = k8sManager.GetFieldIndexer().IndexField(context.Background(), &kedaapi.ScaledObject{}, ScaledObjectField, func(obj client.Object) []string {
		scaledObject := obj.(*kedaapi.ScaledObject)
		if scaledObject.Spec.ScaleTargetRef.Name == "" {
			return nil
		}
		return []string{scaledObject.Spec.ScaleTargetRef.Name}
	})
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()

	fakeScraper = newFakeScraper([]metrics.DataPoint{
		{Timestamp: time.Now().Add(-10 * time.Minute), Value: 60},
		{Timestamp: time.Now().Add(-9 * time.Minute), Value: 80},
		{Timestamp: time.Now().Add(-8 * time.Minute), Value: 100},
		{Timestamp: time.Now().Add(-7 * time.Minute), Value: 50},
		{Timestamp: time.Now().Add(-6 * time.Minute), Value: 30},
	}, []metrics.DataPoint{{Timestamp: time.Now(), Value: 1.3}}, 5*time.Minute)

	fakeScraper1 = newFakeScraper([]metrics.DataPoint{
		{Timestamp: time.Now().Add(-10 * time.Minute), Value: 0},
		{Timestamp: time.Now().Add(-9 * time.Minute), Value: 500},
		{Timestamp: time.Now().Add(-8 * time.Minute), Value: 600},
		{Timestamp: time.Now().Add(-7 * time.Minute), Value: 600},
		{Timestamp: time.Now().Add(-6 * time.Minute), Value: 600},
	}, []metrics.DataPoint{{Timestamp: time.Now(), Value: 1.3}}, 5*time.Minute)

	fakeMetricsTransformer = append(fakeMetricsTransformer, &FakeMetricsTransformer{})

	clientsRegistry = *registry.NewDeploymentClientRegistryBuilder().
		WithK8sClient(k8sClient).
		WithCustomDeploymentClient(registry.NewDeploymentClient(k8sManager.GetClient())).
		WithCustomDeploymentClient(registry.NewRolloutClient(k8sManager.GetClient())).
		Build()

	var trueBool = true
	autoscalerClient := autoscaler.NewScaledobjectClient(k8sManager.GetClient(), &trueBool)

	recommender = NewCpuUtilizationBasedRecommender(k8sClient, redLineUtil,
		metricWindow, fakeScraper, fakeMetricsTransformer, metricStep, minTarget, maxTarget, minPercentageMetricsRequired, clientsRegistry, autoscalerClient, logger)

	recommender1 = NewCpuUtilizationBasedRecommender(k8sManager.GetClient(), redLineUtil,
		metricWindow, fakeScraper, fakeMetricsTransformer, metricStep, minTarget, maxTarget, minPercentageMetricsRequired, clientsRegistry, autoscalerClient, logger)

	recommender2 = NewCpuUtilizationBasedRecommender(k8sManager.GetClient(), redLineUtil,
		metricWindow, fakeScraper1, fakeMetricsTransformer, metricStep, minTarget, maxTarget, minPercentageMetricsRequired, clientsRegistry, autoscalerClient, logger)

	recommender3 = NewCpuUtilizationBasedRecommender(k8sManager.GetClient(), redLineUtil,
		28*24*time.Hour, fakeScraper1, fakeMetricsTransformer, 30*time.Second, minTarget, maxTarget, minPercentageMetricsRequired, clientsRegistry, autoscalerClient, logger)

	safestPolicy = &ottoscaleriov1alpha1.Policy{
		ObjectMeta: metav1.ObjectMeta{Name: "safest-policy"},
		Spec: ottoscaleriov1alpha1.PolicySpec{
			IsDefault:               false,
			RiskIndex:               1,
			MinReplicaPercentageCut: 80,
			TargetUtilization:       10,
		},
	}
	policy1 = &ottoscaleriov1alpha1.Policy{
		ObjectMeta: metav1.ObjectMeta{Name: "policy-1"},
		Spec: ottoscaleriov1alpha1.PolicySpec{
			IsDefault:               true,
			RiskIndex:               10,
			MinReplicaPercentageCut: 100,
			TargetUtilization:       15,
		},
	}
	policy2 = &ottoscaleriov1alpha1.Policy{
		ObjectMeta: metav1.ObjectMeta{Name: "policy-2"},
		Spec: ottoscaleriov1alpha1.PolicySpec{
			IsDefault:               false,
			RiskIndex:               20,
			MinReplicaPercentageCut: 100,
			TargetUtilization:       20,
		},
	}

	fakeK8SClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(safestPolicy, policy1, policy2).Build()

	store = policy.NewPolicyStore(fakeK8SClient)
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testutil.TeardownSingletonEnvironment()
	Expect(err).NotTo(HaveOccurred())
})
