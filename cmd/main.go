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

package main

import (
	"context"
	"flag"
	argov1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/flipkart-incubator/ottoscalr/pkg/controller"
	"github.com/flipkart-incubator/ottoscalr/pkg/integration"
	"github.com/flipkart-incubator/ottoscalr/pkg/metrics"
	"github.com/flipkart-incubator/ottoscalr/pkg/policy"
	"github.com/flipkart-incubator/ottoscalr/pkg/reco"
	"github.com/flipkart-incubator/ottoscalr/pkg/transformer"
	"github.com/flipkart-incubator/ottoscalr/pkg/trigger"
	kedaapi "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/spf13/viper"
	"os"
	"os/signal"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"syscall"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	ottoscaleriov1alpha1 "github.com/flipkart-incubator/ottoscalr/api/v1alpha1"
	//+kubebuilder:scaffold:imports
)

var (
	scaledTargetName = "spec.scaleTargetRef.name"
	scheme           = runtime.NewScheme()
	setupLog         = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(argov1alpha1.AddToScheme(scheme))
	utilruntime.Must(ottoscaleriov1alpha1.AddToScheme(scheme))
	utilruntime.Must(kedaapi.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

type Config struct {
	Port                   int    `yaml:"port"`
	MetricBindAddress      string `yaml:"metricBindAddress"`
	HealthProbeBindAddress string `yaml:"healthProbeBindAddress"`
	EnableLeaderElection   bool   `yaml:"enableLeaderElection"`
	LeaderElectionID       string `yaml:"leaderElectionID"`
	MetricsScraper         struct {
		PrometheusUrl        string `yaml:"prometheusUrl"`
		QueryTimeoutSec      int    `yaml:"queryTimeoutSec"`
		QuerySplitIntervalHr int    `yaml:"querySplitIntervalHr"`
	} `yaml:"metricsScraper"`

	BreachMonitor struct {
		PollingIntervalSec int     `yaml:"pollingIntervalSec"`
		CpuRedLine         float64 `yaml:"cpuRedLine"`
		StepSec            int     `yaml:"stepSec"`
	} `yaml:"breachMonitor"`

	PeriodicTrigger struct {
		PollingIntervalMin int `yaml:"pollingIntervalMin"`
	} `yaml:"periodicTrigger"`

	PolicyRecommendationController struct {
		MaxConcurrentReconciles int    `yaml:"maxConcurrentReconciles"`
		MinRequiredReplicas     int    `yaml:"minRequiredReplicas"`
		PolicyExpiryAge         string `yaml:"policyExpiryAge"`
	} `yaml:"policyRecommendationController"`

	PolicyRecommendationRegistrar struct {
		RequeueDelayMs     int    `yaml:"requeueDelayMs"`
		ExcludedNamespaces string `yaml:"excludedNamespaces"`
		IncludedNamespaces string `yaml:"includedNamespaces"`
	} `yaml:"policyRecommendationRegistrar"`

	CpuUtilizationBasedRecommender struct {
		MetricWindowInDays int `yaml:"metricWindowInDays"`
		StepSec            int `yaml:"stepSec"`
		MinTarget          int `yaml:"minTarget"`
		MaxTarget          int `yaml:"minTarget"`
	} `yaml:"cpuUtilizationBasedRecommender"`
	MetricIngestionTime      float64 `yaml:"metricIngestionTime"`
	MetricProbeTime          float64 `yaml:"metricProbeTime"`
	EnableMetricsTransformer *bool   `yaml:"enableMetricsTransformation"`
	EventCallIntegration     struct {
		EventCalendarAPIEndpoint        string `yaml:"eventCalendarAPIEndpoint"`
		NfrEventCompletedAPIEndpoint    string `yaml:"nfrEventCompletedAPIEndpoint"`
		NfrEventInProgressAPIEndpoint   string `yaml:"nfrEventInProgressAPIEndpoint"`
		EventFetchWindowInHours         int    `yaml:"eventFetchWindowInHours"`
		EventScaleUpBufferPeriodInHours int    `yaml:"eventScaleUpBufferPeriodInHours"`
	} `yaml:"eventCallIntegration"`
}

func main() {

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()
	logger := zap.New(zap.UseFlagOptions(&opts))
	ctrl.SetLogger(logger)

	config := Config{}
	configPath := os.Getenv("OTTOSCALR_CONFIG")
	if len(configPath) == 0 {
		configPath = "./local-config.yaml"
	}
	viper.SetConfigFile(configPath)

	err := viper.ReadInConfig()
	if err != nil {
		setupLog.Error(err, "Unable to read config file")
		os.Exit(1)
	}

	err = viper.Unmarshal(&config)
	if err != nil {
		setupLog.Error(err, "Unable to unmarshall config file")
		os.Exit(1)
	}
	logger.Info("Loaded config", "config", config)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     config.MetricBindAddress,
		Port:                   config.Port,
		HealthProbeBindAddress: config.HealthProbeBindAddress,
		LeaderElection:         config.EnableLeaderElection,
		LeaderElectionID:       config.LeaderElectionID,
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	agingPolicyTTL, err := time.ParseDuration(config.PolicyRecommendationController.PolicyExpiryAge)
	if err != nil {
		logger.Error(err, "Failed to parse policyExpiryAge. Defaulting.")
		agingPolicyTTL = 48 * time.Hour
	}

	scraper, err := metrics.NewPrometheusScraper(config.MetricsScraper.PrometheusUrl,
		time.Duration(config.MetricsScraper.QueryTimeoutSec)*time.Second,
		time.Duration(config.MetricsScraper.QuerySplitIntervalHr)*time.Hour,
		config.MetricIngestionTime,
		config.MetricProbeTime,
	)
	if err != nil {
		setupLog.Error(err, "unable to start prometheus scraper")
		os.Exit(1)
	}

	var eventIntegrations []integration.EventIntegration
	eventCalendarIntegration, err := integration.NewEventCalendarDataFetcher(config.EventCallIntegration.EventCalendarAPIEndpoint,
		time.Duration(config.EventCallIntegration.EventFetchWindowInHours)*time.Hour,
		time.Duration(config.EventCallIntegration.EventScaleUpBufferPeriodInHours)*time.Hour, logger)

	if err != nil {
		setupLog.Error(err, "unable to start event calendar data fetcher")
		os.Exit(1)
	}

	nfrEventIntegration, err := integration.NewNFREventDataFetcher(config.EventCallIntegration.NfrEventCompletedAPIEndpoint,
		config.EventCallIntegration.NfrEventInProgressAPIEndpoint,
		time.Duration(config.EventCallIntegration.EventFetchWindowInHours)*time.Hour,
		time.Duration(config.EventCallIntegration.EventScaleUpBufferPeriodInHours)*time.Hour, logger)

	if err != nil {
		setupLog.Error(err, "unable to start nfr event data fetcher")
		os.Exit(1)
	}

	eventIntegrations = append(eventIntegrations, eventCalendarIntegration, nfrEventIntegration)

	var metricsTransformer []metrics.MetricsTransformer

	if *config.EnableMetricsTransformer == true {
		outlierInterpolatorTransformer, err := transformer.NewOutlierInterpolatorTransformer(eventIntegrations, logger)
		if err != nil {
			setupLog.Error(err, "unable to start metrics transformer")
			os.Exit(1)
		}

		metricsTransformer = append(metricsTransformer, outlierInterpolatorTransformer)
	}

	cpuUtilizationBasedRecommender := reco.NewCpuUtilizationBasedRecommender(mgr.GetClient(),
		config.BreachMonitor.CpuRedLine,
		time.Duration(config.CpuUtilizationBasedRecommender.MetricWindowInDays)*24*time.Hour,
		scraper,
		metricsTransformer,
		time.Duration(config.CpuUtilizationBasedRecommender.StepSec)*time.Second,
		config.CpuUtilizationBasedRecommender.MinTarget,
		config.CpuUtilizationBasedRecommender.MaxTarget,
		logger)

	breachAnalyzer, err := reco.NewBreachAnalyzer(mgr.GetClient(), scraper, config.BreachMonitor.CpuRedLine, time.Duration(config.BreachMonitor.StepSec)*time.Second)
	if err != nil {
		setupLog.Error(err, "unable to initialize breach analyzer")
		os.Exit(1)
	}

	policyRecoReconciler, err := controller.NewPolicyRecommendationReconciler(mgr.GetClient(),
		mgr.GetScheme(), mgr.GetEventRecorderFor(controller.PolicyRecoWorkflowCtrlName),
		config.PolicyRecommendationController.MaxConcurrentReconciles, config.PolicyRecommendationController.MinRequiredReplicas, cpuUtilizationBasedRecommender, reco.NewDefaultPolicyIterator(mgr.GetClient()), reco.NewAgingPolicyIterator(mgr.GetClient(), agingPolicyTTL), breachAnalyzer)
	if err != nil {
		setupLog.Error(err, "Unable to initialize policy reco reconciler")
		os.Exit(1)
	}

	if err = policyRecoReconciler.
		SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PolicyRecommendation")
		os.Exit(1)
	}
	triggerHandler := trigger.NewK8sTriggerHandler(mgr.GetClient(), logger)
	triggerHandler.Start()

	monitorManager := trigger.NewPolicyRecommendationMonitorManager(mgr.GetClient(),
		mgr.GetEventRecorderFor(trigger.BreachStatusManager),
		scraper,
		time.Duration(config.PeriodicTrigger.PollingIntervalMin)*time.Minute,
		time.Duration(config.BreachMonitor.PollingIntervalSec)*time.Second,
		triggerHandler.QueueForExecution,
		config.BreachMonitor.StepSec,
		config.BreachMonitor.CpuRedLine,
		logger)

	excludedNamespaces := parseNamespaces(config.PolicyRecommendationRegistrar.ExcludedNamespaces)
	includedNamespaces := parseNamespaces(config.PolicyRecommendationRegistrar.IncludedNamespaces)

	policyStore := policy.NewPolicyStore(mgr.GetClient())
	if err = controller.NewPolicyRecommendationRegistrar(mgr.GetClient(),
		mgr.GetScheme(),
		config.PolicyRecommendationRegistrar.RequeueDelayMs,
		monitorManager,
		policyStore, excludedNamespaces, includedNamespaces).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller",
			"controller", "PolicyRecommendationRegistration")
		os.Exit(1)
	}

	if err = controller.NewPolicyWatcher(mgr.GetClient(),
		mgr.GetScheme(),
		triggerHandler.QueueAllForExecution).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Policy")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &kedaapi.ScaledObject{}, scaledTargetName, func(obj client.Object) []string {
		scaledObject := obj.(*kedaapi.ScaledObject)
		if scaledObject.Spec.ScaleTargetRef.Name == "" {
			return nil
		}
		return []string{scaledObject.Spec.ScaleTargetRef.Name}
	}); err != nil {
		setupLog.Error(err, "unable to index scaledobject")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}

	// Create a channel to listen for OS signals
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		monitorManager.Shutdown()
		eventCalendarIntegration.Cancel()
		os.Exit(0)
	}()
}

func parseNamespaces(namespaces string) []string {
	if namespaces == "" {
		return nil
	}
	splitNamespaces := strings.Split(namespaces, ",")
	var namespaceList []string
	for _, namespace := range splitNamespaces {
		namespaceList = append(namespaceList, strings.TrimSpace(namespace))
	}
	return namespaceList
}
