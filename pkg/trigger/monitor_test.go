package trigger

import (
	"fmt"
	ottoscaleriov1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	"github.com/flipkart-incubator/ottoscalr/pkg/metrics"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	step time.Duration) ([]metrics.DataPoint, error) {
	return []metrics.DataPoint{}, nil
}

func (fs *FakeScraper) GetCPUUtilizationBreachDataPoints(namespace,
	workloadType,
	workload string,
	redLineUtilization float64,
	start time.Time,
	end time.Time,
	step time.Duration) ([]metrics.DataPoint, error) {
	datapoint := metrics.DataPoint{Timestamp: time.Now(), Value: 1.3}
	return []metrics.DataPoint{datapoint}, nil
}
func (fs *FakeScraper) GetACLByWorkload(namespace,
	workload string) (time.Duration, error) {
	return 5 * time.Minute, nil
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
		policyreco ottoscaleriov1alpha1.PolicyRecommendation
	)

	BeforeEach(func() {
		err := createPolicyReco("test-workload", "default", "")
		Expect(err).ToNot(HaveOccurred())
		handlerCallCounter = 0
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(ctx, &policyreco)).Should(Succeed())
		manager.Shutdown()
	})

	It("should call handler when breaches are detected", func() {

		By("Creating a monitor mgr that only detects breaches")
		manager = NewPolicyRecommendationMonitorManager(k8sClient,
			recorder,
			&FakeScraper{},
			1*time.Hour,
			1*time.Second,
			50,
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
		err := k8sClient.Get(ctx, types.NamespacedName{
			Namespace: workload.Namespace,
			Name:      workload.Name,
		}, &policyreco)
		Expect(err).ToNot(HaveOccurred())
		fmt.Fprintf(GinkgoWriter, "PolicyReco: %v", policyreco)
		Expect(policyreco.Status.Conditions).To(ContainElement(SatisfyAll(
			HaveField("Type", Equal(string(ottoscaleriov1alpha1.HasBreached))),
			HaveField("Reason", Equal(BreachDetectedReason)))))
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
		manager = NewPolicyRecommendationMonitorManager(manager.k8sClient,
			manager.recorder,
			&FakeScraper{},
			1*time.Second,
			1*time.Hour,
			50,
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

func createPolicyReco(name, namespace, policy string) error {
	now := metav1.Now()
	return k8sClient.Create(ctx, &ottoscaleriov1alpha1.PolicyRecommendation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: ottoscaleriov1alpha1.PolicyRecommendationSpec{
			WorkloadMeta:            ottoscaleriov1alpha1.WorkloadMeta{},
			CurrentHPAConfiguration: ottoscaleriov1alpha1.HPAConfiguration{},
			Policy:                  policy,
			GeneratedAt:             &now,
			TransitionedAt:          &now,
		},
	})
}
