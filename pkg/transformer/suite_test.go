/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package transformer

import (
	"github.com/flipkart-incubator/ottoscalr/pkg/integration"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

func TestMetrics(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metrics Suite")
}

type FakeEventIntegration struct{}
type FakeNFREventIntegration struct{}

var (
	outlierInterpolatorTransformer *OutlierInterpolatorTransformer
	fakeEventIntegration           integration.EventIntegration
	fakeNFREventIntegration        integration.EventIntegration
)

func (fe *FakeEventIntegration) GetDesiredEvents(startTime time.Time,
	endTime time.Time) ([]integration.EventDetails, error) {
	time1 := time.Now().Add(-21 * time.Minute)
	time2 := time.Now().Add(-15 * time.Minute)
	return []integration.EventDetails{
		{
			EventName: "event1",
			EventId:   "1234",
			StartTime: time1,
			EndTime:   time2,
		},
	}, nil
}

func (fne *FakeNFREventIntegration) GetDesiredEvents(startTime time.Time,
	endTime time.Time) ([]integration.EventDetails, error) {
	time3 := time.Now().Add(-10 * time.Minute)
	time4 := time.Now().Add(-8 * time.Minute)
	return []integration.EventDetails{
		{
			EventName: "nfr",
			EventId:   "12345",
			StartTime: time3,
			EndTime:   time4,
		},
	}, nil
}

var _ = BeforeSuite(func() {

	fakeEventIntegration = &FakeEventIntegration{}
	fakeNFREventIntegration = &FakeNFREventIntegration{}

	outlierInterpolatorTransformer, _ = NewOutlierInterpolatorTransformer(fakeEventIntegration, fakeNFREventIntegration)
})
