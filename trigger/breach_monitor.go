package trigger

import (
	"context"
	"github.com/flipkart-incubator/ottoscalr/metrics"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"sync"
	"time"
)

type BreachMonitorManager struct {
	metricScraper metrics.Scraper
	checkInterval time.Duration
	handlerFunc   func(workloadName string)
	monitors      map[string]*BreachMonitor
	monitorMutex  sync.Mutex
	logger        logr.Logger
}

const (
	metricStep         = 30 * time.Second
	redLineUtilization = 0.85
)

func NewBreachMonitorManager(metricScraper metrics.Scraper,
	checkInterval time.Duration,
	handlerFunc func(workloadName string),
	logger logr.Logger) *BreachMonitorManager {

	return &BreachMonitorManager{
		metricScraper: metricScraper,
		checkInterval: checkInterval,
		handlerFunc:   handlerFunc,
		monitors:      make(map[string]*BreachMonitor),
		logger:        logger,
	}
}

func (mf *BreachMonitorManager) RegisterBreachMonitor(namespace, workloadType, workloadName string) *BreachMonitor {

	workloadFullName := types.NamespacedName{
		Name:      workloadName,
		Namespace: namespace,
	}.String()
	mf.monitorMutex.Lock()
	defer mf.monitorMutex.Unlock()

	if monitor, ok := mf.monitors[workloadFullName]; ok {
		return monitor
	}

	monitor := NewBreachMonitor(namespace,
		workloadType,
		workloadName,
		mf.metricScraper,
		mf.checkInterval,
		mf.handlerFunc,
		mf.logger)

	mf.monitors[workloadFullName] = monitor
	monitor.Start()
	return monitor
}

func (mf *BreachMonitorManager) DeregisterBreachMonitor(namespace, workloadName string) {
	workloadFullName := types.NamespacedName{
		Name:      workloadName,
		Namespace: namespace,
	}.String()

	mf.monitorMutex.Lock()
	defer mf.monitorMutex.Unlock()
	if monitor, ok := mf.monitors[workloadFullName]; ok {
		monitor.Stop()
		delete(mf.monitors, workloadFullName)
	}
}

type BreachMonitor struct {
	namespace     string
	workloadType  string
	workloadName  string
	metricScraper metrics.Scraper
	checkInterval time.Duration
	handlerFunc   func(workloadName string)
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	logger        logr.Logger
}

func NewBreachMonitor(namespace string,
	workloadType string,
	workloadName string,
	metricScraper metrics.Scraper,
	checkInterval time.Duration,
	handlerFunc func(workloadName string),
	logger logr.Logger) *BreachMonitor {

	ctx, cancel := context.WithCancel(context.Background())
	return &BreachMonitor{
		namespace:     namespace,
		workloadType:  workloadType,
		workloadName:  workloadName,
		metricScraper: metricScraper,
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
					m.workloadName,
					redLineUtilization,
					start,
					end,
					metricStep)
				if err != nil {
					m.logger.Error(err, "Error while executing GetCPUUtilizationBreachDataPoints.Continuing.",
						"namespace", m.namespace,
						"workloadType", m.workloadType,
						"workloadName", m.workloadName)
				}
				if len(dataPoints) > 0 {
					m.handlerFunc(m.workloadName)
				}
			}
		}
	}()
}

func (m *BreachMonitor) Stop() {
	m.cancel()
	m.wg.Wait()
}
