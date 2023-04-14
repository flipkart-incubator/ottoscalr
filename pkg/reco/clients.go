package reco

import (
	"context"
	"errors"
	argorolloutsv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	argorollouts "github.com/argoproj/argo-rollouts/pkg/client/clientset/versioned"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sync"
)

// Maintain a cache for workloads ie. Deployments and Argo Rollouts to enable clients to fetch the data from the cache
// instead of interfacing with the K8s API server

var once sync.Once
var wmc *WorkloadMetaClient

type WorkloadMetaClient struct {
	informerFactory  informers.SharedInformerFactory
	clientset        kubernetes.Interface
	rolloutClientset argorollouts.Interface
	isCached         bool
}

func NewWorkloadMetaClient() (*WorkloadMetaClient, error) {
	once.Do(func() {
		var err error
		wmc, err = newWorkloadMetaClient()
		if err != nil {
			// log the error
			return
		}
	})

	if wmc == nil {
		return nil, errors.New("WorkloadMetaClient is not initialized.")
	}
	return wmc, nil
}

func newWorkloadMetaClient() (*WorkloadMetaClient, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	rolloutsclientset, err1 := argorollouts.NewForConfig(config)
	if err != nil {
		return nil, err1
	}
	return &WorkloadMetaClient{
		informerFactory:  nil,
		clientset:        clientset,
		rolloutClientset: rolloutsclientset,
		isCached:         false,
	}, nil
}

func NewCachedWorkloadMetaClient() (*WorkloadMetaClient, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	//TODO: init sharedinformerfactory

	return &WorkloadMetaClient{
		informerFactory: nil,
		clientset:       clientset,
		isCached:        true,
	}, nil
}

func (wc *WorkloadMetaClient) GetDeployment(name, namespace string) (*appsv1.Deployment, error) {
	if wc.isCached {
		// TODO: return from cache
		return nil, nil
	} else {
		// make an API call to the server
		deployment, err := wc.clientset.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return deployment, nil
	}
}

func (wc *WorkloadMetaClient) GetRollout(name, namespace string) (*argorolloutsv1alpha1.Rollout, error) {
	if wc.isCached {
		// TODO: return from cache
		return nil, nil
	} else {
		// make an API call to the server
		rollouts, err := wc.rolloutClientset.ArgoprojV1alpha1().Rollouts(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return rollouts, nil
	}
}

func isDeployment(wm WorkloadMeta) bool {
	if wm.Kind == "Deployment" && wm.APIVersion == "" {
		return true
	}
	return false
}

func isRollout(wm WorkloadMeta) bool {
	if wm.Kind == "Rollout" {
		return true
	}
	return false
}
