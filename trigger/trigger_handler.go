package trigger

import (
	"context"
	ottoscaleriov1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Handler interface {
	handleBreaches()
	QueueForExecution(workloadName string)
}
type K8sTriggerHandler struct {
	k8sClient            client.Client
	queuedForExecutionCh chan string
	log                  logr.Logger
}

func NewK8sTriggerHandler(k8sClient client.Client, log logr.Logger) *K8sTriggerHandler {
	return &K8sTriggerHandler{
		k8sClient:            k8sClient,
		queuedForExecutionCh: make(chan string),
		log:                  log,
	}
}

func (h *K8sTriggerHandler) Start() {
	go h.handleBreaches()
}

func (h *K8sTriggerHandler) QueueForExecution(workloadName string) {
	h.queuedForExecutionCh <- workloadName
}

func (h *K8sTriggerHandler) handleBreaches() {
	for workloadName := range h.queuedForExecutionCh {
		policyRecommendation := &ottoscaleriov1alpha1.PolicyRecommendation{}
		err := h.k8sClient.Get(context.Background(), types.NamespacedName{Name: workloadName}, policyRecommendation)
		if err != nil {
			h.log.Error(err, "Error while getting policyRecommendation.", "workload", workloadName)
			continue
		}

		policyRecommendation.Spec.QueuedForExecution = true
		err = h.k8sClient.Update(context.Background(), policyRecommendation)
		if err != nil {
			h.log.Error(err, "Error while queueing policyRecommendation.", "workload", workloadName)
			continue
		}
	}
}
