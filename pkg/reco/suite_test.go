package reco

import (
	"context"
	rolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	ottoscaleriov1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	"github.com/flipkart-incubator/ottoscalr/pkg/metrics"
	"github.com/flipkart-incubator/ottoscalr/pkg/testutil"
	"github.com/go-logr/logr"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	cfg       *rest.Config
	k8sClient client.Client
	ctx       context.Context
	cancel    context.CancelFunc

	logger logr.Logger

	currentReplicas   = 2
	redLineUtil       = float32(80)
	metricWindow      = 7 * 24 * time.Hour
	metricStep        = 5 * time.Minute
	minTarget         = 50
	maxTarget         = 200
	fakeScraper       metrics.Scraper
	recommender       *CpuUtilizationBasedRecommender
	workloadNamespace = "test-namespace"
	workloadName      = "test-workload"
)

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

	recommender = NewCpuUtilizationBasedRecommender(k8sClient, ctx, currentReplicas, redLineUtil,
		metricWindow, fakeScraper, metricStep, minTarget, maxTarget, logger)

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
