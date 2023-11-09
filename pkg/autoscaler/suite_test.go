package autoscaler

import (
	"context"
	"testing"

	"github.com/flipkart-incubator/ottoscalr/pkg/testutil"
	kedaapi "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	cfg                *rest.Config
	k8sClient          client.Client
	ctx                context.Context
	cancel             context.CancelFunc
	scaledObjectClient AutoscalerClient
	hpaClient          AutoscalerClient
	hpaClientV2        AutoscalerClient
)

func TestAutoscalerClient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Autoscaler Client Suite")
}

var _ = BeforeSuite(func() {
	logger := zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true))
	logf.SetLogger(logger)
	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	var err error
	// cfg is defined in this file globally.
	cfg, ctx, cancel = testutil.SetupSingletonEnvironment()
	Expect(cfg).NotTo(BeNil())

	err = kedaapi.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:Scheme

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0.0.0.0:0",
	})
	Expect(err).ToNot(HaveOccurred())
	k8sClient = k8sManager.GetClient()
	Expect(k8sClient).NotTo(BeNil())

	scaledObjectClient = NewScaledobjectClient(k8sManager.GetClient())
	hpaClient = NewHPAClient(k8sManager.GetClient())
	hpaClientV2 = NewHPAClientV2(k8sManager.GetClient())
	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testutil.TeardownSingletonEnvironment()
	Expect(err).NotTo(HaveOccurred())
})
