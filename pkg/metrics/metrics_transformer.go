package metrics

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"encoding/json"

	"github.com/hashicorp/go-retryablehttp"
)

type OutlierInterval struct {
	StartTime time.Time
	EndTime   time.Time
}

type MetricsTransformer interface {
	GetOutlierIntervalsAndInterpolate(
		startTime time.Time, dataPoints []DataPoint) ([]DataPoint, error)
}

type ALLEvents struct {
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

type EventMetricsTransformer struct {
	Client           *http.Client
	EventAPIEndpoint string
}

func NewEventMetricsTransformer(eventAPIEndpoint string) (*EventMetricsTransformer, error) {
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 10

	client := retryClient.StandardClient()

	return &EventMetricsTransformer{
		Client:           client,
		EventAPIEndpoint: eventAPIEndpoint,
	}, nil
}

func (ec *EventMetricsTransformer) GetOutlierIntervalsAndInterpolate(startTime time.Time, dataPoints []DataPoint) ([]DataPoint, error) {
	var intervals []OutlierInterval
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
	var allEvents ALLEvents
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
		interval := OutlierInterval{StartTime: start, EndTime: end}
		intervals = append(intervals, interval)
	}
	newDataPoints := ec.cleanOutliersAndInterpolate(dataPoints, intervals)
	return newDataPoints, nil
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

// Handling Overlapping and Unnecessary Outlier Intervals
func filterIntervals(intervals []OutlierInterval) []OutlierInterval {
	var filteredInterval []OutlierInterval
	for _, interval := range intervals {
		if interval.StartTime.Before(time.Now()) {
			filteredInterval = append(filteredInterval, interval)
		}
	}
	if len(filteredInterval) == 0 {
		return nil
	}
	var nonOverLappingInterval []OutlierInterval
	curStart := filteredInterval[0].StartTime
	curEnd := filteredInterval[0].EndTime

	for i := 1; i < len(filteredInterval); i++ {
		if filteredInterval[i].StartTime.Before(curEnd) || filteredInterval[i].StartTime.Equal(curEnd) {
			if filteredInterval[i].EndTime.After(curEnd) {
				curEnd = filteredInterval[i].EndTime
			}
		} else {
			nonOverLappingInterval = append(nonOverLappingInterval, OutlierInterval{curStart, curEnd})
			curStart = filteredInterval[i].StartTime
			curEnd = filteredInterval[i].EndTime
		}
	}
	nonOverLappingInterval = append(nonOverLappingInterval, OutlierInterval{curStart, curEnd})
	return nonOverLappingInterval
}

// CleanOutliersAndInterpolate - Linear Interpolation for the dataPoints in interval range.
func (ec *EventMetricsTransformer) cleanOutliersAndInterpolate(dataPoints []DataPoint, intervals []OutlierInterval) []DataPoint {
	newDataPoints := dataPoints
	filteredIntervals := filterIntervals(intervals)
	fmt.Println(filteredIntervals)
	for _, interval := range filteredIntervals {
		startIndex := -1
		endIndex := 0
		for i := 0; i < len(newDataPoints); i++ {
			if newDataPoints[i].Timestamp.After(interval.StartTime) && newDataPoints[i].Timestamp.Before(interval.EndTime) {
				if startIndex == -1 {
					startIndex = i
				}
				newDataPoints[i].Value = 0.0
				endIndex = i
			}
		}

		actStartIndex := startIndex - 1
		actEndIndex := endIndex + 1

		if actEndIndex == len(newDataPoints) {
			actEndIndex = endIndex
		}
		if actStartIndex <= -1 {
			actStartIndex = 0
		}
		// Interpolate data
		timeDiff := newDataPoints[actEndIndex].Timestamp.Sub(newDataPoints[actStartIndex].Timestamp)
		slope := (newDataPoints[actEndIndex].Value - newDataPoints[actStartIndex].Value) / (timeDiff.Seconds())
		for j := actStartIndex; j < actEndIndex-1; j++ {
			newDataPoints[j+1].Value = newDataPoints[j].Value + slope*(newDataPoints[j+1].Timestamp.Sub(newDataPoints[j].Timestamp).Seconds())
		}
	}
	return newDataPoints
}
