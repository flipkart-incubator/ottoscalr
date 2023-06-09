package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/hashicorp/go-retryablehttp"
	"io/ioutil"
	"net/http"
	"sync"
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
	Client             *http.Client
	EventAPIEndpoint   string
	EventCache         []EventDetails
	EventFetchDuration time.Duration
	logger             logr.Logger
	lock               sync.RWMutex
	ctx                context.Context
	Cancel             context.CancelFunc
}

func NewEventCalendarDataFetcher(eventAPIEndpoint string, eventFetchDuration time.Duration, logger logr.Logger) (*EventCalendarDataFetcher, error) {
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 10

	client := retryClient.StandardClient()
	ctx, cancel := context.WithCancel(context.Background())

	ec := &EventCalendarDataFetcher{
		Client:             client,
		EventAPIEndpoint:   eventAPIEndpoint,
		EventCache:         nil,
		EventFetchDuration: eventFetchDuration,
		logger:             logger,
		lock:               sync.RWMutex{},
		ctx:                ctx,
		Cancel:             cancel,
	}
	start := time.Now().Add(-60 * 24 * time.Hour)
	end := time.Now()

	err := ec.populateEventCache(start, end)

	if err != nil {
		logger.Error(err, "Error in fetching data from event calendar")
	}
	go ec.Start()
	return ec, nil
}

func (ec *EventCalendarDataFetcher) Start() {
	ticker := time.NewTicker(ec.EventFetchDuration)
	defer ticker.Stop()

	for {
		select {
		case <-ec.ctx.Done():
			return
		case <-ticker.C:
			start := time.Now().Add(-60 * 24 * time.Hour)
			end := time.Now()
			err := ec.populateEventCache(start, end)

			if err != nil {
				ec.logger.Error(err, "Error in fetching data from event calendar", "eventCache", ec.EventCache)
			}
		}
	}
}

func (ec *EventCalendarDataFetcher) GetDesiredEvents(startTime time.Time, endTime time.Time) ([]EventDetails, error) {
	ec.lock.RLock()
	defer ec.lock.RUnlock()
	return ec.EventCache, nil
}

func (ec *EventCalendarDataFetcher) populateEventCache(startTime time.Time, endTime time.Time) error {
	var eventDetails []EventDetails
	fetchEventsUrl := fmt.Sprintf("%s?startTime=%d&tier=%s&pageNo=%d&pageSize=%d", ec.EventAPIEndpoint, startTime.UnixMilli(), "ALL_MP", 0, 100)
	req, err := http.NewRequest(http.MethodGet, fetchEventsUrl, nil)
	if err != nil {
		return fmt.Errorf("error creating request for fetching past events list: %v", err)
	}
	req.Header.Add("X-Flipkart-Client", "sparrow")
	req.Header.Add("X-Request-Id", "123")
	req.Header.Add("accept", "application/json")
	resp, err := ec.Client.Do(req)
	if err != nil {
		return fmt.Errorf("error calling event api: %v", err)
	}
	var allEvents AllEvents
	responseReader, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body from event api: %v", err)
	}
	err = json.Unmarshal(responseReader, &allEvents)
	if err != nil {
		return fmt.Errorf("error while unmarshaling event api json response: %v", err)
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
	ec.lock.Lock()
	defer ec.lock.Unlock()
	ec.EventCache = eventDetails
	return nil
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
