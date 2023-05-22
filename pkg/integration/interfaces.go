package integration

import "time"

type EventDetails struct {
	EventName string
	EventId   string
	StartTime time.Time
	EndTime   time.Time
}

type EventIntegration interface {
	GetDesiredEvents(startTime time.Time,
		endTime time.Time) ([]EventDetails, error)
}
