#### Ottoscalr metrics

| Metric name | Metric type | Description | Labels/tags |
|-----------|------|---------|-------------|
| `ottoscalr_policyreco_current_policy_max` | gauge | Current Max replica count to be applied to the HPA | `policyreco`=&lt;policyrecommendation-name&gt; <br> `namespace`=&lt;policyrecommendation-namespace&gt;  |
| `ottoscalr_policyreco_current_policy_min` | gauge | Current Min replica count to be applied to the HPA | `policyreco`=&lt;policyrecommendation-name&gt; <br> `namespace`=&lt;policyrecommendation-namespace&gt;  |
| `ottoscalr_policyreco_current_policy_utilization` | gauge | Current CPU utilization threshold to be applied to the HPA | `policyreco`=&lt;policyrecommendation-name&gt; <br> `namespace`=&lt;policyrecommendation-namespace&gt;  |
| `ottoscalr_policyreco_target_policy_max` | gauge | Max replica count recommended to be applied to the HPA.  | `policyreco`=&lt;policyrecommendation-name&gt; <br> `namespace`=&lt;policyrecommendation-namespace&gt;  |
| `ottoscalr_policyreco_target_policy_min` | gauge | Min replica count recommended to be applied to the HPA.  | `policyreco`=&lt;policyrecommendation-name&gt; <br> `namespace`=&lt;policyrecommendation-namespace&gt;  |
| `ottoscalr_policyreco_target_policy_max` | gauge | CPU utilization threshold recommended to be applied to the HPA.  | `policyreco`=&lt;policyrecommendation-name&gt; <br> `namespace`=&lt;policyrecommendation-namespace&gt;  |
| `ottoscalr_policyreco_workload_info` | gauge | Information about the policyrecommendation  | `policyreco`=&lt;policyrecommendation-name&gt; <br> `namespace`=&lt;policyrecommendation-namespace&gt; <br> `workload`=&lt;policyrecommendation-workload&gt; <br> `workloadKind`=&lt;workloadType(Deployment,Rollout)&gt; |
| `ottoscalr_policyreco_reconciler_conditions` | gauge | Metric for checking the status of different conditions of `.policyrecommendation.status` | `policyreco`=&lt;policyrecommendation-name&gt; <br> `namespace`=&lt;policyrecommendation-namespace&gt; <br> `status`=&lt;True,False&gt; <br> `type`=&lt;RecoTaskProgress,TargetRecoAchieved&gt; |
| `ottoscalr_policyreco_reconciler_task_progress_reason` | gauge | Metric for checking the reason for condition type `RecoTaskProgress`. | `policyreco`=&lt;policyrecommendation-name&gt; <br> `namespace`=&lt;policyrecommendation-namespace&gt; <br> `reason`=&lt;RecoTaskErrored,RecoTaskRecommendationGenerated; |
| `ottoscalr_policyreco_reconciled_count` | counter | Number of times a policyrecommendation has been reconciled | `policyreco`=&lt;policyrecommendation-name&gt; <br> `namespace`=&lt;policyrecommendation-namespace&gt; |
| `ottoscalr_policyreco_reconciler_errored_count` | counter | Number of times a policyrecommendation's reconciliation has errored | `policyreco`=&lt;policyrecommendation-name&gt; <br> `namespace`=&lt;policyrecommendation-namespace&gt; |
| `ottoscalr_policyreco_reconciler_targetreco_slo_days` | histogram | Time taken for a policy reco to achieve the target recommendation in days | `policyreco`=&lt;policyrecommendation-name&gt; <br> `namespace`=&lt;policyrecommendation-namespace&gt; |
| `ottoscalr_minimum_percentage_of_datapoints_present` | gauge | If minimum percentage of datapoints is present to generate recommendation. | `workload`=&lt;deployment-name&gt; <br> `namespace`=&lt;policyrecommendation-namespace&gt; <br> `reason`=&lt;RecoTaskErrored,RecoTaskRecommendationGenerated; |
| `ottoscalr_p8s_query_error_count` | counter | Error counter for a query made to prometheus | `query`=&lt;query-type&gt; <br> `p8sinstance`=&lt;prometheusinstance-name&gt; |
| `ottoscalr_p8s_query_success_count` | counter | Success counter for a query made to prometheus | `query`=&lt;query-type&gt; <br> `p8sinstance`=&lt;prometheusinstance-name&gt; |
| `ottoscalr_p8s_concurrent_queries` | gauge | Number of concurrent p8s api calls for a query | `query`=&lt;query-type&gt; <br> `p8sinstance`=&lt;prometheusinstance-name&gt; |
| `ottoscalr_datapoints_fetched_by_p8s_instance` | gauge | Number of datapoints fetched for a query for a workload from a prometheus instance | `query`=&lt;query-type&gt; <br> `p8sinstance`=&lt;prometheusinstance-name&gt; <br> `workload`=&lt;deployment-name&gt; <br> `namespace`=&lt;policyrecommendation-namespace&gt; |
| `ottoscalr_total_datapoints_fetched` | gauge | Total Number of datapoints fetched for a query for a workload after aggregating from all the prometheus instances | `query`=&lt;query-type&gt; <br> `workload`=&lt;deployment-name&gt; <br> `namespace`=&lt;policyrecommendation-namespace&gt; |
| `ottoscalr_prometheus_scraper_query_latency` | histogram | Time to execute prometheus scraper query in seconds | `query`=&lt;query-type&gt; <br> `p8sinstance`=&lt;prometheusinstance-name&gt; <br> `workload`=&lt;deployment-name&gt; <br> `namespace`=&lt;policyrecommendation-namespace&gt; | 
| `ottoscalr_get_avg_cpu_utilization_query_latency_seconds` | histogram | Total Time to execute utilization datapoint query in seconds | `policyreco`=&lt;policyrecommendation-name&gt; <br> `namespace`=&lt;policyrecommendation-namespace&gt; <br> `workload`=&lt;deployment-name&gt; <br> `workloadKind`=&lt;workloadType(Deployment,Rollout)&gt; |
| `ottoscalr_get_reco_generation_latency_seconds` | histogram | Total time to generate policyrecommendation for a workload once it's execution is started | `policyreco`=&lt;policyrecommendation-name&gt; <br> `namespace`=&lt;policyrecommendation-namespace&gt; <br> `workload`=&lt;deployment-name&gt; <br> `workloadKind`=&lt;workloadType(Deployment,Rollout)&gt; |
| `ottoscalr_breachmonitor_breached` | gauge | If a particular workload has breached the cpu redline or not | `policyreco`=&lt;policyrecommendation-name&gt; <br> `namespace`=&lt;policyrecommendation-namespace&gt; <br> `workload`=&lt;deployment-name&gt; <br> `workloadKind`=&lt;workloadType(Deployment,Rollout)&gt; |
| `ottoscalr_breachmonitor_execution_rate` | gauge | Rate of breachmonitor executions for the workloads | |
| `ottoscalr_concurrent_breachmonitor_executions` | counter | Number of concurrent breachmonitor executions for the workloads | |
| `ottoscalr_breachmonitor_mitigation_latency_seconds` | histogram | Time to mitigate breach in seconds for a workload | `policyreco`=&lt;policyrecommendation-name&gt; <br> `namespace`=&lt;policyrecommendation-namespace&gt; <br> `workload`=&lt;deployment-name&gt; <br> `workloadKind`=&lt;workloadType(Deployment,Rollout)&gt; |
| `ottoscalr_hpaenforcer_reconciled_count` | counter | Number of times a policyrecommendation has been reconciled by HPAEnforcer | `policyreco`=&lt;policyrecommendation-name&gt; <br> `namespace`=&lt;policyrecommendation-namespace&gt; |


#### Ottoscalr Controller Metrics

All the kubebuilder controller metrics as mentioned here:- https://book.kubebuilder.io/reference/metrics-reference are available for ottoscalr controllers. 
All the metrics should be prefixed with `ottoscalr_` .

Following are the names of controllers:-
<b>PolicyRecommendationRegistrar</b>,
<b>RecoWorkflowController</b>,
<b>HPAEnforcementController</b>,
<b>PolicyWatcher</b>,
<b>DeploymentTriggerController</b>







