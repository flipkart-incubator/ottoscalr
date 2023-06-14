package integration

import (
	"context"
	"encoding/json"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type NFREventMetadata struct {
	EventKey  string `json:"nfrEventKey"`
	StartTime string `json:"startDateTime"`
	EndTime   string `json:"endDateTime"`
	Notes     string `json:"notes"`
}

type NFREventDataFetcher struct {
	k8sClient     client.Client
	namespace     string
	configmapName string
}

func NewNFREventDataFetcher(k8sClient client.Client, namespace string, configmapName string) (*NFREventDataFetcher, error) {

	return &NFREventDataFetcher{
		k8sClient:     k8sClient,
		namespace:     namespace,
		configmapName: configmapName,
	}, nil
}

func (ne *NFREventDataFetcher) GetDesiredEvents(startTime time.Time, endTime time.Time) ([]EventDetails, error) {
	nfrDataConfigmap := corev1.ConfigMap{}
	if err := ne.k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: ne.namespace,
		Name:      ne.configmapName,
	}, &nfrDataConfigmap); err != nil {
		return nil, fmt.Errorf("error while getting nfr data configmap: %v", err)
	}
	var eventDetails []EventDetails
	for _, value := range nfrDataConfigmap.Data {
		var nfrEventDetail NFREventMetadata
		err := json.Unmarshal([]byte(value), &nfrEventDetail)
		if err != nil {
			fmt.Printf("Err in umarshaling: %v\n", err)
		}
		eventDetail := EventDetails{
			EventId:   nfrEventDetail.EventKey,
			EventName: "nfr",
			StartTime: formatTime(nfrEventDetail.StartTime),
			EndTime:   formatTime(nfrEventDetail.EndTime),
		}
		if err != nil {
			return nil, fmt.Errorf("error while unmarshaling nfr data from configmap: %v", err)
		}
		eventDetails = append(eventDetails, eventDetail)

	}
	return eventDetails, nil
}

func formatTime(tm string) time.Time {

	parse, _ := time.Parse("2006-01-02T15:04:05", tm)

	unixTime := parse.Unix()
	actualTm := time.Unix(0, unixTime*int64(time.Second))
	formattedTime := actualTm.Add(-330 * time.Minute)
	return formattedTime
}
