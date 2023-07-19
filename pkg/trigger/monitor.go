package trigger

import (
	"context"
	ottoscaleriov1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	"github.com/flipkart-incubator/ottoscalr/pkg/metrics"
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"math/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	p8smetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	"sync"
	"time"
)

var (
	breachGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{Name: "breachmonitor_breached_counter",
			Help: "Number of breaches detected counter"}, []string{"namespace", "policyreco", "workloadKind", "workload"},
	)
	timeToMitigateLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{Name: "breachmonitor_mitigation_latency_seconds",
			Help: "Time to mitigate breach latency in seconds"}, []string{"namespace", "policyreco", "workloadKind", "workload"},
	)
)

func init() {
	p8smetrics.Registry.MustRegister(breachGauge, timeToMitigateLatency)
}

const (
	eventTypeWarning    = "Warning"
	BreachStatusManager = "BreachStatusManager"

	BreachDetectedReason   = "BreachDetected"
	NoBreachDetectedReason = "NoBreachDetected"

	BreachDetectedMessage   = "A breach has been detected"
	NoBreachDetectedMessage = "No breach detected for the current recommendation"
)

type MonitorManager interface {
	RegisterMonitor(workloadType string, workload types.NamespacedName) *Monitor
	DeregisterMonitor(workload types.NamespacedName)
	Shutdown()
}
type PolicyRecommendationMonitorManager struct {
	k8sClient                client.Client
	recorder                 record.EventRecorder
	metricScraper            metrics.Scraper
	metricStep               time.Duration
	cpuRedLine               float64
	periodicRequeueFrequency time.Duration
	breachCheckFrequency     time.Duration
	handlerFunc              func(workloadName types.NamespacedName)
	monitors                 map[string]*Monitor
	monitorMutex             sync.Mutex
	logger                   logr.Logger
}

func NewPolicyRecommendationMonitorManager(k8sClient client.Client,
	recorder record.EventRecorder,
	metricScraper metrics.Scraper,
	periodicRequeueFrequency time.Duration,
	breachCheckFrequency time.Duration,
	handlerFunc func(workloadName types.NamespacedName),
	stepSec int,
	cpuRedLine float64,
	logger logr.Logger) *PolicyRecommendationMonitorManager {

	return &PolicyRecommendationMonitorManager{
		k8sClient:                k8sClient,
		recorder:                 recorder,
		metricScraper:            metricScraper,
		metricStep:               time.Duration(stepSec) * time.Second,
		cpuRedLine:               cpuRedLine,
		periodicRequeueFrequency: periodicRequeueFrequency,
		breachCheckFrequency:     breachCheckFrequency,
		handlerFunc:              handlerFunc,
		monitors:                 make(map[string]*Monitor),
		logger:                   logger,
	}
}

func (mf *PolicyRecommendationMonitorManager) RegisterMonitor(workloadType string,
	workload types.NamespacedName) *Monitor {

	mf.monitorMutex.Lock()
	defer mf.monitorMutex.Unlock()

	if monitor, ok := mf.monitors[workload.String()]; ok {
		return monitor
	}

	monitor := NewMonitor(mf.k8sClient,
		mf.recorder,
		workload.Namespace,
		workload,
		workloadType,
		mf.metricScraper,
		mf.cpuRedLine,
		mf.metricStep,
		mf.periodicRequeueFrequency,
		mf.breachCheckFrequency,
		mf.handlerFunc,
		mf.logger)

	mf.monitors[workload.String()] = monitor
	monitor.Start()
	return monitor
}

func (mf *PolicyRecommendationMonitorManager) DeregisterMonitor(workload types.NamespacedName) {

	mf.monitorMutex.Lock()
	defer mf.monitorMutex.Unlock()
	if monitor, ok := mf.monitors[workload.String()]; ok {
		monitor.Stop()
		delete(mf.monitors, workload.String())
	}
}

