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
	"flag"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	argov1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/flipkart-incubator/ottoscalr/pkg/autoscaler"
	"github.com/flipkart-incubator/ottoscalr/pkg/controller"
	"github.com/flipkart-incubator/ottoscalr/pkg/integration"
	"github.com/flipkart-incubator/ottoscalr/pkg/metrics"
	"github.com/flipkart-incubator/ottoscalr/pkg/policy"
	"github.com/flipkart-incubator/ottoscalr/pkg/reco"
	"github.com/flipkart-incubator/ottoscalr/pkg/registry"
	"github.com/flipkart-incubator/ottoscalr/pkg/transformer"
	"github.com/flipkart-incubator/ottoscalr/pkg/trigger"
	kedaapi "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/spf13/viper"

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
	scheme           = runtime.NewScheme()
	setupLog         = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(ottoscaleriov1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

type Config struct {
	Port                   int    `yaml:"port"`
	MetricBindAddress      string `yaml:"metricBindAddress"`
	PprofBindAddress       string `yaml:"pprofBindAddress"`
	HealthProbeBindAddress string `yaml:"healthProbeBindAddress"`
	EnableLeaderElection   bool   `yaml:"enableLeaderElection"`
	LeaderElectionID       string `yaml:"leaderElectionID"`
	MetricsScraper         struct {
		PrometheusUrl        string `yaml:"prometheusUrl"`
		QueryTimeoutSec      int    `yaml:"queryTimeoutSec"`
		QuerySplitIntervalHr int    `yaml:"querySplitIntervalHr"`
	} `yaml:"metricsScraper"`

	BreachMonitor struct {
		PollingIntervalSec   int     `yaml:"pollingIntervalSec"`
		CpuRedLine           float64 `yaml:"cpuRedLine"`
		StepSec              int     `yaml:"stepSec"`
		ConcurrentExecutions int     `yaml:"concurrentExecutions"`
	} `yaml:"breachMonitor"`

	PeriodicTrigger struct {
		PollingIntervalMin int `yaml:"pollingIntervalMin"`
	} `yaml:"periodicTrigger"`

	PolicyRecommendationController struct {
		MaxConcurrentReconciles int    `yaml:"maxConcurrentReconciles"`
		MinRequiredReplicas     int    `yaml:"minRequiredReplicas"`
		PolicyExpiryAge         string `yaml:"policyExpiryAge"`
	} `yaml:"policyRecommendationController"`

	HPAEnforcer struct {
		MaxConcurrentReconciles int    `yaml:"maxConcurrentReconciles"`
		ExcludedNamespaces      string `yaml:"excludedNamespaces"`
		IncludedNamespaces      string `yaml:"includedNamespaces"`
		IsDryRun                *bool  `yaml:"isDryRun"`
		WhitelistMode           *bool  `yaml:"whitelistMode"`
		MinRequiredReplicas     int    `yaml:"minRequiredReplicas"`
	} `yaml:"hpaEnforcer"`

	PolicyRecommendationRegistrar struct {
		RequeueDelayMs     int    `yaml:"requeueDelayMs"`
		ExcludedNamespaces string `yaml:"excludedNamespaces"`
		IncludedNamespaces string `yaml:"includedNamespaces"`
	} `yaml:"policyRecommendationRegistrar"`

	CpuUtilizationBasedRecommender struct {
		MetricWindowInDays         int `yaml:"metricWindowInDays"`
		StepSec                    int `yaml:"stepSec"`
		MinTarget                  int `yaml:"minTarget"`
		MaxTarget                  int `yaml:"minTarget"`
		MetricsPercentageThreshold int `yaml:"metricsPercentageThreshold"`
	} `yaml:"cpuUtilizationBasedRecommender"`
	MetricIngestionTime      float64 `yaml:"metricIngestionTime"`
	MetricProbeTime          float64 `yaml:"metricProbeTime"`
	EnableMetricsTransformer *bool   `yaml:"enableMetricsTransformation"`
	EventCallIntegration     struct {
		CustomEventDataConfigMapName    string `yaml:"customEventDataConfigMapName"`
	} `yaml:"eventCallIntegration"`
	AutoscalerClient struct {
		ScaledObjectConfigs struct {
			EnableScaledObject    *bool `yaml:"enableScaledObject"`
			EnableEventAutoscaler *bool `yaml:"enableEventAutoscaler"`
		} `yaml:"scaledObjectConfigs"`
		HpaConfigs struct {
			HpaAPIVersion string `yaml:"hpaAPIVersion"`
		} `yaml:"hpaConfigs"`
	} `yaml:"autoscalerClient"`
	EnableArgoRolloutsSupport *bool `yaml:"enableArgoRolloutsSupport"`
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
		PprofBindAddress:       config.PprofBindAddress,
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

	prometheusInstances := parseCommaSeparatedValues(config.MetricsScraper.PrometheusUrl)

	scraper, err := metrics.NewPrometheusScraper(prometheusInstances,
		time.Duration(config.MetricsScraper.QueryTimeoutSec)*time.Second,
		time.Duration(config.MetricsScraper.QuerySplitIntervalHr)*time.Hour,
		config.MetricIngestionTime,
		config.MetricProbeTime,
		logger,
	)
	if err != nil {
		setupLog.Error(err, "unable to start prometheus scraper")
		os.Exit(1)
	}

	var eventIntegrations []integration.EventIntegration
	customEventIntegration, err := integration.NewCustomEventDataFetcher(mgr.GetClient(),
		os.Getenv("DEPLOYMENT_NAMESPACE"), config.EventCallIntegration.CustomEventDataConfigMapName, logger)

	if err != nil {
		setupLog.Error(err, "unable to start custom event data fetcher")
		os.Exit(1)
	}

	eventIntegrations = append(eventIntegrations, customEventIntegration)
	var metricsTransformer []metrics.MetricsTransformer

	if *config.EnableMetricsTransformer {
		outlierInterpolatorTransformer, err := transformer.NewOutlierInterpolatorTransformer(eventIntegrations, logger)
		if err != nil {
			setupLog.Error(err, "unable to start metrics transformer")
			os.Exit(1)
		}

		metricsTransformer = append(metricsTransformer, outlierInterpolatorTransformer)
	}
	deploymentClientRegistryBuilder := registry.NewDeploymentClientRegistryBuilder().
		WithK8sClient(mgr.GetClient()).
		WithCustomDeploymentClient(registry.NewDeploymentClient(mgr.GetClient()))

	if *config.EnableArgoRolloutsSupport {
		utilruntime.Must(argov1alpha1.AddToScheme(scheme))
		//+kubebuilder:scaffold:scheme
		deploymentClientRegistryBuilder = deploymentClientRegistryBuilder.WithCustomDeploymentClient(registry.NewRolloutClient(mgr.GetClient()))
	}
	deploymentClientRegistry := deploymentClientRegistryBuilder.Build()
	cpuUtilizationBasedRecommender := reco.NewCpuUtilizationBasedRecommender(mgr.GetClient(),
		config.BreachMonitor.CpuRedLine,
		time.Duration(config.CpuUtilizationBasedRecommender.MetricWindowInDays)*24*time.Hour,
		scraper,
		metricsTransformer,
		time.Duration(config.CpuUtilizationBasedRecommender.StepSec)*time.Second,
		config.CpuUtilizationBasedRecommender.MinTarget,
		config.CpuUtilizationBasedRecommender.MaxTarget,
		config.CpuUtilizationBasedRecommender.MetricsPercentageThreshold,
		*deploymentClientRegistry,
		logger)

	breachAnalyzer, err := reco.NewBreachAnalyzer(mgr.GetClient(), scraper, config.BreachMonitor.CpuRedLine, time.Duration(config.BreachMonitor.StepSec)*time.Second)
	if err != nil {
		setupLog.Error(err, "unable to initialize breach analyzer")
		os.Exit(1)
	}

	policyStore := policy.NewPolicyStore(mgr.GetClient())

	policyRecoReconciler, err := controller.NewPolicyRecommendationReconciler(mgr.GetClient(),
		mgr.GetScheme(), mgr.GetEventRecorderFor(controller.PolicyRecoWorkflowCtrlName),
		config.PolicyRecommendationController.MaxConcurrentReconciles, config.PolicyRecommendationController.MinRequiredReplicas, cpuUtilizationBasedRecommender, policyStore, reco.NewDefaultPolicyIterator(mgr.GetClient()), reco.NewAgingPolicyIterator(mgr.GetClient(), agingPolicyTTL), breachAnalyzer)
	if err != nil {
		setupLog.Error(err, "Unable to initialize policy reco reconciler")
		os.Exit(1)
	}

	if err = policyRecoReconciler.
		SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PolicyRecommendation")
		os.Exit(1)
	}

	deploymentTriggerReconciler := controller.NewDeploymentTriggerController(mgr.GetClient(), mgr.GetScheme(), *deploymentClientRegistry)
	if err = deploymentTriggerReconciler.
		SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DeploymentController")
		os.Exit(1)
	}

	triggerHandler := trigger.NewK8sTriggerHandler(mgr.GetClient(), logger)
	triggerHandler.Start()

	monitorManager := trigger.NewPolicyRecommendationMonitorManager(mgr.GetClient(),
		mgr.GetEventRecorderFor(trigger.BreachStatusManager),
		scraper,
		time.Duration(config.PeriodicTrigger.PollingIntervalMin)*time.Minute,
		time.Duration(config.BreachMonitor.PollingIntervalSec)*time.Second,
		config.BreachMonitor.ConcurrentExecutions,
		triggerHandler.QueueForExecution,
		config.BreachMonitor.StepSec,
		config.BreachMonitor.CpuRedLine,
		logger)

	excludedNamespaces := parseCommaSeparatedValues(config.PolicyRecommendationRegistrar.ExcludedNamespaces)
	includedNamespaces := parseCommaSeparatedValues(config.PolicyRecommendationRegistrar.IncludedNamespaces)

	hpaEnforcerExcludedNamespaces := parseCommaSeparatedValues(config.HPAEnforcer.ExcludedNamespaces)
	hpaEnforcerIncludedNamespaces := parseCommaSeparatedValues(config.HPAEnforcer.IncludedNamespaces)

	var autoscalerClient autoscaler.AutoscalerClient
	if *config.AutoscalerClient.ScaledObjectConfigs.EnableScaledObject {
		utilruntime.Must(kedaapi.AddToScheme(scheme))
		//+kubebuilder:scaffold:scheme

		autoscalerClient = autoscaler.NewScaledobjectClient(mgr.GetClient(),
			config.AutoscalerClient.ScaledObjectConfigs.EnableEventAutoscaler)
	} else {
		if config.AutoscalerClient.HpaConfigs.HpaAPIVersion == "v2" {
			autoscalerClient = autoscaler.NewHPAClientV2(mgr.GetClient())
		} else {
			autoscalerClient = autoscaler.NewHPAClient(mgr.GetClient())
		}
	}
	hpaEnforcementController, err := controller.NewHPAEnforcementController(mgr.GetClient(),
		mgr.GetScheme(), *deploymentClientRegistry, mgr.GetEventRecorderFor(controller.HPAEnforcementCtrlName),
		config.HPAEnforcer.MaxConcurrentReconciles, config.HPAEnforcer.IsDryRun, &hpaEnforcerExcludedNamespaces, &hpaEnforcerIncludedNamespaces, config.HPAEnforcer.WhitelistMode, config.HPAEnforcer.MinRequiredReplicas, autoscalerClient)
	if err != nil {
		setupLog.Error(err, "Unable to initialize HPA enforcement controller")
		os.Exit(1)
	}

	if err = hpaEnforcementController.
		SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HPAEnforcementController")
		os.Exit(1)
	}

	if err = controller.NewPolicyRecommendationRegistrar(mgr.GetClient(),
		mgr.GetScheme(),
		config.PolicyRecommendationRegistrar.RequeueDelayMs,
		monitorManager,
		policyStore, *deploymentClientRegistry, excludedNamespaces, includedNamespaces).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller",
			"controller", "PolicyRecommendationRegistration")
		os.Exit(1)
	}

	if err = controller.NewPolicyWatcher(mgr.GetClient(),
		mgr.GetScheme(),
		triggerHandler.QueueAllForExecution,
		triggerHandler.QueueForExecution).SetupWithManager(mgr); err != nil {
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
		os.Exit(0)
	}()
}

func parseCommaSeparatedValues(givenConfig string) []string {
	if givenConfig == "" {
		return nil
	}
	splitValues := strings.Split(givenConfig, ",")
	var parsedValues []string
	for _, namespace := range splitValues {
		parsedValues = append(parsedValues, strings.TrimSpace(namespace))
	}
	return parsedValues
}
