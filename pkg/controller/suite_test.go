/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"errors"
	"time"

	rolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/flipkart-incubator/ottoscalr/pkg/autoscaler"
	"github.com/flipkart-incubator/ottoscalr/pkg/policy"
	"github.com/flipkart-incubator/ottoscalr/pkg/reco"
	"github.com/flipkart-incubator/ottoscalr/pkg/registry"
	"github.com/flipkart-incubator/ottoscalr/pkg/testutil"
	"github.com/flipkart-incubator/ottoscalr/pkg/trigger"
	kedaapi "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/net/context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"testing"

	ctrl "sigs.k8s.io/controller-runtime"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	ottoscaleriov1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	cfg        *rest.Config
	k8sClient  client.Client
	k8sClient1 client.Client
	ctx        context.Context
	cancel     context.CancelFunc

	queuedAllRecos                 = false
	queuedOneReco                  []bool
	recommender                    *MockRecommender
	deploymentTriggerControllerEnv *testutil.TestEnvironment
	clientsRegistry                registry.DeploymentClientRegistry
	excludedNamespaces             []string
	includedNamespaces             []string
	hpaEnforcerExcludedNamespaces  *[]string
	hpaEnforcerIncludedNamespaces  *[]string
	hpaEnforcerIsDryRun            *bool
	whitelistMode                  *bool
)

const policyAge = 1 * time.Second

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
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

	deploymentTriggerControllerEnv = testutil.SetupEnvironment()
	k8sClient1, err = client.New(deploymentTriggerControllerEnv.Cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient1).NotTo(BeNil())
	k8sManager1, err := ctrl.NewManager(deploymentTriggerControllerEnv.Cfg, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0.0.0.0:0",
	})
	Expect(err).ToNot(HaveOccurred())

	clientsRegistry = *registry.NewDeploymentClientRegistryBuilder().
		WithK8sClient(k8sClient1).
		WithCustomDeploymentClient(registry.NewDeploymentClient(k8sManager1.GetClient())).
		WithCustomDeploymentClient(registry.NewRolloutClient(k8sManager1.GetClient())).
		Build()
	err = (&DeploymentTriggerController{
		Client:          k8sManager1.GetClient(),
		Scheme:          k8sManager1.GetScheme(),
		ClientsRegistry: clientsRegistry,
	}).SetupWithManager(k8sManager1)
	Expect(err).ToNot(HaveOccurred())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0.0.0.0:0",
	})
	Expect(err).ToNot(HaveOccurred())

	excludedNamespaces = []string{"namespace1", "namespace2"}
	includedNamespaces = []string{}

	clientsRegistry = *registry.NewDeploymentClientRegistryBuilder().
		WithK8sClient(k8sManager.GetClient()).
		WithCustomDeploymentClient(registry.NewDeploymentClient(k8sManager.GetClient())).
		WithCustomDeploymentClient(registry.NewRolloutClient(k8sManager.GetClient())).
		Build()
	err = (&PolicyRecommendationRegistrar{
		Client:             k8sManager.GetClient(),
		Scheme:             k8sManager.GetScheme(),
		MonitorManager:     &FakeMonitorManager{},
		PolicyStore:        newFakePolicyStore(),
		ClientsRegistry:    clientsRegistry,
		ExcludedNamespaces: excludedNamespaces,
		IncludedNamespaces: includedNamespaces,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&PolicyWatcher{
		Client:         k8sManager.GetClient(),
		Scheme:         k8sManager.GetScheme(),
		clientRegistry: clientsRegistry,
		policyStore:    *policy.NewPolicyStore(k8sManager.GetClient()),
		requeueAllFunc: func() {
			queuedAllRecos = true
		},
		requeueOneFunc: func(namespacedName types.NamespacedName) {
			if namespacedName.Name == "test-deployment-afgre" || namespacedName.Name == "test-deployment-afgre2" {
				queuedOneReco = append(queuedOneReco, true)
			}
		},
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	recommender = &MockRecommender{}

	policyRecoReconciler, err := NewPolicyRecommendationReconciler(k8sManager.GetClient(),
		k8sManager.GetScheme(), k8sManager.GetEventRecorderFor(PolicyRecoWorkflowCtrlName),
		1, 3, recommender, newFakePolicyStore(), reco.NewDefaultPolicyIterator(k8sManager.GetClient(), clientsRegistry),
		reco.NewAgingPolicyIterator(k8sManager.GetClient(), policyAge))
	Expect(err).NotTo(HaveOccurred())
	err = policyRecoReconciler.
		SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	hpaEnforcerIsDryRun = new(bool)
	whitelistMode = new(bool)

	hpaEnforcerExcludedNamespaces = new([]string)
	hpaEnforcerIncludedNamespaces = new([]string)
	*hpaEnforcerIsDryRun = falseBool
	*whitelistMode = falseBool
	var autoscalerCRUD autoscaler.AutoscalerClient
	autoscalerCRUD = autoscaler.NewScaledobjectClient(k8sManager.GetClient(), &trueBool)
	hpaenforcer, err := NewHPAEnforcementController(k8sManager.GetClient(),
		k8sManager.GetScheme(), clientsRegistry, k8sManager.GetEventRecorderFor(HPAEnforcementCtrlName),
		1, hpaEnforcerIsDryRun, hpaEnforcerExcludedNamespaces, hpaEnforcerIncludedNamespaces, whitelistMode, 3, autoscalerCRUD)
	Expect(err).NotTo(HaveOccurred())
	err = hpaenforcer.
		SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()
	go func() {
		defer GinkgoRecover()
		err = k8sManager1.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testutil.TeardownSingletonEnvironment()
	Expect(err).NotTo(HaveOccurred())
	err = testutil.TeardownEnvironment(deploymentTriggerControllerEnv)
	Expect(err).NotTo(HaveOccurred())
})

type FakeMonitorManager struct{}

func (f *FakeMonitorManager) RegisterMonitor(workloadType string,
	workload types.NamespacedName) *trigger.Monitor {
	queuedAllRecos = true
	return nil
}

func (f *FakeMonitorManager) DeregisterMonitor(workload types.NamespacedName) {}
func (f *FakeMonitorManager) Shutdown()                                       {}

type FakePolicyStore struct {
	policies []ottoscaleriov1alpha1.Policy
}

func newFakePolicyStore() *FakePolicyStore {
	fakepolicies := []ottoscaleriov1alpha1.Policy{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "safest-policy"},
			Spec: ottoscaleriov1alpha1.PolicySpec{
				IsDefault:               false,
				RiskIndex:               1,
				MinReplicaPercentageCut: 100,
				TargetUtilization:       10,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "policy-1"},
			Spec: ottoscaleriov1alpha1.PolicySpec{
				IsDefault:               false,
				RiskIndex:               10,
				MinReplicaPercentageCut: 100,
				TargetUtilization:       15,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "policy-2"},
			Spec: ottoscaleriov1alpha1.PolicySpec{
				IsDefault:               false,
				RiskIndex:               20,
				MinReplicaPercentageCut: 100,
				TargetUtilization:       20,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "policy-3"},
			Spec: ottoscaleriov1alpha1.PolicySpec{
				IsDefault:               true,
				RiskIndex:               30,
				MinReplicaPercentageCut: 100,
				TargetUtilization:       30,
			},
		}, {
			ObjectMeta: metav1.ObjectMeta{Name: "policy-4"},
			Spec: ottoscaleriov1alpha1.PolicySpec{
				IsDefault:               false,
				RiskIndex:               40,
				MinReplicaPercentageCut: 100,
				TargetUtilization:       40,
			},
		},
	}
	return &FakePolicyStore{policies: fakepolicies}
}

