# OttoScalr

OttoScalr is an autoscaling solution for kubernetes workloads, that works  by continuously monitoring workload resource utilization patterns to proactively configure and tune the horizontal pod autoscaler policies (HPA, ScaledObjects) autonomously ensuring optimal resource utilization and cost efficiency. This alleviates developers concerns of continuously having to tune the HPAs in accordance with the changing traffic/load patterns and performance profile of the workload. It is designed to work with various kubernetes workload types viz. deployment and argo rollouts. With its pluggable design to incorporate different flavors of policy recommenders, it can be extended to customize the policy generation algorithms to suit the needs of the workload/cluster administrators. 


## Features

- **Autoscalers**: OttoScalr doesn't autoscale the workloads by itself, but works by creating/managing the HPAs and [KEDA ScaledObjects](https://keda.sh/docs/1.5/concepts/scaling-deployments/) which influences how and when workloads are scaled.
- **Controllers**: Ottoscalr is made up of a bunch of controllers that perform a variety of tasks in ensuring that the workloads are configured with the right HPA policy at all times.
- **Workloads**: Support for stateless workloads of kinds -- Deployments and [Argo Rollouts](https://argoproj.github.io/argo-rollouts/) (optional, can be toggled during the deployment)  
- **Pluggable Recommenders**: Ottoscalr provides an extensible framework for pluggable recommenders which will generate recommendations of autoscaler configurations which are then enforced on the workload.
- **Graded Policies**: Since there's no one size fits autoscaling policy for a workload. Ottoscalr works with a set of graded policies and takes workload through these policies and doesn't go past the ideal policy recommended by the recommender.
- **Integration with promql compliant metric sources**: Works with any promql compliant metrics source for gathering historical workload resource utilization metrics.

### What Ottoscalr doesn't do

- Automatic performance tuning of applications running within a pod.
- Improvement of pod bootstrap latencies.
- Optimization of pod level resource consumption
- Vertical scaling of pods to optimize average utilization.
- Autoscaling for stateful workloads.


## Getting Started

To get started with OttoScalr, you'll need to have a Kubernetes cluster up and running where the ottoscalr can be installed. Please go through the ottoscalr wiki to understand more about its workings.

# Installation

Ottoscalr is easy to be installed and configured in any kubernetes cluster. The installation process involves setting up the necessary Kubernetes resources for OttoScalr to run. Detailed instructions will be provided in the [installation and configuration guide](INSTALLATION.md).

## Contributing

Contributions to OttoScalr are welcome! Please read our contributing guide to learn about our development process, how to propose bugfixes and improvements, and how to build and test your changes to OttoScalr.

## Support

If you encounter any problems or have any questions about OttoScalr, please open an issue on our GitHub repository.


    
