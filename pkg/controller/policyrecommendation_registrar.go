package controller

import (
	"context"
	"time"

	argov1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	ottoscaleriov1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	"github.com/flipkart-incubator/ottoscalr/pkg/policy"
	"github.com/flipkart-incubator/ottoscalr/pkg/registry"
	"github.com/flipkart-incubator/ottoscalr/pkg/trigger"
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	policyRecoWorkloadGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{Name: "policyreco_workload_info",
			Help: "PolicyReco and workload mapping"}, []string{"namespace", "policyreco", "workloadKind", "workload"},
	)
)

func init() {
	metrics.Registry.MustRegister(policyRecoWorkloadGauge)
}

const PolicyRecoRegistrarCtrlName = "PolicyRecommendationRegistrar"

// PolicyRecommendationRegistrar reconciles a Deployment or ArgoRollout
// object to ensure a PolicyRecommendation exists.
type PolicyRecommendationRegistrar struct {
	Client               client.Client
	Scheme               *runtime.Scheme
	MonitorManager       trigger.MonitorManager
	RequeueDelayDuration time.Duration
	PolicyStore          policy.Store
	ClientsRegistry      registry.DeploymentClientRegistry
	ExcludedNamespaces   []string
	IncludedNamespaces   []string
}

func NewPolicyRecommendationRegistrar(client client.Client,
	scheme *runtime.Scheme,
	requeueDelayMs int,
	monitorManager trigger.MonitorManager,
	policyStore policy.Store,
	clientsRegistry registry.DeploymentClientRegistry,
	excludedNamespaces []string, includedNamespaces []string) *PolicyRecommendationRegistrar {
	return &PolicyRecommendationRegistrar{
		Client:               client,
		Scheme:               scheme,
		MonitorManager:       monitorManager,
		RequeueDelayDuration: time.Duration(requeueDelayMs) * time.Millisecond,
		PolicyStore:          policyStore,
		ClientsRegistry:      clientsRegistry,
		ExcludedNamespaces:   excludedNamespaces,
		IncludedNamespaces:   includedNamespaces,
	}
}

// +kubebuilder:rbac:groups=argoproj.io,resources=rollouts,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=your-group.io,resources=policyrecommendations,verbs=create;get;list;watch;update;delete
//+kubebuilder:rbac:groups=ottoscaler.io,resources=policyrecommendations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ottoscaler.io,resources=policyrecommendations/finalizers,verbs=update

// TODO neerajb Handle the deletion of workloads. We should reregister the monitors.
func (controller *PolicyRecommendationRegistrar) Reconcile(ctx context.Context,
	request ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger = logger.WithValues("request", request).WithName(PolicyRecoRegistrarCtrlName)

	for _, obj := range controller.ClientsRegistry.Clients {
		object, err := obj.GetObject(request.Namespace, request.Name)
		if err == nil {
			return ctrl.Result{}, controller.handleReconcile(ctx, object, controller.Scheme, logger)
		}
		if errors.IsNotFound(err) {
			policyRecoWorkloadGauge.DeletePartialMatch(prometheus.Labels{"namespace": request.Namespace, "policyreco": request.Name})
		}
		if !errors.IsNotFound(err) {
			// Error occurred
			logger.Error(err, "Failed to get. Requeue the request")
			return ctrl.Result{RequeueAfter: controller.RequeueDelayDuration}, err
		}
	}
	logger.Info("Rollout or Deployment not found. It could have been deleted.")
	return ctrl.Result{}, nil
}

