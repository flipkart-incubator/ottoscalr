package metrics

import "fmt"

type QueryComponent struct {
	metric    string
	labelKeys []string
}

type CompositeQuery struct {
	queries map[string]*QueryComponent
}

type CompositeQueryBuilder CompositeQuery

func NewCompositeQueryBuilder() *CompositeQueryBuilder {
	return &CompositeQueryBuilder{}
}

func (qb *CompositeQueryBuilder) WithQuery(name string, query *QueryComponent) *CompositeQueryBuilder {
	if qb.queries == nil {
		qb.queries = make(map[string]*QueryComponent)
	}
	qb.queries[name] = query
	return qb
}

func (qb *CompositeQueryBuilder) Build() *CompositeQuery {
	return &CompositeQuery{
		queries: qb.queries,
	}
}

type CPUUtilizationQuery CompositeQuery
type CPUUtilizationBreachQuery CompositeQuery
type PodReadyLatencyQuery CompositeQuery

func (qb *CPUUtilizationQuery) Render(labels map[string]string) string {

	return fmt.Sprintf("sum(%s * on (namespace,pod) group_left(workload, workload_type)"+
		"%s) by(namespace, workload, workload_type)",
		qb.queries["cpu_utilization_metric"].Render(labels),
		qb.queries["pod_owner_metric"].Render(labels))
}

func (qb *CPUUtilizationBreachQuery) Render(redLineUtilization float64, labels map[string]string) string {

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
		qb.queries["cpu_utilization_metric"].Render(labels),
		qb.queries["pod_owner_metric"].Render(labels),
		qb.queries["resource_limit_metric"].Render(labels),
		qb.queries["pod_owner_metric"].Render(labels),
		redLineUtilization,
		qb.queries["ready_replicas_metric"].Render(labels),
		qb.queries["replicaset_owner_metric"].Render(labels),
		qb.queries["hpa_max_replicas_metric"].Render(labels),
		qb.queries["hpa_owner_info_metric"].Render(labels))

}

func (qb *PodReadyLatencyQuery) Render(labels map[string]string) string {

	return fmt.Sprintf("quantile(0.5,(%s - on (namespace,pod) (%s)) "+
		"* on (namespace,pod) group_left(workload, workload_type)"+
		"(%s))",
		qb.queries["pod_ready_time_metric"].Render(labels),
		qb.queries["pod_created_time_metric"].Render(labels),
		qb.queries["pod_owner_metric"].Render(labels))
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

func (qb *QueryComponentBuilder) WithLabelKeys(labelKeys []string) *QueryComponentBuilder {
	qb.labelKeys = labelKeys
	return qb
}

func (qb *QueryComponentBuilder) Build() *QueryComponent {
	return &QueryComponent{
		metric:    qb.metric,
		labelKeys: qb.labelKeys,
	}
}

func (qc *QueryComponent) Render(m map[string]string) string {

	labels := make(map[string]string)
	for _, labelKey := range qc.labelKeys {
		if labelValue, ok := m[labelKey]; ok {
			labels[labelKey] = labelValue
		}
	}
	return fmt.Sprintf("%s%s", qc.metric, renderLabels(labels))
}

func renderLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	labelStr := "{"
	for key, value := range labels {
		labelStr += fmt.Sprintf("%s=\"%s\",", key, value)
	}
	labelStr = labelStr[:len(labelStr)-1]
	labelStr += "}"
	return labelStr
}
