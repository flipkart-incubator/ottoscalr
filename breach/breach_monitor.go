package breach

import (
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type BreachMonitor struct {
	k8sClient   client.Client
	log         logr.Logger
	ownerObject client.Object
}

func NewBreachMonitor(k8sClient client.Client, log logr.Logger, ownerObject client.Object) BreachMonitor {
	bm := BreachMonitor{k8sClient: k8sClient, log: log, ownerObject: ownerObject}
	return bm
}
func (b *BreachMonitor) MonitorBreaches() error {
	return nil
}
