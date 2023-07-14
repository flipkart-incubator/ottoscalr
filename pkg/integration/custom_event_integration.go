package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type CustomEventMetadata struct {
	EventId   string `json:"eventId"`
	EventName string `json:"eventName"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

type CustomEventDataFetcher struct {
	k8sClient     client.Client
	namespace     string
	configmapName string
	logger        logr.Logger
}

func NewCustomEventDataFetcher(k8sClient client.Client, namespace string, configmapName string, logger logr.Logger) (*CustomEventDataFetcher, error) {

	return &CustomEventDataFetcher{
		k8sClient:     k8sClient,
		namespace:     namespace,
		configmapName: configmapName,
		logger:        logger,
	}, nil
}

func (ce *CustomEventDataFetcher) GetDesiredEvents(startTime time.Time, endTime time.Time) ([]EventDetails, error) {
	customDataConfigmap := corev1.ConfigMap{}
	if err := ce.k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: ce.namespace,
		Name:      ce.configmapName,
	}, &customDataConfigmap); err != nil {
		return nil, fmt.Errorf("error while getting custom event data configmap: %v", err)
	}
	var eventDetails []EventDetails
	for _, value := range customDataConfigmap.Data {
		var customEventDetail CustomEventMetadata
		err := json.Unmarshal([]byte(value), &customEventDetail)
		if err != nil {
			return nil, fmt.Errorf("error while unmarshaling custom event data from configmap: %v", err)
		}
		eventDetail := EventDetails{
			EventId:   customEventDetail.EventId,
			EventName: customEventDetail.EventName,
			StartTime: formatTime(customEventDetail.StartTime),
			EndTime:   formatTime(customEventDetail.EndTime),
		}
		eventDetails = append(eventDetails, eventDetail)
		ce.logger.Info("List of Fetched custom events", "events", eventDetails)
	}
	return eventDetails, nil
}
