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
	"context"
	ottoscaleriov1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const POLICY_WATCHER_CTRL = "PolicyWatcher"

// PolicyWatcher reconciles a Policy object
type PolicyWatcher struct {
	Client         client.Client
	Scheme         *runtime.Scheme
	requeueAllFunc func()
}

func NewPolicyWatcher(client client.Client,
	scheme *runtime.Scheme,
	requeueAllFunc func(),
) *PolicyWatcher {
	return &PolicyWatcher{Client: client,
		Scheme:         scheme,
		requeueAllFunc: requeueAllFunc,
	}
}

//+kubebuilder:rbac:groups=ottoscaler.io,resources=policies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ottoscaler.io,resources=policies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ottoscaler.io,resources=policies/finalizers,verbs=update

func (r *PolicyWatcher) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	logger := log.FromContext(ctx)
	// Get the Policy instance
	var policy ottoscaleriov1alpha1.Policy
	if err := r.Client.Get(ctx, req.NamespacedName, &policy); err != nil {
		if errors.IsNotFound(err) {
			// Ignore not found errors, as the object might have been deleted
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if policy.Spec.IsDefault {
		// If the policy is marked as default, ensure no other policy is marked as default
		var allPolicies ottoscaleriov1alpha1.PolicyList
		if err := r.Client.List(ctx, &allPolicies); err != nil {
			logger.Error(err, "Error getting allPolicies")
			return ctrl.Result{}, err
		}

		for _, otherPolicy := range allPolicies.Items {
			// Skip the current policy
			if otherPolicy.Name == policy.Name {
				continue
			}
			// If the other policy is marked as default, update it to be non-default
			if otherPolicy.Spec.IsDefault {
				otherPolicy.Spec.IsDefault = false
				if err := r.Client.Update(ctx, &otherPolicy); err != nil {
					logger.Error(err, "Error marking policy as default", "policy", otherPolicy)
					return ctrl.Result{}, err
				}
			}
		}
		//requeue all policyRecommendations
		r.requeueAllFunc()
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PolicyWatcher) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).
		For(&ottoscaleriov1alpha1.Policy{}).
		Named(POLICY_WATCHER_CTRL).
		Complete(r)
}
