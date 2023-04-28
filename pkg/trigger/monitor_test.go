package trigger

import (
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sync/atomic"
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

func (fs *FakeScraper) GetPodReadyLatencyByWorkload(namespace,
	workload string) (float64, error) {
	return 0.0, nil
}

var _ = Describe("PolicyRecommendationMonitorManager and Monitor", func() {
	var (
		manager            *PolicyRecommendationMonitorManager
		handlerCallCounter int32
		handlerFunc        = func(workload types.NamespacedName) {
			atomic.AddInt32(&handlerCallCounter, 1)
		}
	)

	BeforeEach(func() {
		handlerCallCounter = 0
	})

	AfterEach(func() {
		manager.Shutdown()
	})

	It("should call handler when breaches are detected", func() {

		By("Creating a monitor mgr that only detects breaches")
		manager = NewPolicyRecommendationMonitorManager(&FakeScraper{},
			1*time.Hour,
			1*time.Second,
			handlerFunc,
			10,
			80,
			zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
		workload := types.NamespacedName{Name: "test-workload", Namespace: "default"}
		workloadType := "test-workload-type"

		By("Registering the monitor and checking that handler is called periodically")
		monitor := manager.RegisterMonitor(workloadType, workload)
		Expect(monitor).ToNot(BeNil())

		time.Sleep(3 * time.Second)
		Expect(handlerCallCounter).To(BeNumerically(">=", 2))
		Expect(handlerCallCounter).To(BeNumerically("<=", 3))

		By("Deregistering the counter and checking that handler is not called anymore")
		manager.DeregisterMonitor(workload)

		currentCallCounter := handlerCallCounter
		time.Sleep(3 * time.Second)
		Expect(handlerCallCounter).To(Equal(currentCallCounter))

		By("Reregistering the counter and checking that handler is not called anymore")
		monitor = manager.RegisterMonitor(workloadType, workload)

		currentCallCounter = handlerCallCounter
		time.Sleep(3 * time.Second)
		Expect(handlerCallCounter).To(BeNumerically(">", currentCallCounter))
	})

	It("should call handler when periodic trigger is fired", func() {

		By("Creating a monitor mgr that only handles periodic trigger")
		manager = NewPolicyRecommendationMonitorManager(&FakeScraper{},
			1*time.Second,
			1*time.Hour,
			handlerFunc,
			10,
			80,
			zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
		workload := types.NamespacedName{Name: "test-workload", Namespace: "default"}
		workloadType := "test-workload-type"

		By("Registering the monitor and checking that handler is called periodically")
		monitor := manager.RegisterMonitor(workloadType, workload)
		Expect(monitor).ToNot(BeNil())

		time.Sleep(3 * time.Second)
		Expect(handlerCallCounter).To(BeNumerically(">=", 2))
		Expect(handlerCallCounter).To(BeNumerically("<=", 8))

		By("Deregistering the counter and checking that handler is not called anymore")
		manager.DeregisterMonitor(workload)

		currentCallCounter := handlerCallCounter
		time.Sleep(3 * time.Second)
		Expect(handlerCallCounter).To(Equal(currentCallCounter))

		By("Reregistering the counter and checking that handler is not called anymore")
		monitor = manager.RegisterMonitor(workloadType, workload)

		currentCallCounter = handlerCallCounter
		time.Sleep(3 * time.Second)
		Expect(handlerCallCounter).To(BeNumerically(">", currentCallCounter))
	})
})
