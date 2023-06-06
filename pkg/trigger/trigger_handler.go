package trigger

import (
	"context"
	ottoscaleriov1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	TRIGGER_HANDLER_K8S = "K8sTriggerHandler"

	RecoQueuedStatusManager = "RecoQueuedStatusManager"
	RecoTaskQueued          = "RecoTaskQueued"
	RecoTaskQueuedMessage   = "Workload Queued for Fresh HPA Recommendation"
)

type Handler interface {
	queuePolicyRecommendations()
	QueueForExecution(workloadName string)
}
type K8sTriggerHandler struct {
	k8sClient            client.Client
	queuedForExecutionCh chan types.NamespacedName
	logger               logr.Logger
}

func NewK8sTriggerHandler(k8sClient client.Client, logger logr.Logger) *K8sTriggerHandler {
	return &K8sTriggerHandler{
		k8sClient:            k8sClient,
		queuedForExecutionCh: make(chan types.NamespacedName),
		logger:               logger,
	}
}

func (h *K8sTriggerHandler) Start() {
	go h.queuePolicyRecommendations()
}

func (h *K8sTriggerHandler) QueueForExecution(recommendation types.NamespacedName) {
	h.queuedForExecutionCh <- recommendation
}

// TODO: @neerajb Handle passing error back to the controllers, so that the reconcile can be run again.
func (h *K8sTriggerHandler) QueueAllForExecution() {
	var allRecommendations ottoscaleriov1alpha1.PolicyRecommendationList
	if err := h.k8sClient.List(context.Background(), &allRecommendations); err != nil {
		h.logger.Error(err, "Error getting allRecommendations")
	}
	for _, reco := range allRecommendations.Items {
		h.logger.V(0).Info("Queuing policy recommendation for execution", "name", reco.Name, "namespace", reco.Namespace)
		h.queuedForExecutionCh <- types.NamespacedName{Name: reco.GetName(), Namespace: reco.GetNamespace()}
	}
}

func (h *K8sTriggerHandler) queuePolicyRecommendations() {
	TRUE := true
	for workload := range h.queuedForExecutionCh {
		now := metav1.Now()
		policyRecommendation := &ottoscaleriov1alpha1.PolicyRecommendation{}

		err := h.k8sClient.Get(context.TODO(), types.NamespacedName{
			Namespace: workload.Namespace,
			Name:      workload.Name,
		}, policyRecommendation)
		if err != nil {
			h.logger.Error(err, "Error while getting policyRecommendation.", "workload", workload)
			continue
		}

		policyRecommendation.Spec.QueuedForExecution = &TRUE
		policyRecommendation.Spec.QueuedForExecutionAt = &now

		err = h.k8sClient.Update(context.TODO(), policyRecommendation, client.FieldOwner(TRIGGER_HANDLER_K8S))
		if err != nil {
			h.logger.Error(err, "Error while updating policyRecommendation.", "workload", workload)
			continue
		}

		statusPatch := &ottoscaleriov1alpha1.PolicyRecommendation{
			TypeMeta: metav1.TypeMeta{
				APIVersion: ottoscaleriov1alpha1.GroupVersion.String(),
				Kind:       "PolicyRecommendation",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      policyRecommendation.Name,
				Namespace: policyRecommendation.Namespace,
			},
			Status: ottoscaleriov1alpha1.PolicyRecommendationStatus{
				Conditions: []metav1.Condition{
					{
						Type:               string(ottoscaleriov1alpha1.RecoTaskQueued),
						Status:             metav1.ConditionTrue,
						LastTransitionTime: now,
						Reason:             RecoTaskQueued,
						Message:            RecoTaskQueuedMessage,
					},
				},
			},
		}
		if err := h.k8sClient.Status().Patch(context.Background(), statusPatch, client.Apply, getSubresourcePatchOptions(RecoQueuedStatusManager)); err != nil {
			h.logger.Error(err, "Error updating the status of the policy reco object")
			continue
		}
	}
}

func getSubresourcePatchOptions(fieldOwner string) *client.SubResourcePatchOptions {
	patchOpts := client.PatchOptions{}
	client.ForceOwnership.ApplyToPatch(&patchOpts)
	client.FieldOwner(fieldOwner).ApplyToPatch(&patchOpts)
	return &client.SubResourcePatchOptions{
		PatchOptions: patchOpts,
	}
}
