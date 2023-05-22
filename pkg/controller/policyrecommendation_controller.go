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
	"fmt"
	"github.com/flipkart-incubator/ottoscalr/pkg/reco"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"time"

	v1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
)

const (
	PolicyRecoWorkflowCtrlName = "RecoWorkflowController"
	eventTypeNormal            = "Normal"
	eventTypeWarning           = "Warning"
)

var (
	falseBool = false
	trueBool  = true
)

// PolicyRecommendationReconciler reconciles a PolicyRecommendation object
type PolicyRecommendationReconciler struct {
	client.Client
	Scheme                  *runtime.Scheme
	Recorder                record.EventRecorder
	MaxConcurrentReconciles int
	PolicyExpiryAge         time.Duration
	RecoWorkflow            reco.RecommendationWorkflow
}

func NewPolicyRecommendationReconciler(client client.Client,
	scheme *runtime.Scheme, recorder record.EventRecorder,
	maxConcurrentReconciles int, recommender reco.Recommender, policyIterators ...reco.PolicyIterator) *PolicyRecommendationReconciler {
	recoWfBuilder := reco.NewRecommendationWorkflowBuilder().
		WithRecommender(recommender)
	for _, pi := range policyIterators {
		recoWfBuilder = recoWfBuilder.WithPolicyIterator(pi)
	}
	return &PolicyRecommendationReconciler{
		Client:                  client,
		Scheme:                  scheme,
		MaxConcurrentReconciles: maxConcurrentReconciles,
		Recorder:                recorder,
		RecoWorkflow:            recoWfBuilder.Build(),
	}
}

//+kubebuilder:rbac:groups=ottoscaler.io,resources=policyrecommendations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ottoscaler.io,resources=policyrecommendations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ottoscaler.io,resources=policyrecommendations/finalizers,verbs=update

func (r *PolicyRecommendationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx).WithName(PolicyRecoWorkflowCtrlName)

	policyreco := v1alpha1.PolicyRecommendation{}
	if err := r.Get(ctx, req.NamespacedName, &policyreco); err != nil {
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.V(2).Info("PolicyRecomemndation retrieved", "policyreco", policyreco)

	r.Recorder.Event(&policyreco, eventTypeNormal, "HPARecoQueuedForExecution", "This workload has been queued for a fresh HPA recommendation.")

	hpaConfigToBeApplied, targetHPAReco, policy, err := r.RecoWorkflow.Execute(ctx, reco.WorkloadMeta{
		TypeMeta:  policyreco.Spec.WorkloadMeta.TypeMeta,
		Name:      policyreco.Spec.WorkloadMeta.Name,
		Namespace: policyreco.Spec.WorkloadMeta.Namespace,
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	if hpaConfigToBeApplied == nil {
		logger.V(0).Error(nil, "HPA config to be applied is empty. Skipping and moving on.")
	}

	if targetHPAReco == nil {
		logger.V(0).Error(nil, "Recommended config is empty. Requeuing")
		return ctrl.Result{
			RequeueAfter: 5 * time.Second,
		}, nil

	}

	var policyName string

	if policy != nil {
		policyName = policy.Name
	}

	transitionedAt := retrieveTransitionTime(hpaConfigToBeApplied, &policyreco)
	now := metav1.Now()
	policyRecoPatch := &v1alpha1.PolicyRecommendation{
		TypeMeta: policyreco.TypeMeta,
		ObjectMeta: metav1.ObjectMeta{
			Name:      policyreco.Name,
			Namespace: policyreco.Namespace,
		},
		Spec: v1alpha1.PolicyRecommendationSpec{
			QueuedForExecution:      &falseBool,
			TargetHPAConfiguration:  *targetHPAReco,
			Policy:                  policyName,
			CurrentHPAConfiguration: *hpaConfigToBeApplied,
			TransitionedAt:          &transitionedAt,
			GeneratedAt:             &now,
		},
	}
	logger.V(0).Info("Policy Patch", "PolicyReco", *policyRecoPatch)
	if err := r.Patch(ctx, policyRecoPatch, client.Apply, client.ForceOwnership, client.FieldOwner(PolicyRecoWorkflowCtrlName)); err != nil {
		logger.Error(err, "Error patching the policy reco object")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	logger.V(0).Info("Policy Patch", "PolicyReco", *policyRecoPatch)

	statusPatch := &v1alpha1.PolicyRecommendation{
		TypeMeta: policyreco.TypeMeta,
		ObjectMeta: metav1.ObjectMeta{
			Name:      policyreco.Name,
			Namespace: policyreco.Namespace,
		},
		Status: v1alpha1.PolicyRecommendationStatus{},
	}
	if err := r.Status().Patch(ctx, statusPatch, client.Apply, getSubresourcePatchOptions()); err != nil {
		logger.Error(err, "Failed to patch the status")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	r.Recorder.Event(&policyreco, eventTypeNormal, "HPARecommendationGenerated", fmt.Sprintf("The HPA recommendation has been generated successfully. The current policy this workload is at %s.", policyName))
	logger.V(1).Info("Successfully generated HPA Recommendation.")
	return ctrl.Result{}, nil
}

func retrieveTransitionTime(hpaConfigToBeApplied *v1alpha1.HPAConfiguration, policyreco *v1alpha1.PolicyRecommendation) metav1.Time {
	if hpaConfigToBeApplied == nil || policyreco == nil {
		return metav1.Now()
	}
	if !hpaConfigToBeApplied.DeepEquals(policyreco.Spec.CurrentHPAConfiguration) {
		return metav1.Now()
	}
	return *policyreco.Spec.TransitionedAt
}

func getSubresourcePatchOptions() *client.SubResourcePatchOptions {
	patchOpts := client.PatchOptions{}
	client.ForceOwnership.ApplyToPatch(&patchOpts)
	client.FieldOwner(PolicyRecoWorkflowCtrlName).ApplyToPatch(&patchOpts)
	return &client.SubResourcePatchOptions{
		PatchOptions: patchOpts,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *PolicyRecommendationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// predicates to filter events with updates to QueuedForExecution or QueuedForExecutionAt
	queuedTaskPredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			objSpec := e.Object.(*v1alpha1.PolicyRecommendation).Spec
			switch {
			case *objSpec.QueuedForExecution == true:
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
			// If it hasn't been touched by the registrar don't reconcile
			if len(newObjSpec.Policy) == 0 || newObjSpec.QueuedForExecutionAt.IsZero() {
				return false
			}
			switch {
			// updates which transition QueuedForExecution from false to true
			case *oldObjSpec.QueuedForExecution == false && *newObjSpec.QueuedForExecution == true:
				return true
			//	updates where there's no change to the QueuedForExecution but to the QueuedForExecutionAt with a later timestamp
			case (*newObjSpec.QueuedForExecution == true && oldObjSpec.QueuedForExecutionAt.Before(newObjSpec.QueuedForExecutionAt)) || newObjSpec.QueuedForExecutionAt.After(newObjSpec.GeneratedAt.Time):
				return true
			default:
				return false
			}
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
	compoundPredicate := predicate.And(predicate.GenerationChangedPredicate{}, queuedTaskPredicate)
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.PolicyRecommendation{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: r.MaxConcurrentReconciles}).
		WithEventFilter(compoundPredicate).
		Named(PolicyRecoWorkflowCtrlName).
		Complete(r)
}
