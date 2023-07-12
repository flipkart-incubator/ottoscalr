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
	argov1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	v1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	"github.com/go-logr/logr"
	kedaapi "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	v1 "k8s.io/api/apps/v1"
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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
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
	scaledObjectField             = ".spec.scaleTargetRef.name"
	policyRecoOwnerField          = ".spec.workloadOwner"
	HPAEnforcedReason             = "ScaledObjectIsCreated"
	HPAEnforcedMessage            = "ScaledObject has been created."
	ScaledObjectExistsReason      = "UserCreatedScaledObjectAlreadyExists"
	ScaledObjectExistsMessage     = "User managed ScaledObject already exists for this workload."
	InvalidPolicyRecoReason       = "InvalidPolicyRecoConfig"
	InvalidPolicyRecoMessage      = "HPA config in the PolicyRecommendation doesn't qualify for the ScaledObject creation criteria."
	HPAEnforcementDisabledReason  = "HPAEnforcementDisabled"
	HPAEnforcementDisabledMessage = "HPA enforcement disabled for this workload"
)

func init() {
}

// PolicyRecommendationReconciler reconciles a PolicyRecommendation object
type HPAEnforcementController struct {
	client.Client
	Scheme                  *runtime.Scheme
	Recorder                record.EventRecorder
	MaxConcurrentReconciles int
	isDryRun                *bool
	ExcludedNamespaces      *[]string
	IncludedNamespaces      *[]string
	WhitelistMode           *bool
}