func (mf *PolicyRecommendationMonitorManager) Shutdown() {
	mf.logger.Info("Shutting down.")
	mf.monitorMutex.Lock()
	defer mf.monitorMutex.Unlock()

	for _, monitor := range mf.monitors {
		monitor.Stop()
	}
}

type Monitor struct {
	k8sClient                client.Client
	recorder                 record.EventRecorder
	namespace                string
	workload                 types.NamespacedName
	workloadType             string
	metricScraper            metrics.Scraper
	cpuRedLine               float64
	metricStep               time.Duration
	periodicRequeueFrequency time.Duration
	breachCheckFrequency     time.Duration
	handlerFunc              func(workload types.NamespacedName)
	ctx                      context.Context
	cancel                   context.CancelFunc
	wg                       sync.WaitGroup
	logger                   logr.Logger
}

func NewMonitor(k8sClient client.Client,
	recorder record.EventRecorder,
	namespace string,
	workload types.NamespacedName,
	workloadType string,
	metricScraper metrics.Scraper,
	cpuRedLine float64,
	metricStep time.Duration,
	periodicRequeueFrequency time.Duration,
	breachCheckFrequency time.Duration,
	handlerFunc func(workload types.NamespacedName),
	logger logr.Logger) *Monitor {

	ctx, cancel := context.WithCancel(context.Background())
	return &Monitor{
		k8sClient:                k8sClient,
		recorder:                 recorder,
		namespace:                namespace,
		workload:                 workload,
		workloadType:             workloadType,
		metricScraper:            metricScraper,
		cpuRedLine:               cpuRedLine,
		metricStep:               metricStep,
		periodicRequeueFrequency: periodicRequeueFrequency,
		breachCheckFrequency:     breachCheckFrequency,
		handlerFunc:              handlerFunc,
		ctx:                      ctx,
		cancel:                   cancel,
		logger:                   logger,
	}
}

func (m *Monitor) Start() {
	m.wg.Add(1)
	go m.monitorBreaches()

	m.wg.Add(1)
	go m.requeueAfterFixedInterval()
}

func (m *Monitor) monitorBreaches() {
	defer m.wg.Done()

	m.logger.Info("Starting the breach monitor routine.")

	ticker := time.NewTicker(m.breachCheckFrequency)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.logger.Info("Executing breach monitor check.", "workload", m.workload)
			end := time.Now()
			start := end.Add(-m.breachCheckFrequency)

			policyreco := ottoscaleriov1alpha1.PolicyRecommendation{}
			if err := m.k8sClient.Get(context.Background(), types.NamespacedName{
				Namespace: m.workload.Namespace,
				Name:      m.workload.Name,
			}, &policyreco); err != nil {
				m.logger.Error(err, "Error while getting policyRecommendation.", "workload", m.workload)
			}
			var breachedInPast bool
			lastBreachedTime := time.Now()
			for _, condition := range policyreco.Status.Conditions {
				if string(ottoscaleriov1alpha1.HasBreached) == condition.Type {
					if condition.Status == metav1.ConditionTrue {
						breachedInPast = true
						lastBreachedTime = condition.LastTransitionTime.Time
					} else {
						breachedInPast = false
						lastBreachedTime = condition.LastTransitionTime.Time
					}
				}
			}
			var statusPatch *ottoscaleriov1alpha1.PolicyRecommendation
			breached, _ := HasBreached(log.IntoContext(context.Background(), m.logger), start, end, m.workloadType, m.workload, m.metricScraper, m.cpuRedLine, m.metricStep)
			var lastTransitionTime time.Time
			if breached == breachedInPast {
				lastTransitionTime = lastBreachedTime
			} else {
				lastTransitionTime = time.Now()
			}
			if breached {
				m.recorder.Event(&policyreco, eventTypeWarning, "BreachDetected", "A breach has been detected for the current policy")
				statusPatch = m.createBreachCondition(ottoscaleriov1alpha1.HasBreached, metav1.ConditionTrue, BreachDetectedReason, BreachDetectedMessage, lastTransitionTime)
				if err := m.k8sClient.Status().Patch(context.Background(), statusPatch, client.Apply, getSubresourcePatchOptions(BreachStatusManager)); err != nil {
					m.logger.Error(err, "Error updating the status of the policy reco object")
				}
				breachGauge.WithLabelValues(policyreco.Namespace, policyreco.Name, policyreco.Spec.WorkloadMeta.Kind, policyreco.Spec.WorkloadMeta.Name).Set(1)
				m.handlerFunc(m.workload)
			} else {
				breachGauge.WithLabelValues(policyreco.Namespace, policyreco.Name, policyreco.Spec.WorkloadMeta.Kind, policyreco.Spec.WorkloadMeta.Name).Set(0)
				statusPatch = m.createBreachCondition(ottoscaleriov1alpha1.HasBreached, metav1.ConditionFalse, NoBreachDetectedReason, NoBreachDetectedMessage, lastTransitionTime)
				if err := m.k8sClient.Status().Patch(context.Background(), statusPatch, client.Apply, getSubresourcePatchOptions(BreachStatusManager)); err != nil {
					m.logger.Error(err, "Error updating the status of the policy reco object")
				}
				if breachedInPast {
					mitigationLatency := time.Since(lastBreachedTime).Seconds()
					timeToMitigateLatency.WithLabelValues(policyreco.Namespace, policyreco.Name, policyreco.Spec.WorkloadMeta.Kind, policyreco.Spec.WorkloadMeta.Name).Observe(mitigationLatency)
				}
			}
		}
	}
}

