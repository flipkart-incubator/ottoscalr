package breach

import (
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Monitor struct {
	k8sClient   client.Client
	log         logr.Logger
	ownerObject client.Object
}

func NewMonitor(k8sClient client.Client, log logr.Logger, ownerObject client.Object) Monitor {
	bm := Monitor{k8sClient: k8sClient, log: log, ownerObject: ownerObject}
	return bm
}
func (b *Monitor) MonitorBreaches() error {
	return nil
}
