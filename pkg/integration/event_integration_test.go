package integration

import (
	"encoding/json"
	"fmt"
	"github.com/go-logr/logr"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	eventIntegration *EventCalendarDataFetcher
	logger           logr.Logger
)

var _ = Describe("GetDesiredEvents", func() {
	It("Should return the outlier intervals by fetching the event metadata and parsing it", func() {
		time1 := time.Now().Add(-20 * time.Minute)
		time2 := time.Now().Add(-14 * time.Minute)
		time3 := time.Now().Add(-21 * time.Minute)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Define the mock endpoint logic
			// Handle unknown endpoints
			response := AllEvents{

				HasMore: true,
				Content: []EventMetadata{
					{
						EventName: "event1",
						EventId:   "1234",
						Lifecycle: LifecycleDTO{StartTime: time1.UnixMilli(), EndTime: time2.UnixMilli()},
						EaSlots:   []LifecycleDTO{{StartTime: time3.UnixMilli(), EndTime: time1.UnixMilli()}},
					},
					{
						EventName: "event2",
						EventId:   "12345",
						Lifecycle: LifecycleDTO{StartTime: time1.UnixMilli(), EndTime: time2.UnixMilli()},
					},
				},
			}
			jsonResponse, err := json.Marshal(response)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				Expect(err).NotTo(HaveOccurred())
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, "%s", string(jsonResponse))
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		eventIntegration, _ = NewEventCalendarDataFetcher(server.URL, 2*time.Second, logger)
		time.Sleep(2 * time.Second)

		events, err := eventIntegration.GetDesiredEvents(time1, time2)
		Expect(err).NotTo(HaveOccurred())
		Expect(events).To(ContainElements([]EventDetails{
			{EventName: "event1", EventId: "1234", StartTime: time3.Truncate(time.Millisecond), EndTime: time2.Truncate(time.Millisecond)},
			{EventName: "event2", EventId: "12345", StartTime: time1.Truncate(time.Millisecond), EndTime: time2.Truncate(time.Millisecond)}}))
	})
})
