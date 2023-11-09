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
	v1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	"github.com/flipkart-incubator/ottoscalr/pkg/autoscaler"
	"github.com/flipkart-incubator/ottoscalr/pkg/reco"
	"github.com/flipkart-incubator/ottoscalr/pkg/registry"
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strconv"
)

const (
	HPAEnforcementCtrlName           = "HPAEnforcementController"
	createdByLabelKey                = "created-by"
	createdByLabelValue              = "ottoscalr"
	hpaEnforcementDisabledAnnotation = "ottoscalr.io/skip-hpa-enforcement"
	hpaEnforcementEnabledAnnotation  = "ottoscalr.io/enable-hpa-enforcement"
	rolloutWaveAnnotation            = "ottoscalr.io/rollout-wave"
)

var (
	autoscalerField               = ".spec.scaleTargetRef.name"
	policyRecoOwnerField          = ".spec.workloadOwner"
	HPAEnforcedReason             = "ScaledObjectIsCreated"
	HPAEnforcedMessage            = "ScaledObject has been created."
	AutoscalerExistsReason        = "UserCreatedScaledObjectAlreadyExists"
	AutoscalerExistsMessage       = "User managed ScaledObject already exists for this workload."
	InvalidPolicyRecoReason       = "InvalidPolicyRecoConfig"
	InvalidPolicyRecoMessage      = "HPA config in the PolicyRecommendation doesn't qualify for the ScaledObject creation criteria."
	HPAEnforcementDisabledReason  = "HPAEnforcementDisabled"
	HPAEnforcementDisabledMessage = "HPA enforcement disabled for this workload"
)

var (
	hpaenforcerReconcileCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{Name: "hpaenforcer_reconciled_count",
			Help: "Number of policyrecos reconciled counter by HPAEnforcer"}, []string{"namespace", "policyreco"},
	)

	hpaenforcerAutoscalerObjectUpdatedCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{Name: "hpaenforcer_autoscaler_updated_count",
			Help: "Number of scaled objects created/updated by HPAEnforcer"}, []string{"namespace", "policyreco", "autoscaler", "change"},
	)

	hpaenforcerAutoscalerObjectDeletedCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{Name: "hpaenforcer_autoscaler_deleted_count",
			Help: "Number of scaled objects created/updated by HPAEnforcer"}, []string{"namespace", "policyreco", "autoscaler"},
	)
)

func init() {
	metrics.Registry.MustRegister(hpaenforcerAutoscalerObjectUpdatedCounter, hpaenforcerAutoscalerObjectDeletedCounter, hpaenforcerReconcileCounter)
}

type HPAEnforcementController struct {
	client.Client
	Scheme                  *runtime.Scheme
	Recorder                record.EventRecorder
	clientsRegistry         registry.DeploymentClientRegistry
	MaxConcurrentReconciles int
	isDryRun                *bool
	ExcludedNamespaces      *[]string
	IncludedNamespaces      *[]string
	WhitelistMode           *bool
	MinRequiredReplicas     int
	autoscalerClient        autoscaler.AutoscalerClient
}

func NewHPAEnforcementController(client client.Client,
	scheme *runtime.Scheme,clientsRegistry registry.DeploymentClientRegistry, recorder record.EventRecorder,
	maxConcurrentReconciles int, isDryRun *bool, excludedNamespaces *[]string, includedNamespaces *[]string, whitelistMode *bool, minRequiredReplicas int, autoscalerClient autoscaler.AutoscalerClient) (*HPAEnforcementController, error) {

	HPAEnforcedReason = fmt.Sprintf("%sIsCreated", autoscalerClient.GetName())
	HPAEnforcedMessage = fmt.Sprintf("%s has been created.", autoscalerClient.GetName())
	AutoscalerExistsReason = fmt.Sprintf("UserCreated%sAlreadyExists", autoscalerClient.GetName())
	AutoscalerExistsMessage = fmt.Sprintf("User managed %s already exists for this workload.", autoscalerClient.GetName())
	InvalidPolicyRecoMessage = fmt.Sprintf("HPA config in the PolicyRecommendation doesn't qualify for the %s creation criteria.", autoscalerClient.GetName())

	return &HPAEnforcementController{
		Client:                  client,
		Scheme:                  scheme,
		clientsRegistry:         clientsRegistry,
		MaxConcurrentReconciles: maxConcurrentReconciles,
		Recorder:                recorder,
		isDryRun:                isDryRun,
		ExcludedNamespaces:      excludedNamespaces,
		IncludedNamespaces:      includedNamespaces,
		WhitelistMode:           whitelistMode,
		MinRequiredReplicas:     minRequiredReplicas,
		autoscalerClient:        autoscalerClient,
	}, nil
}

