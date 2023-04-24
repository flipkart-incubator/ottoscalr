package trigger

import (
	"context"
	"github.com/flipkart-incubator/ottoscalr/internal/metrics"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"sync"
	"time"
)

type MonitorManager interface {
	RegisterBreachMonitor(workloadType string, workload types.NamespacedName) *BreachMonitor
	DeregisterBreachMonitor(workload types.NamespacedName)
	Shutdown()
}
type BreachMonitorManager struct {
	metricScraper metrics.Scraper
	metricStep    time.Duration
	cpuRedLine    float32
	checkInterval time.Duration
	handlerFunc   func(workloadName types.NamespacedName)
	monitors      map[string]*BreachMonitor
	monitorMutex  sync.Mutex
	logger        logr.Logger
}

func NewBreachMonitorManager(metricScraper metrics.Scraper,
	checkInterval time.Duration,
	handlerFunc func(workloadName types.NamespacedName),
	stepSec int,
	cpuRedLine float32,
	logger logr.Logger) *BreachMonitorManager {

	return &BreachMonitorManager{
		metricScraper: metricScraper,
		metricStep:    time.Duration(stepSec) * time.Second,
		cpuRedLine:    cpuRedLine,
		checkInterval: checkInterval,
		handlerFunc:   handlerFunc,
		monitors:      make(map[string]*BreachMonitor),
		logger:        logger,
	}
}

func (mf *BreachMonitorManager) RegisterBreachMonitor(workloadType string,
	workload types.NamespacedName) *BreachMonitor {

	mf.monitorMutex.Lock()
	defer mf.monitorMutex.Unlock()

	if monitor, ok := mf.monitors[workload.String()]; ok {
		return monitor
	}

	monitor := NewBreachMonitor(workload.Namespace,
		workload,
		workloadType,
		mf.metricScraper,
		mf.cpuRedLine,
		mf.metricStep,
		mf.checkInterval,
		mf.handlerFunc,
		mf.logger)

	mf.monitors[workload.String()] = monitor
	monitor.Start()
	return monitor
}

func (mf *BreachMonitorManager) DeregisterBreachMonitor(workload types.NamespacedName) {

	mf.monitorMutex.Lock()
	defer mf.monitorMutex.Unlock()
	if monitor, ok := mf.monitors[workload.String()]; ok {
		monitor.Stop()
		delete(mf.monitors, workload.String())
	}
}

func (mf *BreachMonitorManager) Shutdown() {
	mf.logger.Info("Shutting down.")
	mf.monitorMutex.Lock()
	defer mf.monitorMutex.Unlock()

	for _, monitor := range mf.monitors {
		monitor.Stop()
	}
}

type BreachMonitor struct {
	namespace     string
	workload      types.NamespacedName
	workloadType  string
	metricScraper metrics.Scraper
	checkInterval time.Duration
	cpuRedLine    float32
	metricStep    time.Duration
	handlerFunc   func(workload types.NamespacedName)
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	logger        logr.Logger
}

func NewBreachMonitor(namespace string,
	workload types.NamespacedName,
	workloadType string,
	metricScraper metrics.Scraper,
	cpuRedLine float32,
	metricStep time.Duration,
	checkInterval time.Duration,
	handlerFunc func(workload types.NamespacedName),
	logger logr.Logger) *BreachMonitor {

	ctx, cancel := context.WithCancel(context.Background())
	return &BreachMonitor{
		namespace:     namespace,
		workload:      workload,
		workloadType:  workloadType,
		metricScraper: metricScraper,
		cpuRedLine:    cpuRedLine,
		metricStep:    metricStep,
		checkInterval: checkInterval,
		handlerFunc:   handlerFunc,
		ctx:           ctx,
		cancel:        cancel,
		logger:        logger,
	}
}

func (m *BreachMonitor) Start() {
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		ticker := time.NewTicker(m.checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-m.ctx.Done():
				return
			case <-ticker.C:
				end := time.Now()
				start := end.Add(-m.checkInterval)
				dataPoints, err := m.metricScraper.GetCPUUtilizationBreachDataPoints(m.namespace,
					m.workloadType,
					m.workload.Name,
					m.cpuRedLine,
					start,
					end,
					m.metricStep)
				if err != nil {
					m.logger.Error(err, "Error while executing GetCPUUtilizationBreachDataPoints.Continuing.",
						"namespace", m.namespace,
						"workloadType", m.workloadType,
						"workloadName", m.workload.Name)
				}
				if len(dataPoints) > 0 {
					m.handlerFunc(m.workload)
				}
			}
		}
	}()
}

func (m *BreachMonitor) Stop() {
	m.logger.Info("Stopping monitor.")
	m.cancel()
	m.wg.Wait()
}
