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
	"reflect"

	ottoscaleriov1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const PolicyWatcherCtrl = "PolicyWatcher"
const policyFinalizerName = "finalizer.ottoscaler.io"

var policyRefKey = ".spec.policy"

// PolicyWatcher reconciles a Policy object
type PolicyWatcher struct {
	Client         client.Client
	Scheme         *runtime.Scheme
	requeueAllFunc func()
	requeueOneFunc func(types.NamespacedName)
}

func NewPolicyWatcher(client client.Client,
	scheme *runtime.Scheme,
	requeueAllFunc func(),
	requeueOneFunc func(types.NamespacedName),
) *PolicyWatcher {
	return &PolicyWatcher{Client: client,
		Scheme:         scheme,
		requeueAllFunc: requeueAllFunc,
		requeueOneFunc: requeueOneFunc,
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

	// Add finalizer to the policy if it doesn't have one
	policy, err := r.addFinalizer(ctx, policy)

	if err != nil {
		logger.Error(err, "Error adding finalizer to policy")
		return ctrl.Result{}, err
	}
	//Handle Reconcile
	//If it is a delete event or update in the spec
	//Requeue all policyRecommendations having the request Policy object as a reference
	err = r.handleReconcilation(ctx, policy, logger)

	if err != nil {
		logger.Error(err, "Error handling reconcilation of policy")
		return ctrl.Result{}, err
	}

	// If the policy is deleted
	if !policy.ObjectMeta.DeletionTimestamp.IsZero() {

		// Remove finalizer from the policy
		policy.ObjectMeta.Finalizers = removeString(policy.ObjectMeta.Finalizers, policyFinalizerName)
		if err := r.Client.Update(ctx, &policy); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil

	}

	// If the policy is marked as default, ensure no other policy is marked as default
	if policy.Spec.IsDefault {
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

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &ottoscaleriov1alpha1.PolicyRecommendation{}, policyRefKey, func(rawObj client.Object) []string {
		policyRecommendation := rawObj.(*ottoscaleriov1alpha1.PolicyRecommendation)
		if policyRecommendation.Spec.Policy == "" {
			return nil
		}
		return []string{policyRecommendation.Spec.Policy}
	}); err != nil {
		return err
	}

	reconcilePredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldObj := e.ObjectOld.(*ottoscaleriov1alpha1.Policy)
			newObj := e.ObjectNew.(*ottoscaleriov1alpha1.Policy)

			return !reflect.DeepEqual(oldObj.Spec, newObj.Spec)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		Watches(&ottoscaleriov1alpha1.Policy{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(predicate.Or(predicate.GenerationChangedPredicate{}, reconcilePredicate))).
		Named(PolicyWatcherCtrl).
		Complete(r)
}

func (r *PolicyWatcher) addFinalizer(ctx context.Context, policy ottoscaleriov1alpha1.Policy) (ottoscaleriov1alpha1.Policy, error) {
	// Add finalizer to the policy if it doesn't have one
	if !containsString(policy.ObjectMeta.Finalizers, policyFinalizerName) {
		policy.ObjectMeta.Finalizers = append(policy.ObjectMeta.Finalizers, policyFinalizerName)
		if err := r.Client.Update(ctx, &policy); err != nil {
			return ottoscaleriov1alpha1.Policy{}, err
		}
	}

	return policy, nil
}

func (r *PolicyWatcher) handleReconcilation(ctx context.Context, policy ottoscaleriov1alpha1.Policy, logger logr.Logger) error {
	// Get all PolicyRecommendation objects that reference the Policy object
	var policyRecommendations ottoscaleriov1alpha1.PolicyRecommendationList
	if err := r.Client.List(ctx, &policyRecommendations, &client.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(policyRefKey, policy.Name)}); err != nil {
		return err
	}

	// Requeue all PolicyRecommendation objects having the Policy object as a reference
	for _, policyRecommendation := range policyRecommendations.Items {
		logger.Info("Requeueing PolicyRecommendation as some update/delete in seen the policy field", "policyRecommendation", policyRecommendation.Name, "Namespace", policyRecommendation.Namespace,
			"policy", policyRecommendation.Spec.Policy)
		r.requeueOneFunc(types.NamespacedName{Namespace: policyRecommendation.Namespace,
			Name: policyRecommendation.Name})
	}

	return nil
}

func containsString(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}

	return false
}

func removeString(slice []string, str string) []string {
	var result []string
	for _, s := range slice {
		if s != str {
			result = append(result, s)
		}
	}

	return result
}
