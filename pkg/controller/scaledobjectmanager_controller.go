package controller

import (
	"context"
	"strconv"
	"time"

	argov1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/flipkart-incubator/ottoscalr/pkg/reco"
	"github.com/go-logr/logr"
	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
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

const (
	HPADecouplingCtrlName = "ScaledObjectManagerController"
)

type ScaledObjectManagerController struct {
	Client client.Client
	Scheme *runtime.Scheme
}

func NewScaledObjectManagerController(client client.Client,
	scheme *runtime.Scheme,
) *ScaledObjectManagerController {
	return &ScaledObjectManagerController{
		Client: client,
		Scheme: scheme,
	}
}

// +kubebuilder:rbac:groups=argoproj.io,resources=rollouts,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=your-group.io,resources=policyrecommendations,verbs=create;get;list;watch;update;delete
//+kubebuilder:rbac:groups=ottoscaler.io,resources=policyrecommendations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ottoscaler.io,resources=policyrecommendations/finalizers,verbs=update

func (r *ScaledObjectManagerController) Reconcile(ctx context.Context,
	request ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger = logger.WithValues("request", request).WithName(HPADecouplingCtrlName)

	// Check if Rollout is enqueued
	rollout := argov1alpha1.Rollout{}
	err := r.Client.Get(ctx, request.NamespacedName, &rollout)
	if err == nil {
		// Rollout exists, update the scaledobject
		return ctrl.Result{}, r.updateScaledObject(ctx, &rollout, r.Scheme, logger)
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
		// Deployment exists, update the scaledobject
		return ctrl.Result{}, r.updateScaledObject(ctx, &deployment, r.Scheme, logger)
	}

	if !errors.IsNotFound(err) {
		logger.Error(err, "Failed to get Deployment. Requeue the request")
		return ctrl.Result{RequeueAfter: 1 * time.Second}, err
	}

	logger.Info("Rollout or Deployment not found.")
	return ctrl.Result{}, nil

}

func (r *ScaledObjectManagerController) updateScaledObject(ctx context.Context,
	object client.Object,
	scheme *runtime.Scheme,
	logger logr.Logger) error {

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		so := &kedav1alpha1.ScaledObject{}
		err := r.Client.Get(ctx, types.NamespacedName{Namespace: object.GetNamespace(), Name: object.GetName()}, so)
		if err != nil {
			if !errors.IsNotFound(err) {
				logger.Error(err, "Failed to get ScaledObject")
				return err
			}
			return nil
		}

		// If scaledobject is not created by ottoscalr
		if so.Labels[createdByLabelKey] != createdByLabelValue {
			return nil
		}

		// Get the max pod annotation value
		maxReplicaValue, ok := object.GetAnnotations()[reco.OttoscalrMaxPodAnnotation]
		if ok {
			maxReplica, err := strconv.Atoi(maxReplicaValue)
			if err != nil {
				logger.Error(err, "Failed to convert maxReplicaValue to int")
				return err
			}
			maxReplicaInt32 := int32(maxReplica)
			if so.Spec.MaxReplicaCount != nil && so.Spec.MinReplicaCount != nil {
				// If maxReplicaCount eqaul to max pod annotation value, return
				if *so.Spec.MaxReplicaCount == maxReplicaInt32 {
					return nil
				}
				//update the scaledobject with max pod annotation value
				so.Spec.MaxReplicaCount = &maxReplicaInt32
				if *so.Spec.MinReplicaCount > maxReplicaInt32 {
					so.Spec.MinReplicaCount = &maxReplicaInt32
				}
			}
		}

		err = r.Client.Update(ctx, so)
		if err != nil {
			logger.Error(err, "Failed to update ScaledObject")
			return err
		}
		logger.Info("ScaledObject updated successfully", "name", so.Name, "namespace", so.Namespace)
		return nil
	})

	if err != nil {
		logger.Error(err, "Failed to update ScaledObject")
		return err
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ScaledObjectManagerController) SetupWithManager(mgr ctrl.Manager) error {
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
			newMaxPods = newObj.GetAnnotations()[reco.OttoscalrMaxPodAnnotation]
			oldMaxPods = oldObj.GetAnnotations()[reco.OttoscalrMaxPodAnnotation]

			// If maxPods annotation is updated
			if newMaxPods != oldMaxPods {
				return true
			}

			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}

	scaledObjectUpdatePredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			newSo := e.ObjectNew.(*kedav1alpha1.ScaledObject)
			oldSo := e.ObjectOld.(*kedav1alpha1.ScaledObject)

			var newMaxReplicaCount, oldMaxReplicaCount int32

			if newSo.Spec.MaxReplicaCount == nil || oldSo.Spec.MaxReplicaCount == nil {
				return false
			}
			newMaxReplicaCount = *newSo.Spec.MaxReplicaCount
			oldMaxReplicaCount = *oldSo.Spec.MaxReplicaCount

			return newMaxReplicaCount != oldMaxReplicaCount

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
		Named(HPADecouplingCtrlName).
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
		Watches(
			&source.Kind{Type: &kedav1alpha1.ScaledObject{}},
			handler.EnqueueRequestsFromMapFunc(enqueueFunc), //Using same enqueue function as deployment name is same as scaledobject name
			builder.WithPredicates(scaledObjectUpdatePredicate),
		).
		Complete(r)
}