//+kubebuilder:rbac:groups=ottoscaler.io,resources=policyrecommendations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ottoscaler.io,resources=policyrecommendations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ottoscaler.io,resources=policyrecommendations/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch;delete

func (r *HPAEnforcementController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	logger := ctrl.LoggerFrom(ctx).WithName(HPAEnforcementCtrlName)

	logger.V(0).Info("Reconciling PolicyRecommendation.", "object", req.NamespacedName)
	if r.ExcludedNamespaces != nil {
		logger.V(0).Info("HPA enforcer initialized with namespace filters.", "blacklist", *r.ExcludedNamespaces)
	}

	if r.IncludedNamespaces != nil {
		logger.V(0).Info("HPA enforcer initialized with namespace filters.", "whitelist", *r.IncludedNamespaces)
	}

	policyreco := v1alpha1.PolicyRecommendation{}
	if err := r.Get(ctx, req.NamespacedName, &policyreco); err != nil {
		logger.V(0).Error(err, "Error fetching PolicyRecommendation resource.")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	hpaenforcerReconcileCounter.WithLabelValues(policyreco.Namespace, policyreco.Name).Inc()

	if !isInitialized(policyreco.Status.Conditions) || !isRecoGenerated(policyreco.Status.Conditions) {
		logger.V(0).Info("Skipping policy enforcement as the policy recommendation is not initialized.")
		return ctrl.Result{}, nil
	}

	object, err := r.clientsRegistry.GetObjectClient(policyreco.Spec.WorkloadMeta.Kind)
	if err != nil {
		return ctrl.Result{}, err
	}
	workload, err := object.GetObject(policyreco.Namespace, policyreco.Spec.WorkloadMeta.Name)

	if err != nil {
		logger.V(0).Info("Skipping policy enforcement as workload can't be fetched.")
		return ctrl.Result{}, err
	}

	labelSelector, err := labels.Parse(fmt.Sprintf("!%s", createdByLabelKey))
	if err != nil {
		logger.V(0).Error(err, "Unable to parse label selector string.")
		return ctrl.Result{}, err
	}
	// List only autoscalerObjects not created by this controller
	var conditions []metav1.Condition
	var statusPatch *v1alpha1.PolicyRecommendation

	autoscalerObjects, err := r.autoscalerClient.GetList(ctx, labelSelector, workload.GetNamespace(), fields.OneTermEqualSelector(autoscalerField, workload.GetName()))
	if err != nil && client.IgnoreNotFound(err) != nil {
		return ctrl.Result{}, err
	}

	if len(autoscalerObjects) > 0 {
		logger.V(0).Info(r.autoscalerClient.GetName()+" managed by a different controller/entity already exists for this workload. Skipping.", "workload", workload, "namespace", workload.GetNamespace(), "kind", workload.GetObjectKind(), "autoscaler", autoscalerObjects)
		statusPatch, conditions = CreatePolicyPatch(policyreco, conditions, v1alpha1.HPAEnforced, metav1.ConditionFalse, AutoscalerExistsReason, AutoscalerExistsMessage)
		if err := r.Status().Patch(ctx, statusPatch, client.Apply, getSubresourcePatchOptions(HPAEnforcementCtrlName)); err != nil {
			logger.Error(err, "Error updating the status of the policy reco object")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		return ctrl.Result{}, nil
	}

	if policyreco.Spec.CurrentHPAConfiguration.Max <= r.MinRequiredReplicas || policyreco.Spec.CurrentHPAConfiguration.Min <= r.MinRequiredReplicas || policyreco.Spec.CurrentHPAConfiguration.Min > policyreco.Spec.CurrentHPAConfiguration.Max {
		logger.V(0).Info("Skipping enforcing autoscaling policy due to less max/min pods in the target reco generated.", "workload", workload, "namespace", workload.GetNamespace(), "kind", workload.GetObjectKind())
		if err := r.deleteControllerManagedAutoscaler(ctx, policyreco, workload, logger); err != nil {
			return ctrl.Result{}, err
		}
		statusPatch, conditions = CreatePolicyPatch(policyreco, conditions, v1alpha1.HPAEnforced, metav1.ConditionFalse, InvalidPolicyRecoReason, InvalidPolicyRecoMessage)
		if err := r.Status().Patch(ctx, statusPatch, client.Apply, getSubresourcePatchOptions(HPAEnforcementCtrlName)); err != nil {
			logger.Error(err, "Error updating the status of the policy reco object")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		return ctrl.Result{}, nil
	}

	// Whitelist or Blacklist mode helps process workloads which are either marked for enable or not disable. Workloads without this annotation will
	// be skipped (whitelist mode) or processed (blacklist mode).
	if *r.WhitelistMode {
		if v, ok := workload.GetAnnotations()[hpaEnforcementEnabledAnnotation]; ok {
			if allow, _ := strconv.ParseBool(v); !allow {
				logger.V(0).Info("HPA enforcement is disabled for this workload as it's not marked with ottoscalr.io/enable-hpa-enforcement: true . Skipping.", "workload", workload, "namespace", workload.GetNamespace(), "kind", workload.GetObjectKind())
				if err := r.deleteControllerManagedAutoscaler(ctx, policyreco, workload, logger); err != nil {
					return ctrl.Result{}, err
				}
				statusPatch, conditions = CreatePolicyPatch(policyreco, conditions, v1alpha1.HPAEnforced, metav1.ConditionFalse, HPAEnforcementDisabledReason, HPAEnforcementDisabledMessage)
				if err := r.Status().Patch(ctx, statusPatch, client.Apply, getSubresourcePatchOptions(HPAEnforcementCtrlName)); err != nil {
					logger.Error(err, "Error updating the status of the policy reco object")
					return ctrl.Result{}, client.IgnoreNotFound(err)
				}
				return ctrl.Result{}, nil
			}
			// else continue with autoscaler creation
		} else {
			logger.V(0).Info("HPA enforcement is disabled for this workload as it's not marked with ottoscalr.io/enable-hpa-enforcement: true . Skipping.", "workload", workload, "namespace", workload.GetNamespace(), "kind", workload.GetObjectKind())
			if err := r.deleteControllerManagedAutoscaler(ctx, policyreco, workload, logger); err != nil {
				return ctrl.Result{}, err
			}
			statusPatch, conditions = CreatePolicyPatch(policyreco, conditions, v1alpha1.HPAEnforced, metav1.ConditionFalse, HPAEnforcementDisabledReason, HPAEnforcementDisabledMessage)
			if err := r.Status().Patch(ctx, statusPatch, client.Apply, getSubresourcePatchOptions(HPAEnforcementCtrlName)); err != nil {
				logger.Error(err, "Error updating the status of the policy reco object")
				return ctrl.Result{}, client.IgnoreNotFound(err)
			}
			return ctrl.Result{}, nil
		}
	} else {
		if v, ok := workload.GetAnnotations()[hpaEnforcementDisabledAnnotation]; ok {
			if disallow, _ := strconv.ParseBool(v); disallow {
				logger.V(0).Info("HPA enforcement is disabled for this workload as it's marked with ottoscalr.io/skip-hpa-enforcement: true . Skipping.", "workload", workload, "namespace", workload.GetNamespace(), "kind", workload.GetObjectKind())
				if err := r.deleteControllerManagedAutoscaler(ctx, policyreco, workload, logger); err != nil {
					return ctrl.Result{}, err
				}
				statusPatch, conditions = CreatePolicyPatch(policyreco, conditions, v1alpha1.HPAEnforced, metav1.ConditionFalse, HPAEnforcementDisabledReason, HPAEnforcementDisabledMessage)
				if err := r.Status().Patch(ctx, statusPatch, client.Apply, getSubresourcePatchOptions(HPAEnforcementCtrlName)); err != nil {
					logger.Error(err, "Error updating the status of the policy reco object")
					return ctrl.Result{}, client.IgnoreNotFound(err)
				}
				return ctrl.Result{}, nil
			}
			// else continue with autoscaler creation
		}
	}

	logger.V(0).Info("Reconciling PolicyRecommendation to create/update " + r.autoscalerClient.GetName())
	labels := map[string]string{
		createdByLabelKey: createdByLabelValue,
	}

	min := int32(policyreco.Spec.CurrentHPAConfiguration.Min)
	max := int32(policyreco.Spec.CurrentHPAConfiguration.Max)
	targetCPU := int32(policyreco.Spec.CurrentHPAConfiguration.TargetMetricValue)

	if !*r.isDryRun {

		logger.V(0).Info("Creating/Updating "+r.autoscalerClient.GetName()+" for workload.", "workload", workload.GetName())

		result, err := r.autoscalerClient.CreateOrUpdateAutoscaler(ctx, workload, labels, max, min, targetCPU)
		if err != nil {
			logger.V(0).Error(err, "Error creating or updating "+r.autoscalerClient.GetName())
			return ctrl.Result{}, err
		} else {
			hpaenforcerAutoscalerObjectUpdatedCounter.WithLabelValues(policyreco.Namespace, policyreco.Name, workload.GetName(), result).Inc()
			logger.V(0).Info(fmt.Sprintf("Result of the create or update operation is '%s\n'", result))
		}

	} else {
		logger.V(0).Info("Skipping creating "+r.autoscalerClient.GetName()+" for workload as the controller is deployed in dryRun mode.", "workload", workload.GetName())
		return ctrl.Result{}, nil
	}

	statusPatch, conditions = CreatePolicyPatch(policyreco, conditions, v1alpha1.HPAEnforced, metav1.ConditionTrue, HPAEnforcedReason, HPAEnforcedMessage)
	if err := r.Status().Patch(ctx, statusPatch, client.Apply, getSubresourcePatchOptions(HPAEnforcementCtrlName)); err != nil {
		logger.Error(err, "Error updating the status of the policy reco object")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	r.Recorder.Event(&policyreco, eventTypeNormal, r.autoscalerClient.GetName()+"Created", fmt.Sprintf("The %s has been created successfully.", r.autoscalerClient.GetName()))

	return ctrl.Result{}, nil
}

func isRecoGenerated(conditions []metav1.Condition) bool {
	for _, condition := range conditions {
		if condition.Type == string(v1alpha1.RecoTaskProgress) {
			if condition.Reason == RecoTaskRecommendationGenerated {
				return true
			}
		}
	}
	return false
}

func isInitialized(conditions []metav1.Condition) bool {
	for _, condition := range conditions {
		if condition.Type == string(v1alpha1.Initialized) {
			if condition.Status == metav1.ConditionTrue {
				return true
			}
		}
	}
	return false
}

// SetupWithManager sets up the controller with the Manager.
func (r *HPAEnforcementController) SetupWithManager(mgr ctrl.Manager) error {

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), r.autoscalerClient.GetType(), autoscalerField, func(rawObj client.Object) []string {
		scaleTargetName := r.autoscalerClient.GetScaleTargetName(rawObj)
		if scaleTargetName == "" {
			return nil
		}
		return []string{scaleTargetName}
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &v1alpha1.PolicyRecommendation{}, policyRecoOwnerField, func(rawObj client.Object) []string {
		policyreco := rawObj.(*v1alpha1.PolicyRecommendation)
		if len(policyreco.GetOwnerReferences()) == 0 {
			return nil
		}
		owners := make([]string, len(policyreco.GetOwnerReferences()))
		for _, owner := range policyreco.GetOwnerReferences() {
			owners = append(owners, owner.Name)
		}
		return owners
	}); err != nil {
		return err
	}

	updatePredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			newObj := e.ObjectNew.(*v1alpha1.PolicyRecommendation)
			oldObj := e.ObjectOld.(*v1alpha1.PolicyRecommendation)

			var newHPAEnforcedCondition, oldHPAEnforcedCondition metav1.Condition
			for _, condition := range newObj.Status.Conditions {
				if condition.Type == string(v1alpha1.HPAEnforced) {
					newHPAEnforcedCondition = condition
				}
			}
			for _, condition := range oldObj.Status.Conditions {
				if condition.Type == string(v1alpha1.HPAEnforced) {
					oldHPAEnforcedCondition = condition
				}
			}

			// This ensures that the updates to conditions managed by this controller don't fork recursive updates on the policyrecos
			if conditionChanged(oldHPAEnforcedCondition, newHPAEnforcedCondition) {
				return false
			}
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}

	_ = predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return true
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

			annotationChangedPredicate := predicate.AnnotationChangedPredicate{}

			if annotationChangedPredicate.Update(e) {
				if newMaxPods != oldMaxPods {
					return true
				}
			}
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}

	namespaceFilter := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return r.isWhitelistedNamespace(e.Object.GetNamespace())
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return r.isWhitelistedNamespace(e.ObjectNew.GetNamespace())
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return r.isWhitelistedNamespace(e.Object.GetNamespace())
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return r.isWhitelistedNamespace(e.Object.GetNamespace())
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

	enqueueFunc := func(ctx context.Context,obj client.Object) []reconcile.Request {
		object := r.autoscalerClient.GetType()
		if len(object.GetOwnerReferences()) == 0 {
			return nil
		}
		for _, owner := range object.GetOwnerReferences() {
			_, err := r.clientsRegistry.GetObjectClient(owner.Kind)
			if err != nil {
				policyRecos := &v1alpha1.PolicyRecommendationList{}
				if err := r.List(context.Background(), policyRecos, &client.ListOptions{
					FieldSelector: fields.OneTermEqualSelector(policyRecoOwnerField, owner.Name),
					Namespace:     object.GetNamespace(),
				}); err != nil {
					return nil
				}
				requests := make([]reconcile.Request, 1)
				for _, policyReco := range policyRecos.Items {
					requests = append(requests, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Namespace: policyReco.Namespace,
							Name:      policyReco.Name,
						},
					})
				}
				mgr.GetLogger().Info("Requeing policyrecos")
				return requests
			}
		}
		mgr.GetLogger().Info("Not requeuing any policyreco.", "object", obj)
		return nil
	}

	policyrecoEnqueueFunc := func(ctx context.Context, obj client.Object) []reconcile.Request {
		mgr.GetLogger().Info("Deployment/Rollout change predicated invoked.")
		mgr.GetLogger().Info("Updates to workload received.", "object", obj.GetName())
		// since there's no support in controller runtime to figure out the Kind of the obj skipping the checks https://github.com/kubernetes-sigs/controller-runtime/issues/1735
		// this predicate is used only for Deployment/Rollout changes, so a Kind check should not be necessary
		policyRecos := &v1alpha1.PolicyRecommendationList{}
		if err := r.List(context.Background(), policyRecos, &client.ListOptions{
			FieldSelector: fields.OneTermEqualSelector(policyRecoOwnerField, obj.GetName()),
			Namespace:     obj.GetNamespace(),
		}); err != nil {
			mgr.GetLogger().Error(err, "Error fetching the policy recommendations list given the workload.")
			return nil
		}
		var requests []reconcile.Request
		for _, policyReco := range policyRecos.Items {
			mgr.GetLogger().Info("Queueing policreco request", "policyreco", policyReco.TypeMeta.String())
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: policyReco.Namespace,
					Name:      policyReco.Name,
				},
			})
		}
		mgr.GetLogger().Info("Reconcile reqs", "reqs", requests)
		return requests
	}

	controllerBuilder := ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: r.MaxConcurrentReconciles}).
		Named(HPAEnforcementCtrlName).
		Watches(
			&v1alpha1.PolicyRecommendation{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(predicate.And(predicate.ResourceVersionChangedPredicate{}, updatePredicate, namespaceFilter)),
		).
		Watches(r.autoscalerClient.GetType(),
			handler.EnqueueRequestsFromMapFunc(enqueueFunc),
			builder.WithPredicates(deletePredicate),
		)

	for _, object := range r.clientsRegistry.Clients {
		controllerBuilder.Watches(
			object.GetObjectType(),
			handler.EnqueueRequestsFromMapFunc(policyrecoEnqueueFunc),
			builder.WithPredicates(predicate.And(predicate.AnnotationChangedPredicate{}, namespaceFilter)),
		)
	}

	return controllerBuilder.
		WithEventFilter(namespaceFilter).
		Complete(r)
}

