package autoscaler

import (
	"context"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	falseBool = false
	trueBool  = true
)

type AutoscalerClient interface {
	CreateOrUpdateAutoscaler(ctx context.Context, workload client.Object, labels map[string]string, max int32, min int32, targetCPUUtilization int32) (string, error)
	DeleteAutoscaler(ctx context.Context, obj client.Object) error
	GetType() client.Object
	GetList(ctx context.Context, labelSelector labels.Selector, namespace string, fieldSelector fields.Selector) ([]client.Object, error)
	GetMaxReplicaCount(obj client.Object) int32
	GetScaleTargetName(obj client.Object) string
	GetName() string
}