func (ps *FakePolicyStore) GetSafestPolicy() (*ottoscaleriov1alpha1.Policy, error) {
	return &ps.policies[0], nil

}

func (ps *FakePolicyStore) GetDefaultPolicy() (*ottoscaleriov1alpha1.Policy, error) {
	for _, policy := range ps.policies {
		if policy.Spec.IsDefault {
			return &policy, nil
		}
	}

	return nil, errors.New("No default policy found")
}

func (ps *FakePolicyStore) GetNextPolicy(currentPolicy *ottoscaleriov1alpha1.Policy) (*ottoscaleriov1alpha1.Policy,
	error) {
	return &ottoscaleriov1alpha1.Policy{ObjectMeta: metav1.ObjectMeta{
		Name: "nextSafestPolicy"}, Spec: ottoscaleriov1alpha1.PolicySpec{}}, nil
}

func (ps *FakePolicyStore) GetNextPolicyByName(name string) (*ottoscaleriov1alpha1.Policy,
	error) {
	return &ottoscaleriov1alpha1.Policy{ObjectMeta: metav1.ObjectMeta{
		Name: "nextSafestPolicy"}, Spec: ottoscaleriov1alpha1.PolicySpec{}}, nil
}

func (ps *FakePolicyStore) GetPreviousPolicyByName(name string) (*ottoscaleriov1alpha1.Policy,
	error) {
	return &ottoscaleriov1alpha1.Policy{ObjectMeta: metav1.ObjectMeta{
		Name: "prevSafestPolicy"}, Spec: ottoscaleriov1alpha1.PolicySpec{}}, nil
}

func (ps *FakePolicyStore) GetSortedPolicies() (*ottoscaleriov1alpha1.PolicyList,
	error) {
	return &ottoscaleriov1alpha1.PolicyList{
		Items: ps.policies,
	}, nil
}

func (ps *FakePolicyStore) GetPolicyByName(name string) (*ottoscaleriov1alpha1.Policy,
	error) {
	return &ottoscaleriov1alpha1.Policy{ObjectMeta: metav1.ObjectMeta{
		Name: name}, Spec: ottoscaleriov1alpha1.PolicySpec{}}, nil
}

type MockRecommender struct {
	Min       int
	Threshold int
	Max       int
}

func (r *MockRecommender) Recommend(ctx context.Context, wm reco.WorkloadMeta) (*ottoscaleriov1alpha1.HPAConfiguration, error) {
	return &ottoscaleriov1alpha1.HPAConfiguration{
		Min:               r.Min,
		Max:               r.Max,
		TargetMetricValue: r.Threshold,
	}, nil
}
