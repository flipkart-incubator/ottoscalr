package metrics

import "fmt"

type QueryComponent struct {
	metric string
	labels map[string]string
}

type CPUUtilizationQuery struct {
	queries map[string]*QueryComponent
}

type CPUUtilizationBreachQuery struct {
	queries map[string]*QueryComponent
}

type PodReadyLatencyQuery struct {
	queries map[string]*QueryComponent
}

func NewCPUUtilizationQuery() *CPUUtilizationQuery {
	cpuUtilizationMetric := "node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate"
	podOwnerMetric := "namespace_workload_pod:kube_pod_owner:relabel"

	queries := make(map[string]*QueryComponent)
	queries["cpu_utilization_metric"] = NewQueryComponentBuilder().WithMetric(cpuUtilizationMetric).Build()
	queries["pod_owner_metric"] = NewQueryComponentBuilder().WithMetric(podOwnerMetric).Build()
	return &CPUUtilizationQuery{
		queries: queries,
	}
}

func NewCPUUtilizationBreachQuery() *CPUUtilizationBreachQuery {
	cpuUtilizationMetric := "node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate"
	podOwnerMetric := "namespace_workload_pod:kube_pod_owner:relabel"
	resourceLimitMetric := "cluster:namespace:pod_cpu:active:kube_pod_container_resource_limits"
	readyReplicasMetric := "kube_replicaset_status_ready_replicas"
	replicaSetOwnerMetric := "kube_replicaset_owner"
	hpaMaxReplicasMetric := "kube_horizontalpodautoscaler_spec_max_replicas"
	hpaOwnerInfoMetric := "kube_horizontalpodautoscaler_info"

	queries := make(map[string]*QueryComponent)
	queries["cpu_utilization_metric"] = NewQueryComponentBuilder().WithMetric(cpuUtilizationMetric).Build()
	queries["pod_owner_metric"] = NewQueryComponentBuilder().WithMetric(podOwnerMetric).Build()
	queries["resource_limit_metric"] = NewQueryComponentBuilder().WithMetric(resourceLimitMetric).Build()
	queries["ready_replicas_metric"] = NewQueryComponentBuilder().WithMetric(readyReplicasMetric).Build()
	queries["replicaset_owner_metric"] = NewQueryComponentBuilder().WithMetric(replicaSetOwnerMetric).Build()
	queries["hpa_max_replicas_metric"] = NewQueryComponentBuilder().WithMetric(hpaMaxReplicasMetric).Build()
	queries["hpa_owner_info_metric"] = NewQueryComponentBuilder().WithMetric(hpaOwnerInfoMetric).Build()
	return &CPUUtilizationBreachQuery{
		queries: queries,
	}
}

func NewPodReadyLatencyQuery() *PodReadyLatencyQuery {
	podCreatedTimeMetric := "kube_pod_created"
	podReadyTimeMetric := "alm_kube_pod_ready_time"
	podOwnerMetric := "namespace_workload_pod:kube_pod_owner:relabel"
	queries := make(map[string]*QueryComponent)
	queries["pod_created_time_metric"] = NewQueryComponentBuilder().WithMetric(podCreatedTimeMetric).Build()
	queries["pod_ready_time_metric"] = NewQueryComponentBuilder().WithMetric(podReadyTimeMetric).Build()
	queries["pod_owner_metric"] = NewQueryComponentBuilder().WithMetric(podOwnerMetric).Build()
	return &PodReadyLatencyQuery{
		queries: queries,
	}
}

func (q *CPUUtilizationQuery) BuildCompositeQuery(namespace, workload string) string {
	q.queries["cpu_utilization_metric"].AddLabel("namespace", namespace)
	q.queries["pod_owner_metric"].AddLabel("namespace", namespace)
	q.queries["pod_owner_metric"].AddLabel("workload", workload)
	q.queries["pod_owner_metric"].AddLabel("workload_type", "deployment")

	return fmt.Sprintf("sum(%s * on (namespace,pod) group_left(workload, workload_type)"+
		"%s) by(namespace, workload, workload_type)",
		q.queries["cpu_utilization_metric"].Render(),
		q.queries["pod_owner_metric"].Render())
}

