package trigger

import (
	"context"
	ottoscaleriov1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Handler interface {
	queuePolicyRecommendations()
	QueueForExecution(workloadName string)
}
type K8sTriggerHandler struct {
	k8sClient            client.Client
	queuedForExecutionCh chan types.NamespacedName
	log                  logr.Logger
}

func NewK8sTriggerHandler(k8sClient client.Client, log logr.Logger) *K8sTriggerHandler {
	return &K8sTriggerHandler{
		k8sClient:            k8sClient,
		queuedForExecutionCh: make(chan types.NamespacedName),
		log:                  log,
	}
}

func (h *K8sTriggerHandler) Start() {
	go h.queuePolicyRecommendations()
}

func (h *K8sTriggerHandler) QueueForExecution(workload types.NamespacedName) {
	h.queuedForExecutionCh <- workload
}

func (h *K8sTriggerHandler) queuePolicyRecommendations() {
	for workload := range h.queuedForExecutionCh {
		policyRecommendation := &ottoscaleriov1alpha1.PolicyRecommendation{}
		err := h.k8sClient.Get(context.Background(), types.NamespacedName{Name: workload.Name,
			Namespace: workload.Namespace}, policyRecommendation)
		if err != nil {
			h.log.Error(err, "Error while getting policyRecommendation.", "workload", workload)
			continue
		}

		policyRecommendation.Spec.QueuedForExecution = true
		err = h.k8sClient.Update(context.Background(), policyRecommendation)
		if err != nil {
			h.log.Error(err, "Error while queueing policyRecommendation.", "workload", workload)
			continue
		}
	}
}
