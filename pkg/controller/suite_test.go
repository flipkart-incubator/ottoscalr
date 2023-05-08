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
	rolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/flipkart-incubator/ottoscalr/pkg/reco"
	"github.com/flipkart-incubator/ottoscalr/pkg/testutil"
	"github.com/flipkart-incubator/ottoscalr/pkg/trigger"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/net/context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"testing"

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
	cfg       *rest.Config
	k8sClient client.Client
	ctx       context.Context
	cancel    context.CancelFunc

	queuedAllRecos = false
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

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	err = (&PolicyRecommendationRegistrar{
		Client:         k8sManager.GetClient(),
		Scheme:         k8sManager.GetScheme(),
		MonitorManager: &FakeMonitorManager{},
		PolicyStore:    newFakePolicyStore(),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&PolicyWatcher{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
		requeueAllFunc: func() {
			queuedAllRecos = true
		},
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = NewPolicyRecommendationReconciler(k8sManager.GetClient(),
		k8sManager.GetScheme(), k8sManager.GetEventRecorderFor(POLICY_RECO_WORKFLOW_CTRL_NAME),
		1, &reco.MockRecommender{
			Min:       10,
			Threshold: 60,
			Max:       60,
		}, reco.NewDefaultPolicyIterator(k8sManager.GetClient()),
		reco.NewAgingPolicyIterator(k8sManager.GetClient(), policyAge)).
		SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testutil.TeardownEnvironment()
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
				MinReplicaPercentageCut: 80,
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

func (ps *FakePolicyStore) GetPolicyByName(name string) (*ottoscaleriov1alpha1.Policy,
	error) {
	return &ottoscaleriov1alpha1.Policy{ObjectMeta: metav1.ObjectMeta{
		Name: name}, Spec: ottoscaleriov1alpha1.PolicySpec{}}, nil
}
