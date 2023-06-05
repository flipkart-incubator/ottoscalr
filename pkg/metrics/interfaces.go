package metrics

import "time"

type MetricsTransformer interface {
	Transform(
		startTime time.Time, endTime time.Time, dataPoints []DataPoint) ([]DataPoint, error)
}