func NewHPAEnforcementController(client client.Client,
	scheme *runtime.Scheme, recorder record.EventRecorder,
	maxConcurrentReconciles int, isDryRun *bool, excludedNamespaces *[]string, includedNamespaces *[]string, whitelistMode *bool) (*HPAEnforcementController, error) {
	return &HPAEnforcementController{
		Client:                  client,
		Scheme:                  scheme,
		MaxConcurrentReconciles: maxConcurrentReconciles,
		Recorder:                recorder,
		isDryRun:                isDryRun,
		ExcludedNamespaces:      excludedNamespaces,
		IncludedNamespaces:      includedNamespaces,
		WhitelistMode:           whitelistMode,
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
		logger.V(0).Info("Error fetching PolicyRecommendation resource.")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !isInitialized(policyreco.Status.Conditions) {
		logger.V(0).Info("Skipping policy enforcement as the policy recommendation is not initialized.")
		return ctrl.Result{}, nil
	}

	var workload client.Object
	if policyreco.Spec.WorkloadMeta.Kind == "Rollout" {
		workload = &argov1alpha1.Rollout{}
	} else if policyreco.Spec.WorkloadMeta.Kind == "Deployment" || policyreco.Spec.WorkloadMeta.Kind == "" {
		workload = &v1.Deployment{}
	} else {
		logger.V(0).Info("Skipping policy enforcement due to unrecognizable target workload meta.")
		return ctrl.Result{}, nil
	}

	if err := r.Get(ctx, types.NamespacedName{
		Namespace: policyreco.Namespace,
		Name:      policyreco.Spec.WorkloadMeta.Name,
	}, workload); err != nil {
		logger.V(0).Info("Skipping policy enforcement as workload can't be fetched.")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	scaledObjects := &kedaapi.ScaledObjectList{}
	labelSelector, err := labels.Parse(fmt.Sprintf("!%s", createdByLabelKey))
	if err != nil {
		logger.V(0).Error(err, "Unable to parse label selector string.")
		return ctrl.Result{}, err
	}
	// List only scaledObjects not created by this controller
	if err := r.List(ctx, scaledObjects, &client.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(scaledObjectField, workload.GetName()),
		LabelSelector: labelSelector,
		Namespace:     workload.GetNamespace(),
	}); err != nil && client.IgnoreNotFound(err) != nil {
		return ctrl.Result{}, err
	}

	var conditions []metav1.Condition
	var statusPatch *v1alpha1.PolicyRecommendation
	if len(scaledObjects.Items) > 0 {
		logger.V(0).Info("ScaledObject managed by a different controller/entity already exists for this workload. Skipping.", "workload", workload, "namespace", workload.GetNamespace(), "kind", workload.GetObjectKind(), "scaledobject", scaledObjects)
		statusPatch, conditions = CreatePolicyPatch(policyreco, conditions, v1alpha1.HPAEnforced, metav1.ConditionFalse, ScaledObjectExistsReason, ScaledObjectExistsMessage)
		if err := r.Status().Patch(ctx, statusPatch, client.Apply, getSubresourcePatchOptions(HPAEnforcementCtrlName)); err != nil {
			logger.Error(err, "Error updating the status of the policy reco object")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		return ctrl.Result{}, nil
	}

	if int32(policyreco.Spec.CurrentHPAConfiguration.Max) <= 3 || int32(policyreco.Spec.CurrentHPAConfiguration.Min) <= 3 || policyreco.Spec.CurrentHPAConfiguration.Min > policyreco.Spec.CurrentHPAConfiguration.Max {
		logger.V(0).Info("Skipping enforcing autoscaling policy due to less max/min pods in the target reco generated.", "workload", workload, "namespace", workload.GetNamespace(), "kind", workload.GetObjectKind())
		if err := r.deleteControllerManagedScaledObject(ctx, policyreco, workload, logger); err != nil {
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
				if err := r.deleteControllerManagedScaledObject(ctx, policyreco, workload, logger); err != nil {
					return ctrl.Result{}, err
				}
				statusPatch, conditions = CreatePolicyPatch(policyreco, conditions, v1alpha1.HPAEnforced, metav1.ConditionFalse, HPAEnforcementDisabledReason, HPAEnforcementDisabledMessage)
				if err := r.Status().Patch(ctx, statusPatch, client.Apply, getSubresourcePatchOptions(HPAEnforcementCtrlName)); err != nil {
					logger.Error(err, "Error updating the status of the policy reco object")
					return ctrl.Result{}, client.IgnoreNotFound(err)
				}
			}
		} else {
			logger.V(0).Info("HPA enforcement is disabled for this workload as it's not marked with ottoscalr.io/enable-hpa-enforcement: true . Skipping.", "workload", workload, "namespace", workload.GetNamespace(), "kind", workload.GetObjectKind())
			if err := r.deleteControllerManagedScaledObject(ctx, policyreco, workload, logger); err != nil {
				return ctrl.Result{}, err
			}
			statusPatch, conditions = CreatePolicyPatch(policyreco, conditions, v1alpha1.HPAEnforced, metav1.ConditionFalse, HPAEnforcementDisabledReason, HPAEnforcementDisabledMessage)
			if err := r.Status().Patch(ctx, statusPatch, client.Apply, getSubresourcePatchOptions(HPAEnforcementCtrlName)); err != nil {
				logger.Error(err, "Error updating the status of the policy reco object")
				return ctrl.Result{}, client.IgnoreNotFound(err)
			}
		}
	} else {
		if v, ok := workload.GetAnnotations()[hpaEnforcementDisabledAnnotation]; ok {
			if disallow, _ := strconv.ParseBool(v); disallow {
				logger.V(0).Info("HPA enforcement is disabled for this workload as it's marked with ottoscalr.io/skip-hpa-enforcement: true . Skipping.", "workload", workload, "namespace", workload.GetNamespace(), "kind", workload.GetObjectKind())
				if err := r.deleteControllerManagedScaledObject(ctx, policyreco, workload, logger); err != nil {
					return ctrl.Result{}, err
				}
				statusPatch, conditions = CreatePolicyPatch(policyreco, conditions, v1alpha1.HPAEnforced, metav1.ConditionFalse, HPAEnforcementDisabledReason, HPAEnforcementDisabledMessage)
				if err := r.Status().Patch(ctx, statusPatch, client.Apply, getSubresourcePatchOptions(HPAEnforcementCtrlName)); err != nil {
					logger.Error(err, "Error updating the status of the policy reco object")
					return ctrl.Result{}, client.IgnoreNotFound(err)
				}
			}
		}
	}

	logger.V(0).Info("Reconciling PolicyRecommendation to create/update ScaleObject.")
	min := int32(policyreco.Spec.CurrentHPAConfiguration.Min)
	max := int32(policyreco.Spec.CurrentHPAConfiguration.Max)
	scaleTriggers := []kedaapi.ScaleTriggers{
		{
			Type: "cpu",
			Metadata: map[string]string{
				"type":  "Utilization",
				"value": fmt.Sprint(policyreco.Spec.CurrentHPAConfiguration.TargetMetricValue),
			},
		},
	}
	if isEventScalerEnabled(workload) {
		scaleTriggers = append(scaleTriggers, kedaapi.ScaleTriggers{
			Type: "scheduled-event",
			Metadata: map[string]string{
				"scalingStrategy": "scaleToMax",
			},
		})
	}
	if !*r.isDryRun {
		logger.V(0).Info("Creating/Updating ScaledObject for workload.", "workload", workload.GetName())

		scaledObj := kedaapi.ScaledObject{
			ObjectMeta: metav1.ObjectMeta{
				Name:      workload.GetName(),
				Namespace: workload.GetNamespace(),
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         workload.GetObjectKind().GroupVersionKind().GroupVersion().String(),
					Kind:               workload.GetObjectKind().GroupVersionKind().Kind,
					Name:               workload.GetName(),
					UID:                workload.GetUID(),
					Controller:         &trueBool,
					BlockOwnerDeletion: &trueBool,
				}},
				Labels: map[string]string{
					createdByLabelKey: createdByLabelValue,
				},
			},
			Spec: kedaapi.ScaledObjectSpec{
				ScaleTargetRef: &kedaapi.ScaleTarget{
					Name:       workload.GetName(),
					APIVersion: workload.GetObjectKind().GroupVersionKind().GroupVersion().String(),
					Kind:       workload.GetObjectKind().GroupVersionKind().Kind,
				},
				MinReplicaCount: &min,
				MaxReplicaCount: &max,
				Triggers:        scaleTriggers,
			},
		}
		// Add triggers
		if result, err := controllerutil.CreateOrUpdate(ctx, r.Client, &scaledObj, func() error {
			scaledObj.Spec = kedaapi.ScaledObjectSpec{
				ScaleTargetRef: &kedaapi.ScaleTarget{
					Name:       workload.GetName(),
					APIVersion: workload.GetObjectKind().GroupVersionKind().GroupVersion().String(),
					Kind:       workload.GetObjectKind().GroupVersionKind().Kind,
				},
				MinReplicaCount: &min,
				MaxReplicaCount: &max,
				Triggers:        scaleTriggers,
			}
			return nil
		}); err != nil {
			logger.V(0).Error(err, "Error creating or updating scaledobject")
			return ctrl.Result{}, err
		} else {
			logger.V(0).Info(fmt.Sprintf("Result of the create or update operation is '%s\n'", result))
		}

	} else {
		logger.V(0).Info("Skipping creating ScaledObject for workload as the controller is deployed in dryRun mode.", "workload", workload.GetName())
		return ctrl.Result{}, nil
	}

	statusPatch, conditions = CreatePolicyPatch(policyreco, conditions, v1alpha1.HPAEnforced, metav1.ConditionTrue, HPAEnforcedReason, HPAEnforcedMessage)
	if err := r.Status().Patch(ctx, statusPatch, client.Apply, getSubresourcePatchOptions(HPAEnforcementCtrlName)); err != nil {
		logger.Error(err, "Error updating the status of the policy reco object")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	r.Recorder.Event(&policyreco, eventTypeNormal, "ScaledObjectCreated", fmt.Sprintf("The ScaledObject has been created successfully."))

	return ctrl.Result{}, nil
}

