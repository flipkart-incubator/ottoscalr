package trigger

import (
	"context"
	"github.com/flipkart-incubator/ottoscalr/pkg/metrics"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"math/rand"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sync"
	"time"
)

type MonitorManager interface {
	RegisterMonitor(workloadType string, workload types.NamespacedName) *Monitor
	DeregisterMonitor(workload types.NamespacedName)
	Shutdown()
}
type PolicyRecommendationMonitorManager struct {
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

func NewPolicyRecommendationMonitorManager(metricScraper metrics.Scraper,
	periodicRequeueFrequency time.Duration,
	breachCheckFrequency time.Duration,
	handlerFunc func(workloadName types.NamespacedName),
	stepSec int,
	cpuRedLine float64,
	logger logr.Logger) *PolicyRecommendationMonitorManager {

	return &PolicyRecommendationMonitorManager{
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

	monitor := NewMonitor(workload.Namespace,
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

func NewMonitor(namespace string,
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
			if breached, _ := HasBreached(log.IntoContext(context.Background(), m.logger), start, end, m.workloadType, m.workload, m.metricScraper, m.cpuRedLine, m.metricStep); breached {
				m.handlerFunc(m.workload)
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