func (q *CPUUtilizationBreachQuery) BuildCompositeQuery(namespace, workload, workloadType string, redLineUtilization float64) string {
	q.queries["cpu_utilization_metric"].AddLabel("namespace", namespace)
	q.queries["pod_owner_metric"].AddLabel("namespace", namespace)
	q.queries["pod_owner_metric"].AddLabel("workload", workload)
	q.queries["pod_owner_metric"].AddLabel("workload_type", "deployment")
	q.queries["resource_limit_metric"].AddLabel("namespace", namespace)
	q.queries["ready_replicas_metric"].AddLabel("namespace", namespace)
	q.queries["replicaset_owner_metric"].AddLabel("namespace", namespace)
	q.queries["replicaset_owner_metric"].AddLabel("owner_kind", workloadType)
	q.queries["replicaset_owner_metric"].AddLabel("owner_name", workload)
	q.queries["hpa_max_replicas_metric"].AddLabel("namespace", namespace)
	q.queries["hpa_owner_info_metric"].AddLabel("namespace", namespace)
	q.queries["hpa_owner_info_metric"].AddLabel("scaletargetref_kind", workloadType)
	q.queries["hpa_owner_info_metric"].AddLabel("scaletargetref_name", workload)

	return fmt.Sprintf("(sum(%s * on(namespace,pod) group_left(workload, workload_type) "+
		"%s) by (namespace, workload, workload_type)/ on (namespace, workload, workload_type) "+
		"group_left sum(%s * on(namespace,pod) group_left(workload, workload_type)"+
		"%s) by (namespace, workload, workload_type) > %.2f) and on(namespace, workload) "+
		"label_replace(sum(%s * on(replicaset)"+
		" group_left(namespace, owner_kind, owner_name) %s) by"+
		" (namespace, owner_kind, owner_name) < on(namespace, owner_kind, owner_name) "+
		"(%s * on(namespace, horizontalpodautoscaler) "+
		"group_left(owner_kind, owner_name) label_replace(label_replace(%s,\"owner_kind\", \"$1\", "+
		"\"scaletargetref_kind\", \"(.*)\"), \"owner_name\", \"$1\", \"scaletargetref_name\", \"(.*)\")),"+
		"\"workload\", \"$1\", \"owner_name\", \"(.*)\")",
		q.queries["cpu_utilization_metric"].Render(),
		q.queries["pod_owner_metric"].Render(),
		q.queries["resource_limit_metric"].Render(),
		q.queries["pod_owner_metric"].Render(),
		redLineUtilization,
		q.queries["ready_replicas_metric"].Render(),
		q.queries["replicaset_owner_metric"].Render(),
		q.queries["hpa_max_replicas_metric"].Render(),
		q.queries["hpa_owner_info_metric"].Render())

}

func (q *PodReadyLatencyQuery) BuildCompositeQuery(namespace, workload string) string {
	q.queries["pod_created_time_metric"].AddLabel("namespace", namespace)
	q.queries["pod_ready_time_metric"].AddLabel("namespace", namespace)
	q.queries["pod_owner_metric"].AddLabel("namespace", namespace)
	q.queries["pod_owner_metric"].AddLabel("workload", workload)
	q.queries["pod_owner_metric"].AddLabel("workload_type", "deployment")

	return fmt.Sprintf("quantile(0.5,(%s - on (namespace,pod) (%s)) "+
		"* on (namespace,pod) group_left(workload, workload_type)"+
		"(%s))",
		q.queries["pod_ready_time_metric"].Render(),
		q.queries["pod_created_time_metric"].Render(),
		q.queries["pod_owner_metric"].Render())
}

func ValidateQuery(query string) bool {
	//validate if p8s query is syntactically correct
	stack := make([]rune, 0)
	for _, char := range query {
		switch char {
		case '(', '{':
			stack = append(stack, char)
		case ')':
			if len(stack) == 0 || stack[len(stack)-1] != '(' {
				return false
			}
			stack = stack[:len(stack)-1]
		case '}':
			if len(stack) == 0 || stack[len(stack)-1] != '{' {
				return false
			}
			stack = stack[:len(stack)-1]
		}
	}
	return len(stack) == 0

}

type QueryComponentBuilder QueryComponent

func NewQueryComponentBuilder() *QueryComponentBuilder {
	return &QueryComponentBuilder{}
}

func (qb *QueryComponentBuilder) WithMetric(metric string) *QueryComponentBuilder {
	qb.metric = metric
	return qb
}

func (qb *QueryComponentBuilder) WithLabels(labels map[string]string) *QueryComponentBuilder {
	qb.labels = labels
	return qb
}

func (qb *QueryComponentBuilder) Build() *QueryComponent {
	return &QueryComponent{
		metric: qb.metric,
		labels: qb.labels,
	}
}

func (qc *QueryComponent) AddLabel(key string, value string) {
	if qc.labels == nil {
		qc.labels = make(map[string]string)
	}
	qc.labels[key] = value
}

func (qc *QueryComponent) Render() string {
	if qc.labels == nil {
		return qc.metric
	}
	return fmt.Sprintf("%s{%s}", qc.metric, renderLabels(qc.labels))
}

func renderLabels(labels map[string]string) string {
	var labelStr string
	for key, value := range labels {
		labelStr += fmt.Sprintf("%s=\"%s\",", key, value)
	}
	return labelStr[:len(labelStr)-1]
}
