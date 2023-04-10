package controller

import (
	"context"
	argov1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	ottoscaleriov1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// PolicyRecommendationRegistrar reconciles a Deployment or ArgoRollout
// object to ensure a PolicyRecommendation exists.
type PolicyRecommendationRegistrar struct {
	Client client.Client
	Scheme *runtime.Scheme
}

func (r *PolicyRecommendationRegistrar) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log = log.WithValues("request", request)

	// Check if Rollout exists
	rollout := argov1alpha1.Rollout{}
	err := r.Client.Get(ctx, request.NamespacedName, &rollout)
	if err == nil {
		// Rollout exists, create policy recommendation
		return r.createPolicyRecommendation(ctx, &rollout, log)
	}

	if !errors.IsNotFound(err) {
		// Error occurred
		log.Error(err, "Failed to get Rollout. Requeue the request")
		return reconcile.Result{}, err
	}

	// Check if Deployment exists
	deployment := appsv1.Deployment{}
	err = r.Client.Get(ctx, request.NamespacedName, &deployment)
	if err == nil {
		// Deployment exists, create policy recommendation
		return r.createPolicyRecommendation(ctx, &deployment, log)
	}

	if !errors.IsNotFound(err) {
		log.Error(err, "Failed to get Deployment. Requeue the request")
		return reconcile.Result{}, err
	}

	log.Info("Rollout or Deployment not found. It could have been deleted.")
	return reconcile.Result{}, nil
}

func (r *PolicyRecommendationRegistrar) createPolicyRecommendation(ctx context.Context,
	instance client.Object,
	log logr.Logger) (ctrl.Result, error) {
	// Check if a PolicyRecommendation object already exists
	policy := &ottoscaleriov1alpha1.PolicyRecommendation{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: instance.GetName(), Namespace: instance.GetNamespace()}, policy)
	if err == nil {
		log.Info("PolicyRecommendation object already exists")
		return ctrl.Result{}, nil
	} else if !errors.IsNotFound(err) {
		log.Error(err, "Error reading the object - requeue the request")
		return ctrl.Result{}, err
	}

	log.Info("Creating a new PolicyRecommendation object")

	newPolicyRecommendation := &ottoscaleriov1alpha1.PolicyRecommendation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.GetName(),
			Namespace: instance.GetNamespace(),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(instance, instance.GetObjectKind().GroupVersionKind()),
			},
		},
		Spec: ottoscaleriov1alpha1.PolicyRecommendationSpec{
			//TODO Set the fields in the policy spec as needed

		},
	}

	err = r.Client.Create(ctx, newPolicyRecommendation)
	if err != nil {
		// Error creating the object - requeue the request.
		log.Error(err, "Error creating the object - requeue the request")
		return ctrl.Result{}, err
	}

	log.Info("PolicyRecommendation created successfully")
	// PolicyRecommendation created successfully - return and don't requeue
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PolicyRecommendationRegistrar) SetupWithManager(mgr ctrl.Manager) error {
	createPredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
	}

	enqueueFunc := func(obj client.Object) []reconcile.Request {
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: obj.GetName(),
			Namespace: obj.GetNamespace()}}}
	}

	return ctrl.NewControllerManagedBy(mgr).
		Watches(
			&source.Kind{Type: &argov1alpha1.Rollout{}},
			handler.EnqueueRequestsFromMapFunc(enqueueFunc),
			builder.WithPredicates(createPredicate),
		).
		Watches(
			&source.Kind{Type: &appsv1.Deployment{}},
			handler.EnqueueRequestsFromMapFunc(enqueueFunc),
			builder.WithPredicates(createPredicate),
		).
		Complete(r)
}
