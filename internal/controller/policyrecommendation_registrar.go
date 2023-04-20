package controller

import (
	"context"
	argov1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	ottoscaleriov1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	"github.com/flipkart-incubator/ottoscalr/internal/policy"
	"github.com/flipkart-incubator/ottoscalr/internal/trigger"
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

// PolicyRecommendationRegistrar reconciles a Deployment or ArgoRollout
// object to ensure a PolicyRecommendation exists.
type PolicyRecommendationRegistrar struct {
	Client               client.Client
	Scheme               *runtime.Scheme
	BreachMonitorManager trigger.MonitorManager
	RequeueDelayDuration time.Duration
	PolicyStore          policy.Store
}

func NewPolicyRecommendationRegistrar(client client.Client,
	scheme *runtime.Scheme,
	breachMonitorManager trigger.MonitorManager,
	requeueDelayMs int,
	policyStore policy.Store) *PolicyRecommendationRegistrar {
	return &PolicyRecommendationRegistrar{
		Client:               client,
		Scheme:               scheme,
		BreachMonitorManager: breachMonitorManager,
		RequeueDelayDuration: time.Duration(requeueDelayMs) * time.Millisecond,
		PolicyStore:          policyStore,
	}
}

// +kubebuilder:rbac:groups=argoproj.io,resources=rollouts,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=your-group.io,resources=policyrecommendations,verbs=create;get;list;watch;update;delete
//+kubebuilder:rbac:groups=ottoscaler.io,resources=policyrecommendations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ottoscaler.io,resources=policyrecommendations/finalizers,verbs=update

func (controller *PolicyRecommendationRegistrar) Reconcile(ctx context.Context,
	request ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger = logger.WithValues("request", request)

	// Check if Rollout exists
	rollout := argov1alpha1.Rollout{}
	err := controller.Client.Get(ctx, request.NamespacedName, &rollout)
	if err == nil {
		// Rollout exists, create policy recommendation
		return ctrl.Result{}, controller.handleReconcile(ctx, &rollout, logger)
	}

	if !errors.IsNotFound(err) {
		// Error occurred
		logger.Error(err, "Failed to get Rollout. Requeue the request")
		return ctrl.Result{RequeueAfter: controller.RequeueDelayDuration}, err
	}

	// Rollout doesn't exist. Check if Deployment exists
	deployment := appsv1.Deployment{}
	err = controller.Client.Get(ctx, request.NamespacedName, &deployment)
	if err == nil {
		// Deployment exists, create policy recommendation
		return ctrl.Result{}, controller.handleReconcile(ctx, &deployment, logger)
	}

	if !errors.IsNotFound(err) {
		logger.Error(err, "Failed to get Deployment. Requeue the request")
		return ctrl.Result{RequeueAfter: controller.RequeueDelayDuration}, err
	}

	logger.Info("Rollout or Deployment not found. It could have been deleted.")
	return ctrl.Result{}, nil
}

func (controller *PolicyRecommendationRegistrar) createPolicyRecommendation(
	ctx context.Context,
	instance client.Object,
	logger logr.Logger) (*ottoscaleriov1alpha1.PolicyRecommendation, error) {

	// Check if a PolicyRecommendation object already exists
	policyRecommendation := &ottoscaleriov1alpha1.PolicyRecommendation{}
	err := controller.Client.Get(ctx, types.NamespacedName{Name: instance.GetName(), Namespace: instance.GetNamespace()}, policyRecommendation)
	if err == nil {
		logger.Info("PolicyRecommendation object already exists")
		return nil, nil
	} else if !errors.IsNotFound(err) {
		logger.Error(err, "Error reading the object - requeue the request")
		return nil, err
	}

	safestPolicy, err := controller.PolicyStore.GetSafestPolicy()
	if err != nil {
		logger.Error(err, "Error getting the safest policy - requeue the request")
		return nil, err
	}

	gvk := instance.GetObjectKind().GroupVersionKind()
	logger.Info("Creating a new PolicyRecommendation object", "GroupVersionKind", gvk)

	newPolicyRecommendation := &ottoscaleriov1alpha1.PolicyRecommendation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.GetName(),
			Namespace: instance.GetNamespace(),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(instance, gvk),
			},
		},
		Spec: ottoscaleriov1alpha1.PolicyRecommendationSpec{
			WorkloadSpec: ottoscaleriov1alpha1.WorkloadSpec{Name: instance.GetName(),
				TypeMeta: metav1.TypeMeta{Kind: gvk.Kind, APIVersion: gvk.GroupVersion().String()}},
			Policy:               *safestPolicy,
			QueuedForExecution:   true,
			QueuedForExecutionAt: metav1.NewTime(time.Now()),
		},
	}

	err = controller.Client.Create(ctx, newPolicyRecommendation)
	if err != nil {
		// Error creating the object - requeue the request.
		logger.Error(err, "Error creating the object - requeue the request")
		return nil, err
	}

	logger.Info("PolicyRecommendation created successfully")
	// PolicyRecommendation created successfully
	return newPolicyRecommendation, nil
}

func (controller *PolicyRecommendationRegistrar) handleReconcile(ctx context.Context,
	object client.Object,
	logger logr.Logger) error {

	_, err := controller.createPolicyRecommendation(ctx, object, logger)

	if err == nil {
		controller.BreachMonitorManager.RegisterBreachMonitor(object.GetObjectKind().GroupVersionKind().Kind,
			types.NamespacedName{
				Name:      object.GetName(),
				Namespace: object.GetNamespace(),
			})
	}
	return err
}

// SetupWithManager sets up the controller with the Manager.
func (controller *PolicyRecommendationRegistrar) SetupWithManager(mgr ctrl.Manager) error {
	createPredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
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
		Named("PolicyRecommendationRegistrar").
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
		Complete(controller)
}
