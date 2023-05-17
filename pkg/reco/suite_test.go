package reco

import (
	"context"
	rolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	ottoscaleriov1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	"github.com/flipkart-incubator/ottoscalr/pkg/metrics"
	"github.com/flipkart-incubator/ottoscalr/pkg/policy"
	"github.com/flipkart-incubator/ottoscalr/pkg/testutil"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	cfg           *rest.Config
	k8sClient     client.Client
	fakeK8SClient client.Client
	ctx           context.Context
	cancel        context.CancelFunc

	logger logr.Logger

	redLineUtil  = 0.85
	metricWindow = 1 * time.Hour
	metricStep   = 5 * time.Minute
	minTarget    = 10
	maxTarget    = 60
	fakeScraper  metrics.Scraper
	recommender  *CpuUtilizationBasedRecommender
	store        *policy.PolicyStore
	policyAge    = 1 * time.Second
)

var safestPolicy, policy1, policy2 *ottoscaleriov1alpha1.Policy

type FakeScraper struct{}

func (fs *FakeScraper) GetAverageCPUUtilizationByWorkload(namespace,
	workload string,
	start time.Time,
	end time.Time,
	step time.Duration) ([]metrics.DataPoint, error) {
	dataPoints := []metrics.DataPoint{
		{Timestamp: time.Now().Add(-10 * time.Minute), Value: 60},
		{Timestamp: time.Now().Add(-9 * time.Minute), Value: 80},
		{Timestamp: time.Now().Add(-8 * time.Minute), Value: 100},
		{Timestamp: time.Now().Add(-7 * time.Minute), Value: 50},
		{Timestamp: time.Now().Add(-6 * time.Minute), Value: 30},
	}
	return dataPoints, nil
}

func (fs *FakeScraper) GetCPUUtilizationBreachDataPoints(namespace,
	workloadType,
	workload string,
	redLineUtilization float64,
	start time.Time,
	end time.Time,
	step time.Duration) ([]metrics.DataPoint, error) {
	datapoint := metrics.DataPoint{Timestamp: time.Now(), Value: 1.3}
	return []metrics.DataPoint{datapoint}, nil
}
func (fs *FakeScraper) GetACLByWorkload(namespace,
	workload string) (time.Duration, error) {
	return 5 * time.Minute, nil
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
	cfg, ctx, cancel = testutil.SetupEnvironment()
	Expect(cfg).NotTo(BeNil())

	err = rolloutv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = ottoscaleriov1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:Scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	fakeScraper = &FakeScraper{}

	recommender = NewCpuUtilizationBasedRecommender(k8sClient, redLineUtil,
		metricWindow, fakeScraper, metricStep, minTarget, maxTarget, logger)

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
	go func() {
		defer GinkgoRecover()
	}()
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testutil.TeardownEnvironment()
	Expect(err).NotTo(HaveOccurred())
})