func HasBreached(ctx context.Context, start, end time.Time, workloadType string,
	workload types.NamespacedName,
	metricScraper metrics.Scraper,
	cpuRedLine float64,
	metricStep time.Duration) (bool, error) {
	logger := log.FromContext(ctx)
	dataPoints, err := metricScraper.GetCPUUtilizationBreachDataPoints(workload.Namespace,
		workloadType,
		workload.Name,
		cpuRedLine,
		start,
		end,
		metricStep)
	if err != nil {
		logger.Error(err, "Error while executing GetCPUUtilizationBreachDataPoints.Continuing.",
			"namespace", workload.Namespace,
			"workloadType", workloadType,
			"workloadName", workload.Name)
		return false, err
	}
	if len(dataPoints) > 0 {
		return true, nil
	}
	return false, nil
}

func (m *Monitor) requeueAfterFixedInterval() {
	defer m.wg.Done()

	m.logger.Info("Starting the periodic check routine.")
	//Add a jitter of 10%
	jitter := time.Duration(rand.Int63n(int64(m.periodicRequeueFrequency) / 10))
	queueTicker := time.NewTicker(m.periodicRequeueFrequency + jitter)

	defer queueTicker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-queueTicker.C:
			m.logger.Info("Executing the periodic check routine.")
			m.handlerFunc(m.workload)
		}
	}
}

func (m *Monitor) Stop() {
	m.logger.Info("Stopping monitor.")
	m.cancel()
	m.wg.Wait()
}

func (m *Monitor) createBreachCondition(
	condType ottoscaleriov1alpha1.PolicyRecommendationConditionType,
	status metav1.ConditionStatus, reason, message string, lastTransitionTime time.Time) *ottoscaleriov1alpha1.PolicyRecommendation {

	return &ottoscaleriov1alpha1.PolicyRecommendation{
		TypeMeta: metav1.TypeMeta{
			APIVersion: ottoscaleriov1alpha1.GroupVersion.String(),
			Kind:       "PolicyRecommendation",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.workload.Name,
			Namespace: m.workload.Namespace,
		},
		Status: ottoscaleriov1alpha1.PolicyRecommendationStatus{
			Conditions: []metav1.Condition{
				{
					Type:   string(condType),
					Status: status,
					LastTransitionTime: metav1.Time{
						Time: lastTransitionTime,
					},
					Reason:  reason,
					Message: message,
				},
			},
		},
	}
}