func isEventScalerEnabled(workload client.Object) bool {
	//TODO: define based on annotation
	return true
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
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &kedaapi.ScaledObject{}, scaledObjectField, func(rawObj client.Object) []string {
		scaledObject := rawObj.(*kedaapi.ScaledObject)
		if scaledObject.Spec.ScaleTargetRef.Name == "" {
			return nil
		}
		return []string{scaledObject.Spec.ScaleTargetRef.Name}
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
			return true
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

	enqueueFunc := func(obj client.Object) []reconcile.Request {
		scaledObject := obj.(*kedaapi.ScaledObject)
		if len(scaledObject.OwnerReferences) == 0 {
			return nil
		}
		for _, owner := range scaledObject.OwnerReferences {
			if owner.Kind == "Deployment" || owner.Kind == "Rollout" {
				policyRecos := &v1alpha1.PolicyRecommendationList{}
				if err := r.List(context.Background(), policyRecos, &client.ListOptions{
					FieldSelector: fields.OneTermEqualSelector(policyRecoOwnerField, owner.Name),
					Namespace:     scaledObject.GetNamespace(),
				}); err != nil && client.IgnoreNotFound(err) != nil {
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
				return requests
			}
		}

		return nil
	}

	policyrecoEnqueueFunc := func(obj client.Object) []reconcile.Request {
		if obj.GetObjectKind().GroupVersionKind().Kind == "Deployment" || obj.GetObjectKind().GroupVersionKind().Kind == "Rollout" {
			policyRecos := &v1alpha1.PolicyRecommendationList{}
			if err := r.List(context.Background(), policyRecos, &client.ListOptions{
				FieldSelector: fields.OneTermEqualSelector(policyRecoOwnerField, obj.GetName()),
				Namespace:     obj.GetNamespace(),
			}); err != nil && client.IgnoreNotFound(err) != nil {
				return nil
			}
			var requests []reconcile.Request
			for _, policyReco := range policyRecos.Items {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: policyReco.Namespace,
						Name:      policyReco.Name,
					},
				})
			}
			return requests
		}
		return nil
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: r.MaxConcurrentReconciles}).
		Named(HPAEnforcementCtrlName).
		Watches(
			&source.Kind{Type: &v1alpha1.PolicyRecommendation{}},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(predicate.And(predicate.ResourceVersionChangedPredicate{}, updatePredicate, namespaceFilter)),
		).
		Watches(
			&source.Kind{Type: &kedaapi.ScaledObject{}},
			handler.EnqueueRequestsFromMapFunc(enqueueFunc),
			builder.WithPredicates(deletePredicate),
		).
		Watches(
			&source.Kind{Type: &v1.Deployment{}},
			handler.EnqueueRequestsFromMapFunc(policyrecoEnqueueFunc),
			builder.WithPredicates(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{})),
		).
		Watches(
			&source.Kind{Type: &argov1alpha1.Rollout{}},
			handler.EnqueueRequestsFromMapFunc(policyrecoEnqueueFunc),
			builder.WithPredicates(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{})),
		).
		WithEventFilter(namespaceFilter).
		Complete(r)
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

