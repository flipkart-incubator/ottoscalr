package controller

import (
	"context"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
)

// BreachMonitorTrigger monitors Rollouts and Deployments for SLO breaches
type BreachMonitorTrigger struct {
	client.Client
	Scheme *runtime.Scheme
}

func (c *BreachMonitorTrigger) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Get the Rollout object.
	rollout := &argov1alpha1.Rollout{}
	err := c.Client.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, rollout)
	if err != nil {
		log.Error(err, "Failed to get Rollout")
		return ctrl.Result{}, err
	}

	// Get the Deployment object.
	deployment := &appsv1.Deployment{}
	err = c.Client.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, deployment)
	if err != nil {
		log.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}

	// Monitor for breaches.
	go func() {
		for {
			time.Sleep(5 * time.Second)

			// Check for breaches in the Rollout or Deployment.
			//if isBreach(rollout, deployment) {
			//	log.Info("Breach detected!")
			//	// TODO  Handle breach logic.
			//	// ...
			//}
		}
	}()

	return ctrl.Result{}, nil
}

func (c *BreachMonitorTrigger) SetupWithManager(mgr ctrl.Manager) error {
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
		Complete(c)
}
