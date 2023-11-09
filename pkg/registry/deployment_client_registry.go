package registry

import (
	"fmt"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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

func (cr *DeploymentClientRegistryBuilder) WithCustomDeploymentClient(client ObjectClient) *DeploymentClientRegistryBuilder {
	cr.Clients = append(cr.Clients, client)
	return cr
}
