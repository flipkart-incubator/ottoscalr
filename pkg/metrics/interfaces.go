package metrics

import "time"

type MetricsTransformer interface {
	GetOutlierIntervalsAndInterpolate(
		startTime time.Time, dataPoints []DataPoint) ([]DataPoint, error)
}
