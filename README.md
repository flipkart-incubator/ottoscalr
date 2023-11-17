# OttoScalr

OttoScalr is an autoscaling solution for kubernetes workloads, that works  by continuously monitoring workload resource utilization patterns to proactively configure and tune the horizontal pod autoscaler policies (HPA, ScaledObjects) autonomously ensuring optimal resource utilization and cost efficiency. It is designed to work with various kubernetes workload types viz. deployment and argo rollouts. With its pluggable design to incorporate different flavors of policy recommenders, it can be extended to customize the policy generation algorithms. 


## Features

- **Autoscalers**: OttoScalr doesn't autoscale the workloads by itself, but works by creating/managing the HPAs and KEDA ScaledObjects which influences how and when workloads are scaled.
- **Controllers**: Ottoscalr is made up of a bunch of controllers that perform a bunch of tasks in ensuring that the workloads are configured with the right HPA policies at all times.
- **Workloads**: Support for stateless workloads of kinds -- Deployments and Argo Rollouts (optional, can be toggled during the deployment)  
- **Pluggable Recommenders**: Ottoscalr provides an extensible framework for pluggable recommenders which will generate recommendations of autoscaler configurations which are then enforced on the workload.
- **Graded Policies**: Since there's no one size fits autoscaling policy for a workload. Ottoscalr works with a set of graded policies and takes workload through these policies and doesn't go past the ideal policy recommended by the recommender.
- **Integration with promql compliant metric sources**: Works with any promql compliant metrics source for gathering historical workload resource utilization metrics.

## Getting Started

To get started with OttoScalr, you'll need to have a Kubernetes cluster up and running. You'll also need to have access to the command line on a machine with `kubectl` installed and configured to interact with your cluster.

## Installation

The installation process involves setting up the necessary Kubernetes resources for OttoScalr to run. Detailed instructions will be provided in the installation guide.

## Usage

Once OttoScalr is installed and running, you can start configuring it to monitor your applications and make scaling decisions. Detailed instructions on how to do this will be provided in the usage guide.

## Contributing

Contributions to OttoScalr are welcome! Please read our contributing guide to learn about our development process, how to propose bugfixes and improvements, and how to build and test your changes to OttoScalr.

## License

OttoScalr is licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for the full license text.

## Support

If you encounter any problems or have any questions about OttoScalr, please open an issue on our GitHub repository.

