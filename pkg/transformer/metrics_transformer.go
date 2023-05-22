package transformer

import (
	"fmt"
	"github.com/flipkart-incubator/ottoscalr/pkg/integration"
	"github.com/flipkart-incubator/ottoscalr/pkg/metrics"
	"time"
)

type OutlierInterval struct {
	StartTime time.Time
	EndTime   time.Time
}

type OutlierInterpolatorTransformer struct {
	EventIntegration integration.EventIntegration
}

func NewOutlierInterpolatorTransformer(eventIntegration integration.EventIntegration) (*OutlierInterpolatorTransformer, error) {

	return &OutlierInterpolatorTransformer{
		EventIntegration: eventIntegration,
	}, nil
}

func (ot *OutlierInterpolatorTransformer) Transform(startTime time.Time, endTime time.Time, dataPoints []metrics.DataPoint) ([]metrics.DataPoint, error) {
	eventDetails, err := ot.EventIntegration.GetDesiredEvents(startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("error in getting events from event calendar: %v", err)
	}
	intervals := getOutlierIntervals(eventDetails)
	newDataPoints := ot.cleanOutliersAndInterpolate(dataPoints, intervals)
	return newDataPoints, nil
}

func getOutlierIntervals(eventDetails []integration.EventDetails) []OutlierInterval {
	var intervals []OutlierInterval
	for _, event := range eventDetails {
		interval := OutlierInterval{
			StartTime: event.StartTime,
			EndTime:   event.EndTime,
		}
		intervals = append(intervals, interval)
	}
	return intervals
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
func (ot *OutlierInterpolatorTransformer) cleanOutliersAndInterpolate(dataPoints []metrics.DataPoint, intervals []OutlierInterval) []metrics.DataPoint {
	newDataPoints := dataPoints
	filteredIntervals := filterIntervals(intervals)
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
