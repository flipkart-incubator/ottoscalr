package policy

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"

	"github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Store interface {
	GetSafestPolicy() (*v1alpha1.Policy, error)
	GetDefaultPolicy() (*v1alpha1.Policy, error)
	GetNextPolicyByName(name string) (*v1alpha1.Policy, error)
	GetPreviousPolicyByName(name string) (*v1alpha1.Policy, error)
	GetPolicyByName(name string) (*v1alpha1.Policy, error)
	GetSortedPolicies() (*v1alpha1.PolicyList, error)
}
type PolicyStore struct {
	k8sClient client.Client
}

func NewPolicyStore(k8sClient client.Client) *PolicyStore {
	return &PolicyStore{
		k8sClient: k8sClient,
	}
}

var NoNextPolicyFoundErr = errors.New("no next policy found")
var NoPrevPolicyFoundErr = errors.New("no previous policy found")
var NoPolicyFoundErr = errors.New("no policy found")

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

func (ps *PolicyStore) GetNextPolicyByName(name string) (*v1alpha1.Policy, error) {
	log.Println("Identifying next policy to ", name)
	currentPolicy, err := ps.GetPolicyByName(name)
	if err != nil {
		return nil, err
	}

	policies, err2 := ps.GetSortedPolicies()
	if err2 != nil {
		log.Println("Error when fetching policies.")
		return nil, err2
	}

	for i, policy := range policies.Items {
		if policy.Name == currentPolicy.Name {
			if i+1 < len(policies.Items) {
				return &policies.Items[i+1], nil
			}
			break
		}
	}

	return nil, NoNextPolicyFoundErr
}

func (ps *PolicyStore) GetPreviousPolicyByName(name string) (*v1alpha1.Policy, error) {
	log.Println("Identifying previous policy to ", name)
	currentPolicy, err := ps.GetPolicyByName(name)
	if err != nil {
		return nil, err
	}

	policies, err2 := ps.GetSortedPolicies()
	if err2 != nil {
		log.Println("Error when fetching policies.")
		return nil, err2
	}
	for i, policy := range policies.Items {
		if policy.Name == currentPolicy.Name {
			if i-1 >= 0 {
				return &policies.Items[i-1], nil
			}
			break
		}
	}

	return nil, NoPrevPolicyFoundErr
}

func (ps *PolicyStore) GetSortedPolicies() (*v1alpha1.PolicyList, error) {
	policies := &v1alpha1.PolicyList{}
	err2 := ps.k8sClient.List(context.Background(), policies)
	if err2 != nil {
		return nil, err2
	}

	//Get only policies having deletion timestamp as zero
	filteredPolicies := policies.DeepCopy()
	filteredPolicies.Items = nil
	for _, policy := range policies.Items {
		if policy.ObjectMeta.DeletionTimestamp.IsZero() {
			filteredPolicies.Items = append(filteredPolicies.Items, policy)
		}
	}

	sort.Slice(filteredPolicies.Items, func(i, j int) bool {
		return filteredPolicies.Items[i].Spec.RiskIndex < filteredPolicies.Items[j].Spec.RiskIndex
	})
	return filteredPolicies, nil
}

func (ps *PolicyStore) GetPolicyByName(name string) (*v1alpha1.Policy, error) {
	policy := &v1alpha1.Policy{}
	err := ps.k8sClient.Get(context.Background(), types.NamespacedName{Name: name}, policy)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, NoPolicyFoundErr
		}
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

func IsLastPolicy(err error) bool {
	return errors.Is(err, NoNextPolicyFoundErr)
}

func IsSafestPolicy(err error) bool {
	return errors.Is(err, NoNextPolicyFoundErr)
}
