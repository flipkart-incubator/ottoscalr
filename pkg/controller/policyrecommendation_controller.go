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
	"github.com/flipkart-incubator/ottoscalr/pkg/reco"
	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"strconv"
	"time"

	v1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	PolicyRecoWorkflowCtrlName = "RecoWorkflowController"
	RecoQueuedStatusManager    = "RecoQueuedStatusManager"
	eventTypeNormal            = "Normal"
	eventTypeWarning           = "Warning"
)

var (
	reconcileCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{Name: "policyreco_reconciled_count",
			Help: "Number of policyrecos reconciled counter"}, []string{"namespace", "policyreco"},
	)
	reconcileErroredCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{Name: "policyreco_reconciler_errored_count",
			Help: "Number of policyrecos reconcile errored counter"}, []string{"namespace", "policyreco"},
	)
	targetRecoSLI = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "policyreco_reconciler_targetreco_slo_days",
			Help:    "Time taken for a policy reco to achieve the target reco in days",
			Buckets: []float64{1, 2, 3, 5, 7, 10, 15, 20, 25, 28},
		}, []string{"namespace", "policyreco"},
	)
	policyRecoConditionsGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{Name: "policyreco_reconciler_conditions",
			Help: "PolicyReco conditions"}, []string{"namespace", "policyreco", "type", "status"},
	)
	policyRecoTaskProgressReasonsGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{Name: "policyreco_reconciler_task_progress_reason",
			Help: "PolicyReco RecoTaskProgress Reason"}, []string{"namespace", "policyreco", "type", "reason"})

	policyRecoTargetMin = promauto.NewGaugeVec(
		prometheus.GaugeOpts{Name: "policyreco_target_policy_min",
			Help: "PolicyReco Target Policy Min"}, []string{"namespace", "policyreco"})

	policyRecoTargetMax = promauto.NewGaugeVec(
		prometheus.GaugeOpts{Name: "policyreco_target_policy_max",
			Help: "PolicyReco Target Policy Max"}, []string{"namespace", "policyreco"})

	policyRecoTargetUtil = promauto.NewGaugeVec(
		prometheus.GaugeOpts{Name: "policyreco_target_policy_utilization",
			Help: "PolicyReco Target Policy Utilization"}, []string{"namespace", "policyreco"})

	policyRecoCurrentMin = promauto.NewGaugeVec(
		prometheus.GaugeOpts{Name: "policyreco_current_policy_min",
			Help: "PolicyReco Current Policy Min"}, []string{"namespace", "policyreco"})

	policyRecoCurrentMax = promauto.NewGaugeVec(
		prometheus.GaugeOpts{Name: "policyreco_current_policy_max",
			Help: "PolicyReco Current Policy Max"}, []string{"namespace", "policyreco"})

	policyRecoCurrentUtil = promauto.NewGaugeVec(
		prometheus.GaugeOpts{Name: "policyreco_current_policy_utilization",
			Help: "PolicyReco Current Policy Utilization"}, []string{"namespace", "policyreco"})
)

func init() {
	metrics.Registry.MustRegister(reconcileCounter, reconcileErroredCounter, targetRecoSLI,
		policyRecoConditionsGauge, policyRecoTaskProgressReasonsGauge, policyRecoTargetMin, policyRecoTargetMax, policyRecoTargetUtil,
		policyRecoCurrentMin, policyRecoCurrentMax, policyRecoCurrentUtil)
}

// PolicyRecommendationReconciler reconciles a PolicyRecommendation object
type PolicyRecommendationReconciler struct {
	client.Client
	Scheme                  *runtime.Scheme
	Recorder                record.EventRecorder
	MaxConcurrentReconciles int
	PolicyExpiryAge         time.Duration
	RecoWorkflow            reco.RecommendationWorkflow
}

