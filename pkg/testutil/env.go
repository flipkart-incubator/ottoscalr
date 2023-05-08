package testutil

import (
	"context"
	"k8s.io/client-go/rest"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sync"
)

type singletonEnvironment struct {
	Cfg     *rest.Config
	TestEnv *envtest.Environment
	Ctx     context.Context
	Cancel  context.CancelFunc
}

var (
	instance *singletonEnvironment
	once     sync.Once
)

func getInstance() *singletonEnvironment {
	once.Do(func() {
		instance = &singletonEnvironment{}
		instance.TestEnv = &envtest.Environment{
			CRDDirectoryPaths: []string{filepath.Join("..", "..", "config", "crd", "bases"),
				filepath.Join("..", "testconfig")},
			ErrorIfCRDPathMissing: true,
		}
		cfg, err := instance.TestEnv.Start()
		if err != nil {
			panic(err)
		}
		instance.Cfg = cfg
		instance.Ctx, instance.Cancel = context.WithCancel(context.TODO())
	})
	return instance
}

func SetupEnvironment() (*rest.Config, context.Context, context.CancelFunc) {
	envInstance := getInstance()
	return envInstance.Cfg, envInstance.Ctx, envInstance.Cancel
}

func TeardownEnvironment() error {
	envInstance := getInstance()
	if envInstance.TestEnv != nil {
		err := envInstance.TestEnv.Stop()
		if err != nil {
			return err
		}
	}
	return nil
}
