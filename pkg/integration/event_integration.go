package integration

import (
	"encoding/json"
	"fmt"
	"github.com/hashicorp/go-retryablehttp"
	"io/ioutil"
	"net/http"
	"time"
)

type AllEvents struct {
	HasMore bool            `json:"hasMore,omitempty"`
	Content []EventMetadata `json:"content,omitempty"`
}

type EventMetadata struct {
	EventId          string           `json:"eventId,omitempty"`
	UserId           string           `json:"userId,omitempty"`
	SegmentInfoList  []SegmentInfoDTO `json:"segmentInfoList,omitempty"`
	EventName        string           `json:"eventName,omitempty"`
	Version          int64            `json:"version,omitempty"`
	EventTier        string           `json:"eventTier,omitempty"`
	Type             string           `json:"type,omitempty"`
	EventFroze       bool             `json:"eventFroze,omitempty"`
	EarlyAccess      EarlyAccessDTO   `json:"earlyAccess,omitempty"`
	Prebook          PrebookDTO       `json:"prebook,omitempty"`
	Lifecycle        LifecycleDTO     `json:"lifecycle,omitempty"`
	EaSlots          []LifecycleDTO   `json:"eaSlots,omitempty"`
	EventStatus      string           `json:"eventStatus,omitempty"`
	ApplicableFees   []FeeDetailDTO   `json:"applicableFees,omitempty"`
	DiscountingDepth string           `json:"discountingDepth,omitempty"`
}

type EarlyAccessDTO struct {
	PointTimeMarkers []PointTimeMarkerDTO `json:"pointTimeMarkers,omitempty"`
	DseId            string               `json:"dseId,omitempty"`
	Description      string               `json:"description,omitempty"`
	StartTime        int64                `json:"startTime,omitempty"`
	EndTime          int64                `json:"endTime,omitempty"`
	Status           string               `json:"status,omitempty"`
}

type PointTimeMarkerDTO struct {
	Id          string          `json:"id,omitempty"`
	EaSlots     []LifecycleDTO  `json:"eaSlots,omitempty"`
	TimeAnchors []TimeAnchorDTO `json:"timeAnchors,omitempty"`
}

type TimeAnchorDTO struct {
	Id        string `json:"id,omitempty"`
	StartTime int64  `json:"startTime,omitempty"`
}

type SegmentInfoDTO struct {
	MarketPlace   string `json:"marketPlace,omitempty"`
	BusinessUnit  string `json:"businessUnit,omitempty"`
	SuperCategory string `json:"superCategory,omitempty"`
}

type LifecycleDTO struct {
	StartTime int64 `json:"startTime,omitempty"`
	EndTime   int64 `json:"endTime,omitempty"`
}

type FeeDetailDTO struct {
	FeeType   string  `json:"feeType,omitempty"`
	FeeTag    string  `json:"feeTag,omitempty"`
	FeeAmount float64 `json:"feeAmount,omitempty"`
}

type PrebookDTO struct {
	PrebookId        string       `json:"prebookId,omitempty"`
	PrebookLifecycle LifecycleDTO `json:"prebookLifecycle,omitempty"`
	Name             string       `json:"name,omitempty"`
}

type EventCalendarDataFetcher struct {
	Client           *http.Client
	EventAPIEndpoint string
}

func NewEventCalendarDataFetcher(eventAPIEndpoint string) (*EventCalendarDataFetcher, error) {
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 10

	client := retryClient.StandardClient()

	return &EventCalendarDataFetcher{
		Client:           client,
		EventAPIEndpoint: eventAPIEndpoint,
	}, nil
}

func (ec *EventCalendarDataFetcher) GetDesiredEvents(startTime time.Time, endTime time.Time) ([]EventDetails, error) {
	var eventDetails []EventDetails
	fetchEventsUrl := fmt.Sprintf("http://%s/fk-event-calendar-service/v1/eventCalendar/search?startTime=%d&tier=%s&pageNo=%d&pageSize=%d", ec.EventAPIEndpoint, startTime.UnixMilli(), "ALL_MP", 0, 100)
	req, err := http.NewRequest(http.MethodGet, fetchEventsUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request for fetching past events list: %v", err)
	}
	req.Header.Add("X-Flipkart-Client", "sparrow")
	req.Header.Add("X-Request-Id", "123")
	req.Header.Add("accept", "application/json")
	resp, err := ec.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error calling event api: %v", err)
	}
	var allEvents AllEvents
	responseReader, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body from event api: %v", err)
	}
	err = json.Unmarshal(responseReader, &allEvents)
	if err != nil {
		return nil, fmt.Errorf("error while unmarshaling event api json response: %v", err)
	}
	for _, events := range allEvents.Content {
		start := time.Unix(0, events.Lifecycle.StartTime*int64(time.Millisecond))
		end := time.Unix(0, events.Lifecycle.EndTime*int64(time.Millisecond))
		start = handleEaSlots(events, start)
		eventDetail := EventDetails{
			EventName: events.EventName,
			EventId:   events.EventId,
			StartTime: start,
			EndTime:   end,
		}
		eventDetails = append(eventDetails, eventDetail)
	}
	return eventDetails, nil
}

// Handling Early Access Slots
func handleEaSlots(eventMetaData EventMetadata, start time.Time) time.Time {
	updatedStart := start
	for _, ea := range eventMetaData.EaSlots {
		eaStartTimeUnix := ea.StartTime
		eaTm := time.Unix(0, eaStartTimeUnix*int64(time.Millisecond))

		if eaTm.Before(start) {
			updatedStart = eaTm
		}
	}
	return updatedStart
}
