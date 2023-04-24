package trigger

import (
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// FakeScraper mocks the metrics.Scraper for testing purposes
type FakeScraper struct{}

func (fs *FakeScraper) GetAverageCPUUtilizationByWorkload(namespace,
	workload string,
	start time.Time,
	end time.Time,
	step time.Duration) ([]float64, error) {
	return []float64{}, nil
}

func (fs *FakeScraper) GetCPUUtilizationBreachDataPoints(namespace,
	workloadType,
	workload string,
	redLineUtilization float32,
	start time.Time,
	end time.Time,
	step time.Duration) ([]float64, error) {
	return []float64{1.3}, nil
}

var _ = Describe("BreachMonitorManager and BreachMonitor", func() {
	var (
		manager       *BreachMonitorManager
		handlerCalled bool
	)

	BeforeEach(func() {
		handlerCalled = false
		handlerFunc := func(workload types.NamespacedName) {
			handlerCalled = true
		}
		manager = NewBreachMonitorManager(&FakeScraper{},
			1*time.Second,
			handlerFunc,
			10,
			80,
			zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	})

	It("should register and deregister a BreachMonitor", func() {
		workload := types.NamespacedName{Name: "test-workload", Namespace: "default"}
		workloadType := "test-workload-type"

		// Register a BreachMonitor
		monitor := manager.RegisterBreachMonitor(workloadType, workload)
		Expect(monitor).ToNot(BeNil())

		// Deregister the BreachMonitor
		manager.DeregisterBreachMonitor(workload)
	})

	It("should start and stop a BreachMonitor", func() {
		workload := types.NamespacedName{Name: "test-workload", Namespace: "default"}
		workloadType := "Rollout"

		monitor := NewBreachMonitor("default",
			workload, workloadType,
			&FakeScraper{},
			80,
			10*time.Second,
			1*time.Second,
			func(workload types.NamespacedName) {
				handlerCalled = true
			}, zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

		monitor.Start()
		time.Sleep(3 * time.Second)
		Expect(handlerCalled).To(BeTrue())
		monitor.Stop()
	})
})
