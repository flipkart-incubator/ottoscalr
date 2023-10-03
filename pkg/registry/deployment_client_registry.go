package registry

import (
	"fmt"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var gvkClientMap map[string]func(k8sClient client.Client) ObjectClient

func init() {
	gvkClientMap = make(map[string]func(k8sClient client.Client) ObjectClient)
	gvkClientMap[DeploymentGVK.String()] = AddDeploymentClient
	gvkClientMap[RolloutGVK.String()] = AddRolloutClient
}

func AddDeploymentClient(k8sClient client.Client) ObjectClient {
	return &DeploymentClient{
		k8sClient: k8sClient,
		gvk:       DeploymentGVK,
	}
}

func AddRolloutClient(k8sClient client.Client) ObjectClient {
	return &RolloutClient{
		k8sClient: k8sClient,
		gvk:       RolloutGVK,
	}
}

type ObjectClient interface {
	GetObject(namespace string, name string) (client.Object, error)
	GetObjectType() client.Object
	GetKind() string
	GetMaxReplicaFromAnnotation(namespace string, name string) (int, error)
	GetContainerResourceLimits(namespace string, name string) (float64, error)
	GetReplicaCount(namespace string, name string) (int, error)
	Scale(namespace string, name string, replicas int32) error
}

type DeploymentClientRegistry struct {
	k8sClient client.Client
	Clients   []ObjectClient
}

type DeploymentClientRegistryBuilder DeploymentClientRegistry

func (cr *DeploymentClientRegistryBuilder) Build() *DeploymentClientRegistry {
	return &DeploymentClientRegistry{
		Clients: cr.Clients,
	}
}

func (cr *DeploymentClientRegistryBuilder) WithK8sClient(k8sClient client.Client) *DeploymentClientRegistryBuilder {
	cr.k8sClient = k8sClient
	return cr
}

func NewDeploymentClientRegistryBuilder() *DeploymentClientRegistryBuilder {
	return &DeploymentClientRegistryBuilder{}
}

func (cr *DeploymentClientRegistry) GetObjectClient(objectKind string) (ObjectClient, error) {
	for _, cl := range cr.Clients {
		if cl.GetKind() == objectKind {
			return cl, nil
		}
	}
	return nil, fmt.Errorf("object kind not found in client registry")
}

func (cr *DeploymentClientRegistryBuilder) WithCustomDeploymentClient(gvk schema.GroupVersionKind) *DeploymentClientRegistryBuilder {

	if addCustomClient, ok := gvkClientMap[gvk.String()]; ok {
		object := addCustomClient(cr.k8sClient)
		if cr.Clients == nil {
			var clientList []ObjectClient
			clientList = append(clientList, object)
			cr.Clients = clientList
			return cr
		}
		cr.Clients = append(cr.Clients, object)
	}
	return cr
}
