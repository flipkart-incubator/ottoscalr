package trigger

import (
	"context"
	ottoscaleriov1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const TRIGGER_HANDLER_K8S = "K8sTriggerHandler"

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
		policyRecommendation := &ottoscaleriov1alpha1.PolicyRecommendation{
			TypeMeta: metav1.TypeMeta{
				Kind:       "PolicyRecommendation",
				APIVersion: "ottoscaler.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      workload.Name,
				Namespace: workload.Namespace,
			},
			Spec: ottoscaleriov1alpha1.PolicyRecommendationSpec{
				QueuedForExecution:   &TRUE,
				QueuedForExecutionAt: &now,
			},
		}

		err := h.k8sClient.Patch(context.TODO(), policyRecommendation, client.Apply, client.ForceOwnership, client.FieldOwner(TRIGGER_HANDLER_K8S))
		if err != nil {
			h.logger.Error(err, "Error while getting policyRecommendation.", "workload", workload)
			continue
		}

	}
}
