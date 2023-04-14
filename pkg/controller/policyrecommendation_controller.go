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
	"github.com/flipkart-incubator/ottoscalr/pkg/reco"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	v1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
)

const recowfctrl = "RecoWorkflowController"

// PolicyRecommendationReconciler reconciles a PolicyRecommendation object
type PolicyRecommendationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=ottoscaler.io,resources=policyrecommendations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ottoscaler.io,resources=policyrecommendations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ottoscaler.io,resources=policyrecommendations/finalizers,verbs=update

func (r *PolicyRecommendationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	policyreco := v1alpha1.PolicyRecommendation{}
	if err := r.Get(ctx, req.NamespacedName, &policyreco); err != nil {
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	recowf, err := reco.NewRecommendationWorkflow()
	if err != nil {
		return ctrl.Result{}, err
	}
	currentreco, targetreco, policy, err := recowf.Execute(ctx, reco.WorkloadMeta{
		TypeMeta:  policyreco.Spec.WorkloadMeta.TypeMeta,
		Name:      policyreco.Spec.WorkloadMeta.Name,
		Namespace: policyreco.Spec.WorkloadMeta.Namespace,
	})

	if err := r.Patch(ctx, &v1alpha1.PolicyRecommendation{
		TypeMeta:   policyreco.TypeMeta,
		ObjectMeta: policyreco.ObjectMeta,
		Spec: v1alpha1.PolicyRecommendationSpec{
			TargetHPAConfiguration:  *targetreco,
			Policy:                  policy.Name,
			CurrentHPAConfiguration: *currentreco,
		},
	}, client.Apply, client.ForceOwnership, client.FieldOwner(recowfctrl)); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// TODO: Add status and events
	statusPatch := &v1alpha1.PolicyRecommendation{
		TypeMeta:   policyreco.TypeMeta,
		ObjectMeta: policyreco.ObjectMeta,
		Status:     v1alpha1.PolicyRecommendationStatus{},
	}
	if err := r.Status().Patch(ctx, statusPatch, client.Apply, getSubresourcePatchOptions()); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	return ctrl.Result{}, nil
}

func getSubresourcePatchOptions() *client.SubResourcePatchOptions {
	patchOpts := client.PatchOptions{}
	client.ForceOwnership.ApplyToPatch(&patchOpts)
	client.FieldOwner(recowfctrl).ApplyToPatch(&patchOpts)
	return &client.SubResourcePatchOptions{
		PatchOptions: patchOpts,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *PolicyRecommendationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// implement predicates to filter events with updates to QueuedForExecution or QueuedForExecutionAt
	queuedTaskPredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			objSpec := e.Object.(*v1alpha1.PolicyRecommendation).Spec
			switch {
			case objSpec.QueuedForExecution == true:
				return true
			default:
				return false
			}
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldObjSpec := e.ObjectOld.(*v1alpha1.PolicyRecommendation).Spec
			newObjSpec := e.ObjectNew.(*v1alpha1.PolicyRecommendation).Spec
			switch {
			case oldObjSpec.QueuedForExecution == false && newObjSpec.QueuedForExecution == true:
				return true
			case newObjSpec.QueuedForExecution == true && (oldObjSpec.QueuedForExecutionAt.Before(&newObjSpec.QueuedForExecutionAt)):
				return true
			default:
				return false
			}
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
	predicate := predicate.And(predicate.GenerationChangedPredicate{}, queuedTaskPredicate)
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.PolicyRecommendation{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}).
		WithEventFilter(predicate).
		Named(recowfctrl).
		Complete(r)
}
