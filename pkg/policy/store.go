package policy

import (
	"context"
	"errors"
	"fmt"
	"github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"sort"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Store interface {
	GetSafestPolicy() (*v1alpha1.Policy, error)
	GetDefaultPolicy() (*v1alpha1.Policy, error)
	GetNextPolicy(currentPolicy *v1alpha1.Policy) (*v1alpha1.Policy, error)
	GetNextPolicyByName(name string) (*v1alpha1.Policy, error)
	GetPolicyByName(name string) (*v1alpha1.Policy, error)
}
type PolicyStore struct {
	k8sClient client.Client
}

func NewPolicyStore(k8sClient client.Client) *PolicyStore {
	return &PolicyStore{
		k8sClient: k8sClient,
	}
}

func (ps *PolicyStore) GetSafestPolicy() (*v1alpha1.Policy, error) {
	policies := &v1alpha1.PolicyList{}
	err := ps.k8sClient.List(context.Background(), policies)
	if err != nil {
		return nil, err
	}

	if len(policies.Items) == 0 {
		return nil, fmt.Errorf("no policies found")
	}

	sort.Slice(policies.Items, func(i, j int) bool {
		return policies.Items[i].Spec.RiskIndex < policies.Items[j].Spec.RiskIndex
	})

	return &policies.Items[0], nil
}

func (ps *PolicyStore) GetNextPolicy(currentPolicy *v1alpha1.Policy) (*v1alpha1.Policy, error) {
	policies := &v1alpha1.PolicyList{}
	err := ps.k8sClient.List(context.Background(), policies)
	if err != nil {
		return nil, err
	}

	sort.Slice(policies.Items, func(i, j int) bool {
		return policies.Items[i].Spec.RiskIndex < policies.Items[j].Spec.RiskIndex
	})

	for i, policy := range policies.Items {
		if policy.Spec.RiskIndex == currentPolicy.Spec.RiskIndex {
			if i+1 < len(policies.Items) {
				return &policies.Items[i+1], nil
			}
			break
		}
	}

	return nil, fmt.Errorf("no next policy found")
}

func (ps *PolicyStore) GetNextPolicyByName(name string) (*v1alpha1.Policy, error) {
	currentPolicy, err := ps.GetPolicyByName(name)
	if err != nil {
		return nil, err
	}

	policies := &v1alpha1.PolicyList{}
	err2 := ps.k8sClient.List(context.Background(), policies)
	if err2 != nil {
		return nil, err2
	}

	sort.Slice(policies.Items, func(i, j int) bool {
		return policies.Items[i].Spec.RiskIndex < policies.Items[j].Spec.RiskIndex
	})

	for i, policy := range policies.Items {
		if policy.Spec.RiskIndex == currentPolicy.Spec.RiskIndex {
			if i+1 < len(policies.Items) {
				return &policies.Items[i+1], nil
			}
			break
		}
	}

	return nil, fmt.Errorf("no next policy found")
}

func (ps *PolicyStore) GetPolicyByName(name string) (*v1alpha1.Policy, error) {
	policy := &v1alpha1.Policy{}
	err := ps.k8sClient.Get(context.Background(), types.NamespacedName{Name: name}, policy)
	if err != nil {
		return nil, err
	}
	return policy, nil
}

func (ps *PolicyStore) GetDefaultPolicy() (*v1alpha1.Policy, error) {
	policies := &v1alpha1.PolicyList{}
	err := ps.k8sClient.List(context.Background(), policies)
	if err != nil {
		return nil, err
	}

	if len(policies.Items) == 0 {
		return nil, fmt.Errorf("no policies found")
	}

	sort.Slice(policies.Items, func(i, j int) bool {
		return policies.Items[i].Spec.RiskIndex < policies.Items[j].Spec.RiskIndex
	})

	for _, policy := range policies.Items {
		if isDefault(policy) {
			return &policy, nil
		}
	}

	return nil, errors.New("No default policy found")
}

func isDefault(policy v1alpha1.Policy) bool {
	return policy.Spec.IsDefault
}