func (controller *PolicyRecommendationRegistrar) createPolicyRecommendation(
	ctx context.Context,
	instance client.Object,
	scheme *runtime.Scheme,
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

	now := metav1.Now()
	newPolicyRecommendation := &ottoscaleriov1alpha1.PolicyRecommendation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.GetName(),
			Namespace: instance.GetNamespace(),
		},
		Spec: ottoscaleriov1alpha1.PolicyRecommendationSpec{
			WorkloadMeta: ottoscaleriov1alpha1.WorkloadMeta{
				Name:     instance.GetName(),
				TypeMeta: metav1.TypeMeta{Kind: gvk.Kind, APIVersion: gvk.GroupVersion().String()}},
			Policy:               safestPolicy.Name,
			TransitionedAt:       &now,
			QueuedForExecution:   &trueBool,
			QueuedForExecutionAt: &now,
		},
	}
	policyRecoWorkloadGauge.WithLabelValues(instance.GetNamespace(), instance.GetName(), gvk.Kind, instance.GetName()).Set(1)

	err = controllerutil.SetControllerReference(instance, newPolicyRecommendation, scheme)
	if err != nil {
		logger.Error(err, "Error setting owner reference - requeue the request")
		return nil, err
	}

	err = controller.Client.Create(ctx, newPolicyRecommendation)
	if err != nil {
		// Error creating the object - requeue the request.
		logger.Error(err, "Error creating the object - requeue the request")
		return nil, err
	}

	var conditions []metav1.Condition
	logger.Info("PolicyRecommendation created successfully")
	statusPatch, conditions := CreatePolicyPatch(*newPolicyRecommendation, conditions, ottoscaleriov1alpha1.Initialized, metav1.ConditionTrue, PolicyRecommendationCreated, InitializedMessage)
	if err := controller.Client.Status().Patch(ctx, statusPatch, client.Apply, getSubresourcePatchOptions(PolicyRecoRegistrarCtrlName)); err != nil {
		logger.Error(err, "Failed to patch the status")
		return nil, client.IgnoreNotFound(err)
	}
	logger.V(1).Info("Initialized Status Patch applied", "patch", *statusPatch)
	// PolicyRecommendation created successfully
	return newPolicyRecommendation, nil
}

func (controller *PolicyRecommendationRegistrar) handleReconcile(ctx context.Context,
	object client.Object,
	scheme *runtime.Scheme,
	logger logr.Logger) error {

	_, err := controller.createPolicyRecommendation(ctx, object, scheme, logger)

	if err == nil {
		controller.MonitorManager.RegisterMonitor(object.GetObjectKind().GroupVersionKind().Kind,
			types.NamespacedName{
				Name:      object.GetName(),
				Namespace: object.GetNamespace(),
			})
	}

	return err
}

// SetupWithManager sets up the controller with the Manager.
func (controller *PolicyRecommendationRegistrar) SetupWithManager(mgr ctrl.Manager) error {
	// TODO: Filter out system and blacklisted namespaces
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

	deletePredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}

	namespaceFilter := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return controller.isWhitelistedNamespace(e.Object.GetNamespace())
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return controller.isWhitelistedNamespace(e.ObjectNew.GetNamespace())
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return controller.isWhitelistedNamespace(e.Object.GetNamespace())
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return controller.isWhitelistedNamespace(e.Object.GetNamespace())
		},
	}

	enqueueFunc := func(ctx context.Context, obj client.Object) []reconcile.Request {
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: obj.GetName(),
			Namespace: obj.GetNamespace()}}}
	}

	controllerBuilder := ctrl.NewControllerManagedBy(mgr).
		Named(PolicyRecoRegistrarCtrlName).
		Watches(
			&ottoscaleriov1alpha1.PolicyRecommendation{},
			handler.EnqueueRequestForOwner(
				mgr.GetScheme(), mgr.GetRESTMapper(), &argov1alpha1.Rollout{},
			),
			builder.WithPredicates(deletePredicate),
		).
		Watches(
			&ottoscaleriov1alpha1.PolicyRecommendation{},
			handler.EnqueueRequestForOwner(
				mgr.GetScheme(), mgr.GetRESTMapper(), &appsv1.Deployment{},
			),
			builder.WithPredicates(deletePredicate),
		)

	for _, object := range controller.ClientsRegistry.Clients {
		controllerBuilder.Watches(
			object.GetObjectType(),
			handler.EnqueueRequestsFromMapFunc(enqueueFunc),
			builder.WithPredicates(createPredicate),
		)
	}

	return controllerBuilder.WithEventFilter(namespaceFilter).
		Complete(controller)
}

func (controller *PolicyRecommendationRegistrar) isWhitelistedNamespace(namespace string) bool {

	if len(controller.IncludedNamespaces) > 0 {
		for _, ns := range controller.IncludedNamespaces {
			if namespace == ns {
				return true
			}
		}
		return false
	}

	if len(controller.ExcludedNamespaces) > 0 {
		for _, ns := range controller.ExcludedNamespaces {
			if namespace == ns {
				return false
			}
		}
		return true

	}

	return true
}
