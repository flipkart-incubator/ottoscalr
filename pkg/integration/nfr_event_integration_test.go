package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	nfrEventIntegration *NFREventDataFetcher
)

var _ = Describe("GetDesiredEvents", func() {
	It("Should return the outlier intervals by fetching the event metadata and parsing it", func() {
		time1 := time.Now().Add(-20 * time.Minute)
		time2 := time.Now().Add(-14 * time.Minute)
		server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Define the mock endpoint logic
			// Handle unknown endpoints
			response := NFRResponse{

				Success: true,
				Error:   "",
				Status:  200,
				Message: []NFREventMetadata{
					{
						EventKey:  "123456",
						StartTime: "2023-06-04 13:30",
						EndTime:   "2023-06-04 18:30",
					},
					{
						EventKey:  "1234567",
						StartTime: "2023-06-06 13:30",
						EndTime:   "2023-06-06 18:30",
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
		defer server2.Close()

		server3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Define the mock endpoint logic
			// Handle unknown endpoints
			response := NFRResponse{

				Success: true,
				Error:   "",
				Status:  200,
				Message: []NFREventMetadata{
					{
						EventKey:  "123456",
						StartTime: "2023-06-04 13:30",
						EndTime:   "2023-06-04 18:30",
					},
					{
						EventKey:  "1234567",
						StartTime: "2023-06-06 13:30",
						EndTime:   "2023-06-06 18:30",
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
		defer server3.Close()

		nfrEventIntegration, _ = NewNFREventDataFetcher(server2.URL, server3.URL, 2*time.Second, 0*time.Second, logger)
		time.Sleep(2 * time.Second)

		events, err := nfrEventIntegration.GetDesiredEvents(time1, time2)
		fmt.Fprintf(GinkgoWriter, "events: %v\n", events)
		Expect(len(events)).To(Equal(4))
		Expect(err).NotTo(HaveOccurred())
		Expect(events).To(ContainElements([]EventDetails{
			{EventName: "nfr", EventId: "123456", StartTime: formatTime("2023-06-04 13:30"), EndTime: formatTime("2023-06-04 18:30")},
			{EventName: "nfr", EventId: "1234567", StartTime: formatTime("2023-06-06 13:30"), EndTime: formatTime("2023-06-06 18:30")},
			{EventName: "nfr", EventId: "123456", StartTime: formatTime("2023-06-04 13:30"), EndTime: formatTime("2023-06-04 18:30")},
			{EventName: "nfr", EventId: "1234567", StartTime: formatTime("2023-06-06 13:30"), EndTime: formatTime("2023-06-06 18:30")},
		}))
	})
})
