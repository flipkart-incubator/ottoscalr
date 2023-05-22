package transformer

import (
	"github.com/flipkart-incubator/ottoscalr/pkg/metrics"
	"math"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("cleanOutliersAndInterpolate", func() {
	It("Should clear the outlier data corresponding to the given interval and perform a Linear Interpolation", func() {
		dataPoints := []metrics.DataPoint{
			{Timestamp: time.Now().Add(-30 * time.Minute), Value: 60},
			{Timestamp: time.Now().Add(-29 * time.Minute), Value: 80},
			{Timestamp: time.Now().Add(-28 * time.Minute), Value: 100},
			{Timestamp: time.Now().Add(-27 * time.Minute), Value: 50},
			{Timestamp: time.Now().Add(-26 * time.Minute), Value: 30},
			{Timestamp: time.Now().Add(-25 * time.Minute), Value: 60},
			{Timestamp: time.Now().Add(-24 * time.Minute), Value: 80},
			{Timestamp: time.Now().Add(-23 * time.Minute), Value: 100},
			{Timestamp: time.Now().Add(-22 * time.Minute), Value: 50},
			{Timestamp: time.Now().Add(-21 * time.Minute), Value: 30},
			{Timestamp: time.Now().Add(-20 * time.Minute), Value: 60},
			{Timestamp: time.Now().Add(-19 * time.Minute), Value: 80},
			{Timestamp: time.Now().Add(-18 * time.Minute), Value: 100},
			{Timestamp: time.Now().Add(-17 * time.Minute), Value: 50},
			{Timestamp: time.Now().Add(-16 * time.Minute), Value: 30},
			{Timestamp: time.Now().Add(-15 * time.Minute), Value: 60},
			{Timestamp: time.Now().Add(-14 * time.Minute), Value: 80},
			{Timestamp: time.Now().Add(-13 * time.Minute), Value: 100},
			{Timestamp: time.Now().Add(-12 * time.Minute), Value: 50},
			{Timestamp: time.Now().Add(-11 * time.Minute), Value: 30},
			{Timestamp: time.Now().Add(-10 * time.Minute), Value: 60},
			{Timestamp: time.Now().Add(-9 * time.Minute), Value: 80},
			{Timestamp: time.Now().Add(-8 * time.Minute), Value: 100},
			{Timestamp: time.Now().Add(-7 * time.Minute), Value: 50},
			{Timestamp: time.Now().Add(-6 * time.Minute), Value: 30},
		}

		//Interval in between
		intervals := []OutlierInterval{
			{StartTime: time.Now().Add(-20 * time.Minute), EndTime: time.Now().Add(-15 * time.Minute)},
		}
		newDataPoints := outlierInterpolatorTransformer.cleanOutliersAndInterpolate(dataPoints, intervals)
		Expect(math.Floor(newDataPoints[11].Value*100) / 100).To(Equal(63.33))

		//Interval from start
		intervals = []OutlierInterval{
			{StartTime: time.Now().Add(-31 * time.Minute), EndTime: time.Now().Add(-25 * time.Minute)},
		}
		newDataPoints = outlierInterpolatorTransformer.cleanOutliersAndInterpolate(dataPoints, intervals)
		Expect(math.Floor(newDataPoints[0].Value*100) / 100).To(Equal(0.00))
		Expect(math.Floor(newDataPoints[2].Value*100) / 100).To(Equal(26.66))

		//Interval at end
		intervals = []OutlierInterval{
			{StartTime: time.Now().Add(-9 * time.Minute), EndTime: time.Now().Add(2 * time.Minute)},
		}
		newDataPoints = outlierInterpolatorTransformer.cleanOutliersAndInterpolate(dataPoints, intervals)
		Expect(math.Floor(newDataPoints[24].Value*100) / 100).To(Equal(0.00))
		Expect(math.Floor(newDataPoints[22].Value*100) / 100).To(Equal(53.33))

		//Overlapping intervals
		intervals = []OutlierInterval{
			{StartTime: time.Now().Add(-20 * time.Minute), EndTime: time.Now().Add(-15 * time.Minute)},
			{StartTime: time.Now().Add(-18 * time.Minute), EndTime: time.Now().Add(-10 * time.Minute)},
		}
		newData := outlierInterpolatorTransformer.cleanOutliersAndInterpolate(dataPoints, intervals)
		Expect(math.Floor(newData[15].Value*100) / 100).To(Equal(69.09))
		Expect(math.Floor(newData[12].Value*100) / 100).To(Equal(63.63))
	})
})

var _ = Describe("filterIntervals", func() {
	It("Should clear any overlapping intervals as well as interval starting after current time", func() {
		time1 := time.Now().Add(-20 * time.Minute)
		time2 := time.Now().Add(-15 * time.Minute)
		time3 := time.Now().Add(20 * time.Minute)
		time4 := time.Now().Add(25 * time.Minute)
		time5 := time.Now().Add(-18 * time.Minute)
		time6 := time.Now().Add(-10 * time.Minute)
		time7 := time.Now().Add(-9 * time.Minute)
		time8 := time.Now().Add(-1 * time.Minute)
		intervals := []OutlierInterval{
			{StartTime: time1, EndTime: time2},
			{StartTime: time3, EndTime: time4},
			{StartTime: time5, EndTime: time6},
			{StartTime: time7, EndTime: time8},
		}
		filteredInterval := filterIntervals(intervals)
		Expect(filteredInterval).To(HaveLen(2))
		Expect(filteredInterval).To(ContainElements([]OutlierInterval{{StartTime: time1, EndTime: time6},
			{StartTime: time7, EndTime: time8}}))
	})
})

var _ = Describe("Transform", func() {
	It("Should transform the given dataPoints", func() {
		dataPoints := []metrics.DataPoint{
			{Timestamp: time.Now().Add(-30 * time.Minute), Value: 60},
			{Timestamp: time.Now().Add(-29 * time.Minute), Value: 80},
			{Timestamp: time.Now().Add(-28 * time.Minute), Value: 100},
			{Timestamp: time.Now().Add(-27 * time.Minute), Value: 50},
			{Timestamp: time.Now().Add(-26 * time.Minute), Value: 30},
			{Timestamp: time.Now().Add(-25 * time.Minute), Value: 60},
			{Timestamp: time.Now().Add(-24 * time.Minute), Value: 80},
			{Timestamp: time.Now().Add(-23 * time.Minute), Value: 100},
			{Timestamp: time.Now().Add(-22 * time.Minute), Value: 50},
			{Timestamp: time.Now().Add(-21 * time.Minute), Value: 30},
			{Timestamp: time.Now().Add(-20 * time.Minute), Value: 60},
			{Timestamp: time.Now().Add(-19 * time.Minute), Value: 80},
			{Timestamp: time.Now().Add(-18 * time.Minute), Value: 100},
			{Timestamp: time.Now().Add(-17 * time.Minute), Value: 50},
			{Timestamp: time.Now().Add(-16 * time.Minute), Value: 30},
			{Timestamp: time.Now().Add(-15 * time.Minute), Value: 60},
			{Timestamp: time.Now().Add(-14 * time.Minute), Value: 80},
			{Timestamp: time.Now().Add(-13 * time.Minute), Value: 100},
			{Timestamp: time.Now().Add(-12 * time.Minute), Value: 50},
			{Timestamp: time.Now().Add(-11 * time.Minute), Value: 30},
			{Timestamp: time.Now().Add(-10 * time.Minute), Value: 60},
			{Timestamp: time.Now().Add(-9 * time.Minute), Value: 80},
			{Timestamp: time.Now().Add(-8 * time.Minute), Value: 100},
			{Timestamp: time.Now().Add(-7 * time.Minute), Value: 50},
			{Timestamp: time.Now().Add(-6 * time.Minute), Value: 30},
		}
		start := time.Now().Add(-50 * time.Minute)
		end := time.Now()
		newDataPoints, err := outlierInterpolatorTransformer.Transform(start, end, dataPoints)
		Expect(err).NotTo(HaveOccurred())
		Expect(math.Floor(newDataPoints[12].Value*100) / 100).To(Equal(51.42))
	})
})
