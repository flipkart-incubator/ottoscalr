# Installation Guide

Ottoscalr can be installed in any kubernetes cluster which meets the prerequisites. Since its functionality relies upon the availability of historical data for the recommendations to be generated, a promql compliant time series database is needed.

## Prerequisites

- Kubernetes cluster (1.24+)
- kube state metrics (2.8+)
- Promql compliant metrics source (with historical workload utilization metrics)


## Building the image

The following section outlines the docker image build process for the ottoscalr. 
The following commands builds the image for the platform linux/amd64. Change the TARGETOS and TARGETARCH if it is required to be built for other platform.
```console
$ TARGETOS=linux;TARGETARCH=amd64;make docker-build docker-push IMG={repository}:{tag}
```

## Deploying the ottoscalr

Ottoscalr can be deployed via this [helm chart](https://github.com/flipkart-incubator/ottoscalr/tree/main/charts/ottoscalr). 

This chart bootstraps ottoscalr on a Kubernetes cluster using the Helm package manager. As part of that, it will install all the required Custom Resource Definitions (CRD).

### Installing the Chart

To install the chart with the release name `ottoscalr`:

```console
$ kubectl create namespace ottoscalr
$ helm install ottoscalr <path-of-the-helm-chart> --namespace ottoscalr -f <path-of-the-helm-chart>/values.yaml
```

### Upgrading the Chart

To update the already installed chart `ottoscalr`:

```console
$ helm upgrade ottoscalr <path-of-the-helm-chart> --namespace ottoscalr -f <path-of-the-helm-chart>/values.yaml
```

### Uninstalling the Chart

To uninstall/delete the `ottoscalr` Helm chart:

```console
helm uninstall ottoscalr
```
The command removes all the Kubernetes components associated with the chart and deletes the release.



### Configuration

The following table lists the configurable parameters of the KEDA chart and
their default values.

#### Ottoscalr Deployment

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `image.repository` | string | `""` | Image name of ottoscalr deployment |
| `image.tag` | string | `""` | Image tag of ottoscalr deployment |
| `replicaCount` | int | `1` | Capability to configure the number of replicas for ottoscalr operator. While you can run more replicas of our operator, only one operator instance will be the leader and serving traffic. You can run multiple replicas, but they will not improve the performance of ottoscalr, it could only reduce downtime during a failover. |
| `resources` | object | `{"limits":{"cpu":1,"memory":"2Gi"},"requests":{"cpu":"1","memory":"2Gi"}}` | Manage [resource request & limits] of ottoscalr operator pod |

#### Operations

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `ottoscalr.config.metricsScraper.prometheusUrl` | string | `""` | URL where prometheus for the kubernetes cluster is running.   |
| `ottoscalr.config.metricsScraper.queryTimeoutSec` | int | `300` | Time in seconds within which the response for any query should be served by the prometheus  |
| `ottoscalr.config.metricsScraper.querySplitIntervalHr` | int | `24` | The shortest period in hour for which data will be fetched from prometheus |
| `ottoscalr.config.policyRecommendationController.maxConcurrentReconciles` | int | `1` | Maximum number of concurrent Reconciles which can be run. |
| `ottoscalr.config.policyRecommendationController.minRequiredReplicas` | int | `3` | The hpa.spec.minReplicas  recommended by the controller will not have replicas minimum than this.  |
| `ottoscalr.config.policyRecommendationController.policyExpiryAge` | string | `3h` | Target Recommendation will be reached in multiple iterations and through different policies. This is the time after which a policy expires and next policy in the list can be applied. |
| `ottoscalr.config.breachMonitor.pollingIntervalSec` | int | `300` | Time in seconds to check for any breaches happened during this time |
| `ottoscalr.config.breachMonitor.cpuRedLine` | float | `0.75` | Conisder it a breach if CPU utilization goes above this number. |
| `ottoscalr.config.breachMonitor.concurrentExecutions` | int | `50` | Concurrent execution of breach queries. Configure it according to the number of concurrent connections that can be handled safely by the prometheus  |
| `ottoscalr.config.periodicTrigger.pollingIntervalMin` | int |  `180` | Duration in minutes to periodically trigger the recommendation generation |
| `ottoscalr.config.policyRecommendationRegistrar.excludedNamespaces` | string | `""` | Comma separated namespaces where ottoscalr will not generate recommendations. Example: "namespace1,namespace2,namespace3" |
| `ottoscalr.config.cpuUtilizationBasedRecommender.metricWindowInDays` | int | `28` | Number of days for which cpu utilization metrics should be fetched for generating the recommendation. |
| `ottoscalr.config.cpuUtilizationBasedRecommender.minTarget` | int | `5` | The cpu utilization threshold recommended will not be less than this. |
| `ottoscalr.config.cpuUtilizationBasedRecommender.maxTarget` | int | `60` | The cpu utilization threshold recommended will not be more than this. |
| `ottoscalr.config.cpuUtilizationBasedRecommender.metricsPercentageThreshold` | int | `25` | Minimum percentage of metrics to be present to generate any recommendation. Example: if `ottoscalr.config.cpuUtilizationBasedRecommender.metricWindowInDays` is 28 days and `ottoscalr.config.cpuUtilizationBasedRecommender.metricsPercentageThreshold` is 25%, then atleast 7 days worth of data should be present. |
| `ottoscalr.config.hpaEnforcer.maxConcurrentReconciles` | int | `1` | Maximum number of concurrent Reconciles which can be run by hpaenforcement controller |
| `ottoscalr.config.hpaEnforcer.excludedNamespaces` | string | `""` | Comma separated namespaces where ottoscalr will not create HPAs. Example: "namespace1,namespace2,namespace3" |
| `ottoscalr.config.hpaEnforcer.includedNamespaces` | string | `""` | If provided, ottoscalr will only create HPAs for these namespaces. If it is empty, it will include all except for the excluded ones. Example: "namespace1,namespace2,namespace3" |
| `ottoscalr.config.hpaEnforcer.whitelistMode` | bool | `true` | hpa controller will act only on deployments having `ottoscalr.io/enable-hpa-enforcement: true` annotation and create hpa for them. If false, hpaEnforcer runs on blacklistMode where it will create hpas for every workload except for ones having `ottoscalr.io/disable-hpa-enforcement: true`. It is recommended to run with whitelistMode as true. |
| `ottoscalr.config.hpaEnforcer.isDryRun` | bool | `false` | If true, hpa controller will not create any HPAs.   |
| `ottoscalr.config.hpaEnforcer.enableMetricsTransformer` | bool | `true` | This metrics transformer can be used to interpolate any known period of data that should not be used for generating recommendation.  |
| `ottoscalr.config.autoscalerClient.scaledObjectConfigs.enableScaledObject` | bool | `false` | Flag whether to use KEDA ScaledObjects or HPA for autoscaling. KEDA needs to be deployed on your cluster for enabling it. |
| `ottoscalr.config.autoscalerClient.hpaConfigs.hpaAPIVersion` | string | `"v2"` | Set this if using HPA for autoscaling. By default, `autoscaling/v2` api is supported. If you wish to use `autoscaling/v1` api for HPA, change this to `"v1"`. |