func NewPolicyRecommendationReconciler(client client.Client,
	scheme *runtime.Scheme, recorder record.EventRecorder,
	maxConcurrentReconciles int, minRequiredReplicas int, recommender reco.Recommender, policyIterators ...reco.PolicyIterator) (*PolicyRecommendationReconciler, error) {
	recoWfBuilder := reco.NewRecommendationWorkflowBuilder().
		WithRecommender(recommender).WithMinRequiredReplicas(minRequiredReplicas)
	for _, pi := range policyIterators {
		recoWfBuilder = recoWfBuilder.WithPolicyIterator(pi)
	}
	recoWorkflow, err := recoWfBuilder.Build()
	if err != nil {
		return nil, err
	}
	return &PolicyRecommendationReconciler{
		Client:                  client,
		Scheme:                  scheme,
		MaxConcurrentReconciles: maxConcurrentReconciles,
		Recorder:                recorder,
		RecoWorkflow:            recoWorkflow,
	}, nil
}

//+kubebuilder:rbac:groups=ottoscaler.io,resources=policyrecommendations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ottoscaler.io,resources=policyrecommendations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ottoscaler.io,resources=policyrecommendations/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch;delete

func (r *PolicyRecommendationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	logger := ctrl.LoggerFrom(ctx).WithName(PolicyRecoWorkflowCtrlName)

	// Keeping this here to consider the generatedAt timestamp to be the beginning of the reconcile op
	generatedAt := metav1.Now()

	policyreco := v1alpha1.PolicyRecommendation{}
	if err := r.Get(ctx, req.NamespacedName, &policyreco); err != nil {
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.V(2).Info("PolicyRecomemndation retrieved", "policyreco", policyreco)

	r.Recorder.Event(&policyreco, eventTypeNormal, "HPARecoQueuedForExecution", "This workload has been queued for a fresh HPA recommendation.")

	policyRecoWorkloadGauge.WithLabelValues(policyreco.Namespace, policyreco.Name, policyreco.Spec.WorkloadMeta.TypeMeta.Kind, policyreco.Spec.WorkloadMeta.Name).Set(1)

	var conditions []metav1.Condition

	statusPatch, conditions := CreatePolicyPatch(policyreco, conditions, v1alpha1.RecoTaskProgress, metav1.ConditionTrue, RecoTaskInProgress, RecoTaskInProgressMessage)
	if err := r.Status().Patch(ctx, statusPatch, client.Apply, getSubresourcePatchOptions(PolicyRecoWorkflowCtrlName)); err != nil {
		logger.Error(err, "Error updating the status of the policy reco object")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	logPolicyRecoGaugeMetric(policyreco, v1alpha1.RecoTaskProgress, metav1.ConditionTrue)

	hpaConfigToBeApplied, targetHPAReco, policy, err := r.RecoWorkflow.Execute(ctx, reco.WorkloadMeta{
		TypeMeta:  policyreco.Spec.WorkloadMeta.TypeMeta,
		Name:      policyreco.Spec.WorkloadMeta.Name,
		Namespace: policyreco.Namespace,
	})
	if err != nil {
		statusPatch, conditions = CreatePolicyPatch(policyreco, conditions, v1alpha1.RecoTaskProgress, metav1.ConditionFalse, RecoTaskErrored, err.Error())
		if err := r.Status().Patch(ctx, statusPatch, client.Apply, getSubresourcePatchOptions(PolicyRecoWorkflowCtrlName)); err != nil {
			logger.Error(err, "Error updating the status of the policy reco object")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		logPolicyRecoGaugeMetric(policyreco, v1alpha1.RecoTaskProgress, metav1.ConditionFalse)
		logRecoTaskProgressReasonGaugeMetric(policyreco, v1alpha1.RecoTaskProgress, RecoTaskErrored)
		reconcileErroredCounter.WithLabelValues(policyreco.Namespace, policyreco.Name).Inc()
		return ctrl.Result{}, err
	}

	if targetHPAReco == nil {
		statusPatch, conditions = CreatePolicyPatch(policyreco, conditions, v1alpha1.RecoTaskProgress, metav1.ConditionFalse, RecoTaskErrored, EmptyRecoConfigMessage)
		if err := r.Status().Patch(ctx, statusPatch, client.Apply, getSubresourcePatchOptions(PolicyRecoWorkflowCtrlName)); err != nil {
			logger.Error(err, "Error updating the status of the policy reco object")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		logPolicyRecoGaugeMetric(policyreco, v1alpha1.RecoTaskProgress, metav1.ConditionFalse)
		logRecoTaskProgressReasonGaugeMetric(policyreco, v1alpha1.RecoTaskProgress, RecoTaskErrored)
		logger.V(0).Error(nil, "Recommended config is empty. Requeuing")
		reconcileErroredCounter.WithLabelValues(policyreco.Namespace, policyreco.Name).Inc()
		return ctrl.Result{
			RequeueAfter: 5 * time.Second,
		}, nil
	}

	if hpaConfigToBeApplied == nil {
		statusPatch, conditions = CreatePolicyPatch(policyreco, conditions, v1alpha1.RecoTaskProgress, metav1.ConditionFalse, RecoTaskErrored, EmptyHPAConfigMessage)
		if err := r.Status().Patch(ctx, statusPatch, client.Apply, getSubresourcePatchOptions(PolicyRecoWorkflowCtrlName)); err != nil {
			logger.Error(err, "Error updating the status of the policy reco object")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		logPolicyRecoGaugeMetric(policyreco, v1alpha1.RecoTaskProgress, metav1.ConditionFalse)
		logRecoTaskProgressReasonGaugeMetric(policyreco, v1alpha1.RecoTaskProgress, RecoTaskErrored)
		logger.V(0).Error(nil, "HPA config to be applied is empty. Requeuing")
		reconcileErroredCounter.WithLabelValues(policyreco.Namespace, policyreco.Name).Inc()
		return ctrl.Result{
			RequeueAfter: 5 * time.Second,
		}, nil
	}

	var policyName string

	if policy != nil {
		policyName = policy.Name
	} else {
		policyName = policyreco.Spec.Policy
	}

	transitionedAt := retrieveTransitionTime(hpaConfigToBeApplied, &policyreco, generatedAt)
	policyRecoPatch := &v1alpha1.PolicyRecommendation{
		TypeMeta: policyreco.TypeMeta,
		ObjectMeta: metav1.ObjectMeta{
			Name:      policyreco.Name,
			Namespace: policyreco.Namespace,
		},
		Spec: v1alpha1.PolicyRecommendationSpec{
			QueuedForExecution:      &falseBool,
			TargetHPAConfiguration:  *targetHPAReco,
			Policy:                  policyName,
			CurrentHPAConfiguration: *hpaConfigToBeApplied,
			TransitionedAt:          &transitionedAt,
			GeneratedAt:             &generatedAt,
		},
	}
	logger.V(0).Info("Policy Patch", "PolicyReco", *policyRecoPatch)
	if err := r.Patch(ctx, policyRecoPatch, client.Apply, client.ForceOwnership, client.FieldOwner(PolicyRecoWorkflowCtrlName)); err != nil {
		logger.Error(err, "Error patching the policy reco object")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	logger.V(1).Info("Policy Patch", "PolicyReco", *policyRecoPatch)

	logTargetHPAConfiguration(policyreco, targetHPAReco)
	logCurrentHPAConfiguration(policyreco, hpaConfigToBeApplied)

	initializedTime := fetchInitializedTime(&policyreco)
	targetAchievedAlready := fetchTargetAchieved(&policyreco)
	if policyRecoPatch.Spec.TargetHPAConfiguration.DeepEquals(policyRecoPatch.Spec.CurrentHPAConfiguration) {
		if !targetAchievedAlready {
			targetRecoSLI.WithLabelValues(policyreco.Namespace, policyreco.Name).Observe(time.Since(initializedTime).Hours() / 24)
		}
		statusPatch, conditions = CreatePolicyPatch(policyreco, conditions, v1alpha1.TargetRecoAchieved, metav1.ConditionTrue, PolicyRecommendationAtTargetReco, TargetRecoAchievedSuccessMessage)
		logPolicyRecoGaugeMetric(policyreco, v1alpha1.TargetRecoAchieved, metav1.ConditionTrue)
	} else {
		targetRecoSLI.DeleteLabelValues(policyreco.Namespace, policyreco.Name)
		statusPatch, conditions = CreatePolicyPatch(policyreco, conditions, v1alpha1.TargetRecoAchieved, metav1.ConditionFalse, PolicyRecommendationNotAtTargetReco, TargetRecoAchievedFailureMessage)
		logPolicyRecoGaugeMetric(policyreco, v1alpha1.TargetRecoAchieved, metav1.ConditionFalse)
	}

	statusPatch, conditions = CreatePolicyPatch(policyreco, conditions, v1alpha1.RecoTaskProgress, metav1.ConditionFalse, RecoTaskRecommendationGenerated, RecommendationGeneratedMessage)
	if err := r.Status().Patch(ctx, statusPatch, client.Apply, getSubresourcePatchOptions(PolicyRecoWorkflowCtrlName)); err != nil {
		logger.Error(err, "Error updating the of status the policy reco object")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	logPolicyRecoGaugeMetric(policyreco, v1alpha1.RecoTaskProgress, metav1.ConditionFalse)
	logRecoTaskProgressReasonGaugeMetric(policyreco, v1alpha1.RecoTaskProgress, RecoTaskRecommendationGenerated)

	statusPatch, conditions = CreatePolicyPatch(policyreco, conditions, v1alpha1.RecoTaskQueued, metav1.ConditionFalse, RecoTaskExecutionDone, RecoTaskExecutionDoneMessage)
	if err := r.Status().Patch(ctx, statusPatch, client.Apply, getSubresourcePatchOptions(RecoQueuedStatusManager)); err != nil {
		logger.Error(err, "Error updating the status of the policy reco object")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.V(1).Info("Recommendation generated Policy Patch Applied", "PolicyReco", *statusPatch)

	reconcileCounter.WithLabelValues(policyreco.Namespace, policyreco.Name).Inc()
	r.Recorder.Event(&policyreco, eventTypeNormal, "HPARecommendationGenerated", fmt.Sprintf("The HPA recommendation has been generated successfully. The current policy this workload is at %s.", policyName))
	logger.V(1).Info("Successfully generated HPA Recommendation.")
	return ctrl.Result{}, nil
}

func logCurrentHPAConfiguration(policyreco v1alpha1.PolicyRecommendation, currentHPAReco *v1alpha1.HPAConfiguration) {
	policyRecoCurrentMin.WithLabelValues(policyreco.Namespace, policyreco.Name).Set(float64(currentHPAReco.Min))
	policyRecoCurrentMax.WithLabelValues(policyreco.Namespace, policyreco.Name).Set(float64(currentHPAReco.Max))
	policyRecoCurrentUtil.WithLabelValues(policyreco.Namespace, policyreco.Name).Set(float64(currentHPAReco.TargetMetricValue))
}

func logTargetHPAConfiguration(policyreco v1alpha1.PolicyRecommendation, targetHPAReco *v1alpha1.HPAConfiguration) {
	policyRecoTargetMin.WithLabelValues(policyreco.Namespace, policyreco.Name).Set(float64(targetHPAReco.Min))
	policyRecoTargetMax.WithLabelValues(policyreco.Namespace, policyreco.Name).Set(float64(targetHPAReco.Max))
	policyRecoTargetUtil.WithLabelValues(policyreco.Namespace, policyreco.Name).Set(float64(targetHPAReco.TargetMetricValue))
}

func logPolicyRecoGaugeMetric(policyreco v1alpha1.PolicyRecommendation, condition v1alpha1.PolicyRecommendationConditionType, status metav1.ConditionStatus) {
	if status == metav1.ConditionTrue {
		policyRecoConditionsGauge.WithLabelValues(policyreco.Namespace, policyreco.Name, string(condition), string(metav1.ConditionTrue)).Set(1)
		policyRecoConditionsGauge.WithLabelValues(policyreco.Namespace, policyreco.Name, string(condition), string(metav1.ConditionFalse)).Set(0)
	} else {
		policyRecoConditionsGauge.WithLabelValues(policyreco.Namespace, policyreco.Name, string(condition), string(metav1.ConditionTrue)).Set(0)
		policyRecoConditionsGauge.WithLabelValues(policyreco.Namespace, policyreco.Name, string(condition), string(metav1.ConditionFalse)).Set(1)
	}
}

func logRecoTaskProgressReasonGaugeMetric(policyreco v1alpha1.PolicyRecommendation, condition v1alpha1.PolicyRecommendationConditionType, reason string) {
	if reason == RecoTaskRecommendationGenerated {
		policyRecoTaskProgressReasonsGauge.WithLabelValues(policyreco.Namespace, policyreco.Name, string(condition), RecoTaskRecommendationGenerated).Set(1)
		policyRecoTaskProgressReasonsGauge.WithLabelValues(policyreco.Namespace, policyreco.Name, string(condition), RecoTaskErrored).Set(0)
	} else if reason == RecoTaskErrored {
		policyRecoTaskProgressReasonsGauge.WithLabelValues(policyreco.Namespace, policyreco.Name, string(condition), RecoTaskRecommendationGenerated).Set(0)
		policyRecoTaskProgressReasonsGauge.WithLabelValues(policyreco.Namespace, policyreco.Name, string(condition), RecoTaskErrored).Set(1)
	}
}

func fetchTargetAchieved(policyreco *v1alpha1.PolicyRecommendation) bool {
	if policyreco == nil {
		return false
	}
	targetAchieved := false
	for _, condition := range policyreco.Status.Conditions {
		if condition.Type == string(v1alpha1.TargetRecoAchieved) {
			targetAchieved, _ = strconv.ParseBool(string(condition.Status))
			return targetAchieved
		}
	}
	return targetAchieved
}

func fetchInitializedTime(policyreco *v1alpha1.PolicyRecommendation) time.Time {
	if policyreco == nil {
		return time.Now()
	}
	for _, condition := range policyreco.Status.Conditions {
		if condition.Type == string(v1alpha1.Initialized) {
			return condition.LastTransitionTime.Time
		}
	}
	return time.Now()
}

func retrieveTransitionTime(hpaConfigToBeApplied *v1alpha1.HPAConfiguration, policyreco *v1alpha1.PolicyRecommendation, generatedAt metav1.Time) metav1.Time {
	if hpaConfigToBeApplied == nil && policyreco != nil {
		return *policyreco.Spec.TransitionedAt
	} else if hpaConfigToBeApplied == nil || policyreco == nil {
		return generatedAt
	}
	if !hpaConfigToBeApplied.DeepEquals(policyreco.Spec.CurrentHPAConfiguration) {
		return generatedAt
	}
	return *policyreco.Spec.TransitionedAt
}

func getSubresourcePatchOptions(fieldOwner string) *client.SubResourcePatchOptions {
	patchOpts := client.PatchOptions{}
	client.ForceOwnership.ApplyToPatch(&patchOpts)
	client.FieldOwner(fieldOwner).ApplyToPatch(&patchOpts)
	return &client.SubResourcePatchOptions{
		PatchOptions: patchOpts,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *PolicyRecommendationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// predicates to filter events with updates to QueuedForExecution or QueuedForExecutionAt
	queuedTaskPredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			objSpec := e.Object.(*v1alpha1.PolicyRecommendation).Spec
			switch {
			case *objSpec.QueuedForExecution == true:
				return true
			default:
				return false
			}
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldObjSpec := e.ObjectOld.(*v1alpha1.PolicyRecommendation).Spec
			newObjSpec := e.ObjectNew.(*v1alpha1.PolicyRecommendation).Spec
			if newObjSpec.QueuedForExecutionAt.IsZero() {
				return false
			}
			switch {
			// updates which transition QueuedForExecution from false to true
			case *oldObjSpec.QueuedForExecution == false && *newObjSpec.QueuedForExecution == true:
				return true
			//	updates where there's no change to the QueuedForExecution but to the QueuedForExecutionAt with a later timestamp
			case (*newObjSpec.QueuedForExecution == true && oldObjSpec.QueuedForExecutionAt.Before(newObjSpec.QueuedForExecutionAt)) || newObjSpec.QueuedForExecutionAt.After(newObjSpec.GeneratedAt.Time):
				return true
			default:
				return false
			}
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
	compoundPredicate := predicate.And(predicate.GenerationChangedPredicate{}, queuedTaskPredicate)
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.PolicyRecommendation{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: r.MaxConcurrentReconciles}).
		WithEventFilter(compoundPredicate).
		Named(PolicyRecoWorkflowCtrlName).
		Complete(r)
}
