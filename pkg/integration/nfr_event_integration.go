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

type NFRResponse struct {
	Success bool               `json:"success"`
	Error   string             `json:"error"`
	Status  int                `json:"status"`
	Message []NFREventMetadata `json:"message"`
}

type NFREventMetadata struct {
	EventKey  string `json:"nfrEventKey"`
	StartTime string `json:"startDateTime"`
	EndTime   string `json:"endDateTime"`
	Notes     string `json:"notes"`
}

type NFREventDataFetcher struct {
	Client                *http.Client
	NFREventAPIEndpoint   string
	NFREventCache         []EventDetails
	NFREventFetchDuration time.Duration
	logger                logr.Logger
	lock                  sync.RWMutex
	ctx                   context.Context
	Cancel                context.CancelFunc
}

func NewNFREventDataFetcher(nfrEventAPIEndpoint string, nfrEventFetchDuration time.Duration, logger logr.Logger) (*NFREventDataFetcher, error) {
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 10

	client := retryClient.StandardClient()
	ctx, cancel := context.WithCancel(context.Background())

	ne := &NFREventDataFetcher{
		Client:                client,
		NFREventAPIEndpoint:   nfrEventAPIEndpoint,
		NFREventCache:         nil,
		NFREventFetchDuration: nfrEventFetchDuration,
		logger:                logger,
		lock:                  sync.RWMutex{},
		ctx:                   ctx,
		Cancel:                cancel,
	}
	start := time.Now().Add(-60 * 24 * time.Hour)
	end := time.Now()

	err := ne.populateNFREventCache(start, end)

	if err != nil {
		logger.Error(err, "Error in fetching data from nfr event calendar")
	}

	go ne.Start()
	return ne, nil
}

func (ne *NFREventDataFetcher) Start() {
	ticker := time.NewTicker(ne.NFREventFetchDuration)
	defer ticker.Stop()

	for {
		select {
		case <-ne.ctx.Done():
			return
		case <-ticker.C:
			start := time.Now().Add(-60 * 24 * time.Hour)
			end := time.Now()
			err := ne.populateNFREventCache(start, end)

			if err != nil {
				ne.logger.Error(err, "Error in fetching data from event calendar", "eventCache", ne.NFREventCache)
			}
		}
	}
}

func (ne *NFREventDataFetcher) GetDesiredEvents(startTime time.Time, endTime time.Time) ([]EventDetails, error) {
	ne.lock.RLock()
	defer ne.lock.RUnlock()
	return ne.NFREventCache, nil
}

func (ne *NFREventDataFetcher) populateNFREventCache(startTime time.Time, endTime time.Time) error {
	var eventDetails []EventDetails
	url := fmt.Sprintf("%s", ne.NFREventAPIEndpoint)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("error creating request for fetching past nfr events list: %v", err)
	}
	//set the headers
	req.Header.Add("Content-Type", "application/json")
	resp, err := ne.Client.Do(req)
	if err != nil {
		return fmt.Errorf("error calling nfr event api: %v", err)
	}
	var nfrEvents NFRResponse
	responseReader, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body from nfr event api: %v", err)
	}

	err = json.Unmarshal(responseReader, &nfrEvents)
	if err != nil {
		return fmt.Errorf("error while unmarshaling nfr event api json response: %v", err)
	}
	ne.logger.Info("List of fetched NFR events", "events", nfrEvents)
	//iterate over the nfrEvents List and parse it to required format
	for _, events := range nfrEvents.Message {
		start := formatTime(events.StartTime)
		end := formatTime(events.EndTime)
		eventDetail := EventDetails{
			EventName: "nfr",
			EventId:   events.EventKey,
			StartTime: start,
			EndTime:   end,
		}
		eventDetails = append(eventDetails, eventDetail)
	}
	ne.lock.Lock()
	defer ne.lock.Unlock()
	ne.NFREventCache = eventDetails
	return nil
}

func formatTime(tm string) time.Time {

	parse, _ := time.Parse("2006-01-02 15:04", tm)
	unixTime := parse.Unix()
	actualTm := time.Unix(0, unixTime*int64(time.Second))
	formattedTime := actualTm.Add(-330 * time.Minute)

	return formattedTime
}
