package transformer

import (
	"fmt"
	"github.com/flipkart-incubator/ottoscalr/pkg/integration"
	"github.com/flipkart-incubator/ottoscalr/pkg/metrics"
	"github.com/go-logr/logr"
	"sort"
	"time"
)

type OutlierInterval struct {
	StartTime time.Time
	EndTime   time.Time
}

type OutlierInterpolatorTransformer struct {
	EventIntegration []integration.EventIntegration
	logger           logr.Logger
}

func NewOutlierInterpolatorTransformer(eventIntegration []integration.EventIntegration, logger logr.Logger) (*OutlierInterpolatorTransformer, error) {

	return &OutlierInterpolatorTransformer{
		EventIntegration: eventIntegration,
		logger:           logger,
	}, nil
}

func (ot *OutlierInterpolatorTransformer) Transform(startTime time.Time, endTime time.Time, dataPoints []metrics.DataPoint) ([]metrics.DataPoint, error) {
	var eventDetails []integration.EventDetails
	for _, ei := range ot.EventIntegration {
		events, err := ei.GetDesiredEvents(startTime, endTime)
		if err != nil {
			return nil, fmt.Errorf("error in getting events from event integration: %v", err)
		}
		eventDetails = append(eventDetails, events...)
	}
	intervals := getOutlierIntervals(eventDetails)
	intervals = filterIntervals(intervals, startTime, endTime)
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
	sort.SliceStable(intervals, func(i, j int) bool {
		return intervals[i].StartTime.Before(intervals[j].StartTime)
	})

	return intervals
}

// Handling Overlapping and Unnecessary Outlier Intervals
func filterIntervals(intervals []OutlierInterval, start time.Time, end time.Time) []OutlierInterval {
	var filteredInterval []OutlierInterval
	for _, interval := range intervals {
		if interval.StartTime.Before(end) && interval.EndTime.After(start) {
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
	var newDataPoints []metrics.DataPoint
	for _, dataPoint := range dataPoints {
		newDataPoints = append(newDataPoints, dataPoint)
	}
	for _, interval := range intervals {
		ot.logger.V(2).Info("Interpolating for interval: ", "start", interval.StartTime, "end", interval.EndTime)
		startIndex := -1
		endIndex := 0
		for i := 0; i < len(newDataPoints); i++ {
			if newDataPoints[i].Timestamp.After(interval.StartTime) && newDataPoints[i].Timestamp.Before(interval.EndTime) {
				if startIndex == -1 {
					startIndex = i
				}
				endIndex = i
			}
		}

		if endIndex >= len(newDataPoints)-1 {
			//Remove ending dataPoints
			newDataPoints = newDataPoints[0:startIndex]
		} else if startIndex <= 0 {
			//Remove starting dataPoints
			newDataPoints = newDataPoints[(endIndex + 1):]
		} else {
			//Interpolate data
			actStartIndex := startIndex - 1
			actEndIndex := endIndex + 1
			timeDiff := newDataPoints[actEndIndex].Timestamp.Sub(newDataPoints[actStartIndex].Timestamp)
			slope := (newDataPoints[actEndIndex].Value - newDataPoints[actStartIndex].Value) / (timeDiff.Seconds())
			for j := actStartIndex; j < actEndIndex-1; j++ {
				newDataPoints[j+1].Value = newDataPoints[j].Value + slope*(newDataPoints[j+1].Timestamp.Sub(newDataPoints[j].Timestamp).Seconds())
			}
		}
	}
	return newDataPoints
}