func (r *HPAEnforcementController) deleteControllerManagedScaledObject(ctx context.Context, policyreco v1alpha1.PolicyRecommendation, workload client.Object, logger logr.Logger) error {
	scaledObjects := &kedaapi.ScaledObjectList{}
	labelSelector, err := labels.Parse(fmt.Sprintf("%s=%s", createdByLabelKey, createdByLabelValue))
	if err != nil {
		logger.V(0).Error(err, "Unable to parse label selector string.")
		return err
	}
	// List only scaledObjects created by this controller
	if err := r.List(ctx, scaledObjects, &client.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(scaledObjectField, policyreco.GetName()),
		LabelSelector: labelSelector,
		Namespace:     policyreco.GetNamespace(),
	}); err != nil && client.IgnoreNotFound(err) != nil {
		return err
	}

	var maxPods int32
	for _, scaledObject := range scaledObjects.Items {
		maxPods = *scaledObject.Spec.MaxReplicaCount
		r.Delete(ctx, &scaledObject)
		logger.V(0).Info("Deleted ScaledObject for the policyreco.", "policyreco.name", policyreco.GetName(), "policyreco.namespace", policyreco.GetNamespace(), "scaledobject.name", scaledObject.Name, "scaledobject.namespace", scaledObject.Namespace)
	}

	var workloadPatch client.Object
	if workload.GetObjectKind().GroupVersionKind().Kind == "Rollout" {
		workloadPatch = &argov1alpha1.Rollout{
			ObjectMeta: metav1.ObjectMeta{
				Name:      workload.GetName(),
				Namespace: workload.GetNamespace(),
			},
			Spec: argov1alpha1.RolloutSpec{Replicas: &maxPods},
		}
	} else if workload.GetObjectKind().GroupVersionKind().Kind == "Deployment" {
		workloadPatch = &v1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      workload.GetName(),
				Namespace: workload.GetNamespace(),
			},
			Spec: v1.DeploymentSpec{Replicas: &maxPods},
		}
	} else {
		logger.Error(err, "Unrecognized workload type")
		return nil
	}
	
	if err := r.Patch(ctx, workloadPatch, client.Apply, client.ForceOwnership, client.FieldOwner(HPAEnforcementCtrlName)); err != nil {
		logger.Error(err, "Error patching the workload")
		return client.IgnoreNotFound(err)
	}

	return nil
}
