package controller

import (
	"context"
	argov1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	ottoscaleriov1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
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

func (r *PolicyRecommendationRegistrar) Reconcile(ctx context.Context,
	request ctrl.Request) (ctrl.Result, error) {

	log := log.FromContext(ctx)

	// Fetch the Deployment or Rollout instance
	//TODO handle the case for deployment
	instance := argov1alpha1.Rollout{}
	err := r.Client.Get(ctx, request.NamespacedName, &instance)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Object not found, could have been deleted after reconcile request")
			// Object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Error reading the object - requeue the request")
		return reconcile.Result{}, err
	}

	// Check if a PolicyRecommendation object already exists
	policy := &ottoscaleriov1alpha1.PolicyRecommendation{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, policy)
	if err == nil {
		log.Info("PolicyRecommendation object already exists")
		// PolicyRecommendation already exists, so we don't need to create one
		return ctrl.Result{}, nil
	} else if !errors.IsNotFound(err) {
		// Error reading the object - requeue the request.
		log.Error(err, "Error reading the object - requeue the request")
		return ctrl.Result{}, err
	}

	log.Info("Creating a new PolicyRecommendation object")

	// Create a new PolicyRecommendation object
	newPolicyRecommendation := &ottoscaleriov1alpha1.PolicyRecommendation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(&instance, instance.GroupVersionKind()),
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

	enqueueFunc := func(a client.Object) []reconcile.Request {
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: a.GetName(), Namespace: a.GetNamespace()}}}
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&argov1alpha1.Rollout{}).
		For(&appsv1.Deployment{}).
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
