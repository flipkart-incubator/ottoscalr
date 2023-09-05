package controller

import (
	"context"
	"fmt"
	argov1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	ottoscaleriov1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	"github.com/flipkart-incubator/ottoscalr/pkg/reco"
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
	"time"
)

const (
	DeploymentCtrlName = "DeploymentController"
)

type DeploymentController struct {
	Client client.Client
	Scheme *runtime.Scheme
}

func NewDeploymentController(client client.Client,
	scheme *runtime.Scheme,
) *DeploymentController {
	return &DeploymentController{
		Client: client,
		Scheme: scheme,
	}
}

// +kubebuilder:rbac:groups=argoproj.io,resources=rollouts,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=your-group.io,resources=policyrecommendations,verbs=create;get;list;watch;update;delete
//+kubebuilder:rbac:groups=ottoscaler.io,resources=policyrecommendations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ottoscaler.io,resources=policyrecommendations/finalizers,verbs=update

// TODO neerajb Handle the deletion of workloads. We should reregister the monitors.
func (r *DeploymentController) Reconcile(ctx context.Context,
	request ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger = logger.WithValues("request", request).WithName(PolicyRecoRegistrarCtrlName)

	// Check if Rollout is enqueued
	rollout := argov1alpha1.Rollout{}
	err := r.Client.Get(ctx, request.NamespacedName, &rollout)
	if err == nil {
		// Rollout exists, requeue the policyreco
		return ctrl.Result{}, r.requeuePolicyRecommendation(ctx, &rollout, r.Scheme, logger)
	}

	if !errors.IsNotFound(err) {
		// Error occurred
		logger.Error(err, "Failed to get Rollout. Requeue the request")
		return ctrl.Result{RequeueAfter: 1 * time.Second}, err
	}

	// Rollout isn't queued. Check if Deployment is enqueued
	deployment := appsv1.Deployment{}
	err = r.Client.Get(ctx, request.NamespacedName, &deployment)
	if err == nil {
		// Deployment exists, requeue the policyreco
		return ctrl.Result{}, r.requeuePolicyRecommendation(ctx, &deployment, r.Scheme, logger)
	}

	if !errors.IsNotFound(err) {
		logger.Error(err, "Failed to get Deployment. Requeue the request")
		return ctrl.Result{RequeueAfter: 1 * time.Second}, err
	}

	logger.Info("Rollout or Deployment not found.")
	return ctrl.Result{}, nil
}

func (r *DeploymentController) requeuePolicyRecommendation(ctx context.Context,
	object client.Object,
	scheme *runtime.Scheme,
	logger logr.Logger) error {

	now := metav1.Now()
	policyRecommendation := &ottoscaleriov1alpha1.PolicyRecommendation{}

	err := r.Client.Get(context.TODO(), types.NamespacedName{
		Namespace: object.GetNamespace(),
		Name:      object.GetName(),
	}, policyRecommendation)
	if err != nil {
		logger.Error(err, "Error while getting policyRecommendation.", "workloadName", object.GetName(), "workloadNamespace", object.GetNamespace())
		return err
	}

	policyRecommendation.Spec.QueuedForExecution = &trueBool
	policyRecommendation.Spec.QueuedForExecutionAt = &now

	err = r.Client.Update(context.TODO(), policyRecommendation, client.FieldOwner(DeploymentCtrlName))
	if err != nil {
		logger.Error(err, "Error while updating policyRecommendation.", "workloadName", object.GetName(), "workloadNamespace", object.GetNamespace())
		return err
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeploymentController) SetupWithManager(mgr ctrl.Manager) error {
	fmt.Println("Deployment Manager setup")
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

	enqueueFunc := func(obj client.Object) []reconcile.Request {
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: obj.GetName(),
			Namespace: obj.GetNamespace()}}}
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(DeploymentCtrlName).
		Watches(
			&source.Kind{Type: &argov1alpha1.Rollout{}},
			handler.EnqueueRequestsFromMapFunc(enqueueFunc),
			builder.WithPredicates(annotationUpdatePredicate),
		).
		Watches(
			&source.Kind{Type: &appsv1.Deployment{}},
			handler.EnqueueRequestsFromMapFunc(enqueueFunc),
			builder.WithPredicates(annotationUpdatePredicate),
		).
		Complete(r)
}