func conditionChanged(oldCond metav1.Condition, newCond metav1.Condition) bool {
	if oldCond.Type != newCond.Type || oldCond.Status != newCond.Status || oldCond.Reason != newCond.Reason ||
		!oldCond.LastTransitionTime.Equal(&newCond.LastTransitionTime) || oldCond.Message != newCond.Message ||
		oldCond.ObservedGeneration != newCond.ObservedGeneration {
		return true
	}
	return false
}

func (r *HPAEnforcementController) isWhitelistedNamespace(namespace string) bool {
	if r.IncludedNamespaces != nil && len(*r.IncludedNamespaces) > 0 {
		for _, ns := range *r.IncludedNamespaces {
			if namespace == ns {
				return true
			}
		}
		return false
	}

	if r.ExcludedNamespaces != nil && len(*r.ExcludedNamespaces) > 0 {
		for _, ns := range *r.ExcludedNamespaces {
			if namespace == ns {
				return false
			}
		}
		return true
	}

	return true
}

func (r *HPAEnforcementController) deleteControllerManagedAutoscaler(ctx context.Context, policyreco v1alpha1.PolicyRecommendation, workload client.Object, logger logr.Logger) error {
	labelSelector, err := labels.Parse(fmt.Sprintf("%s=%s", createdByLabelKey, createdByLabelValue))
	if err != nil {
		logger.V(0).Error(err, "Unable to parse label selector string.")
		return err
	}
	var maxPods int32
	// List only autoscalerObjects created by this controller

	autoscalerObjects, err := r.autoscalerClient.GetList(context.Background(), labelSelector, workload.GetNamespace(), fields.OneTermEqualSelector(autoscalerField, workload.GetName()))
	if err != nil && client.IgnoreNotFound(err) != nil {
		return err
	}
	if len(autoscalerObjects) == 0 {
		return nil
	}
	logger.V(0).Info(fmt.Sprintf("Found %d %s(s) for policyreco %s.", len(autoscalerObjects), r.autoscalerClient.GetName(), policyreco.Name))
	logger.V(0).Info("Deleting the " + r.autoscalerClient.GetName() + " and resetting workload spec.replicas")

	for _, autoscalerObject := range autoscalerObjects {
		maxPods = r.autoscalerClient.GetMaxReplicaCount(autoscalerObject)
		err := r.autoscalerClient.DeleteAutoscaler(context.Background(), autoscalerObject)
		if err != nil {
			logger.V(0).Error(err, "Error while deleting the "+r.autoscalerClient.GetName(), r.autoscalerClient.GetName(), autoscalerObject)
			return client.IgnoreNotFound(err)
		}
		r.Recorder.Event(&policyreco, eventTypeNormal, r.autoscalerClient.GetName()+"Deleted", fmt.Sprintf("The %s '%s' has been deleted.", r.autoscalerClient.GetName(), autoscalerObject.GetName()))
		hpaenforcerAutoscalerObjectDeletedCounter.WithLabelValues(policyreco.Namespace, policyreco.Name, autoscalerObject.GetName()).Inc()
		logger.V(0).Info("Deleted "+r.autoscalerClient.GetName()+" for the policyreco.", "policyreco.name", policyreco.GetName(), "policyreco.namespace", policyreco.GetNamespace(), "autoscaler.name", autoscalerObject.GetName(), "autoscaler.namespace", autoscalerObject.GetNamespace(), "maxReplicas", maxPods)
	}

	if maxPods == 0 {
		logger.Info(r.autoscalerClient.GetName() + " maxReplicas is not configured. Not resetting the workload.spec.replicas.")
		return nil
	}

	object, err := r.clientsRegistry.GetObjectClient(workload.GetObjectKind().GroupVersionKind().Kind)
	if err != nil {
		return err
	}

	logger.V(0).Info("Patching the workload to update spec.replicas.", "workloadName", workload.GetName(), "workloadNamespace", workload.GetNamespace(), "maxReplicas", maxPods)
	err = object.Scale(workload.GetNamespace(), workload.GetName(), maxPods)

	if err != nil {
		logger.Error(err, "Error patching the workload")
		return err
	}
	r.Recorder.Event(&policyreco, eventTypeNormal, r.autoscalerClient.GetName()+"Deleted", fmt.Sprintf("Workload has be rescaled to max replicas '%d' from the deleted "+r.autoscalerClient.GetName(), maxPods))
	return nil
}
