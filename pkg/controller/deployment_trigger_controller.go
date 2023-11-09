package controller

import (
	"context"
	"time"

	ottoscaleriov1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	"github.com/flipkart-incubator/ottoscalr/pkg/reco"
	"github.com/flipkart-incubator/ottoscalr/pkg/registry"
	"github.com/go-logr/logr"
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
)

const (
	DeploymentTriggerCtrlName = "DeploymentTriggerController"
)

type DeploymentTriggerController struct {
	Client          client.Client
	Scheme          *runtime.Scheme
	ClientsRegistry registry.DeploymentClientRegistry
}

func NewDeploymentTriggerController(client client.Client,
	scheme *runtime.Scheme, clientsRegistry registry.DeploymentClientRegistry) *DeploymentTriggerController {
	return &DeploymentTriggerController{
		Client:          client,
		Scheme:          scheme,
		ClientsRegistry: clientsRegistry,
	}
}

// +kubebuilder:rbac:groups=argoproj.io,resources=rollouts,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=your-group.io,resources=policyrecommendations,verbs=create;get;list;watch;update;delete
//+kubebuilder:rbac:groups=ottoscaler.io,resources=policyrecommendations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ottoscaler.io,resources=policyrecommendations/finalizers,verbs=update

func (r *DeploymentTriggerController) Reconcile(ctx context.Context,
	request ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger = logger.WithValues("request", request).WithName(DeploymentTriggerCtrlName)

	for _, obj := range r.ClientsRegistry.Clients {
		object, err := obj.GetObject(request.Namespace, request.Name)
		if err == nil {
			return ctrl.Result{}, r.requeuePolicyRecommendation(ctx, object, r.Scheme, logger)
		}
		if !errors.IsNotFound(err) {
			// Error occurred
			logger.Error(err, "Failed to get. Requeue the request")
			return ctrl.Result{RequeueAfter: 1 * time.Second}, err
		}
	}

	logger.Info("Rollout or Deployment not found.")
	return ctrl.Result{}, nil
}

func (r *DeploymentTriggerController) requeuePolicyRecommendation(ctx context.Context,
	object client.Object,
	scheme *runtime.Scheme,
	logger logr.Logger) error {

	now := metav1.Now()
	policyRecommendation := &ottoscaleriov1alpha1.PolicyRecommendation{}

	err := r.Client.Get(context.Background(), types.NamespacedName{
		Namespace: object.GetNamespace(),
		Name:      object.GetName(),
	}, policyRecommendation)
	if err != nil {
		logger.Error(err, "Error while getting policyRecommendation.", "workloadName", object.GetName(), "workloadNamespace", object.GetNamespace())
		return client.IgnoreNotFound(err)
	}

	policyRecommendation.Spec.QueuedForExecution = &trueBool
	policyRecommendation.Spec.QueuedForExecutionAt = &now

	err = r.Client.Update(context.Background(), policyRecommendation, client.FieldOwner(DeploymentTriggerCtrlName))
	if err != nil {
		logger.Error(err, "Error while updating policyRecommendation.", "workloadName", object.GetName(), "workloadNamespace", object.GetNamespace())
		return err
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeploymentTriggerController) SetupWithManager(mgr ctrl.Manager) error {
	annotationUpdatePredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			newObj := e.ObjectNew
			oldObj := e.ObjectOld

			var newMaxPods, oldMaxPods string
			newMaxPods, _ = newObj.GetAnnotations()[reco.OttoscalrMaxPodAnnotation]
			oldMaxPods, _ = oldObj.GetAnnotations()[reco.OttoscalrMaxPodAnnotation]

			if newMaxPods != oldMaxPods {
				return true
			}

			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}

	enqueueFunc := func(ctx context.Context, obj client.Object) []reconcile.Request {
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: obj.GetName(),
			Namespace: obj.GetNamespace()}}}
	}

	controllerBuilder := ctrl.NewControllerManagedBy(mgr).
		Named(DeploymentTriggerCtrlName)

	for _, object := range r.ClientsRegistry.Clients {
		controllerBuilder.Watches(
			object.GetObjectType(),
			handler.EnqueueRequestsFromMapFunc(enqueueFunc),
			builder.WithPredicates(annotationUpdatePredicate),
		)
	}

	return controllerBuilder.Complete(r)
}
