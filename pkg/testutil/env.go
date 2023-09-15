package testutil

import (
	"context"
	"k8s.io/client-go/rest"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sync"
)

type TestEnvironment struct {
	Cfg     *rest.Config
	TestEnv *envtest.Environment
	Ctx     context.Context
	Cancel  context.CancelFunc
}

var (
	envInstance *TestEnvironment
	once        sync.Once
)

func getSingletonInstance() *TestEnvironment {
	once.Do(func() {
		envInstance = &TestEnvironment{}
		envInstance.TestEnv = &envtest.Environment{
			CRDDirectoryPaths: []string{filepath.Join("..", "..", "config", "crd", "bases"),
				filepath.Join("..", "testconfig")},
			ErrorIfCRDPathMissing: true,
		}
		cfg, err := envInstance.TestEnv.Start()
		if err != nil {
			panic(err)
		}
		envInstance.Cfg = cfg
		envInstance.Ctx, envInstance.Cancel = context.WithCancel(context.TODO())
	})
	return envInstance
}

func SetupSingletonEnvironment() (*rest.Config, context.Context, context.CancelFunc) {
	envInstance := getSingletonInstance()
	return envInstance.Cfg, envInstance.Ctx, envInstance.Cancel
}

func TeardownSingletonEnvironment() error {
	envInstance := getSingletonInstance()
	if envInstance.TestEnv != nil {
		err := envInstance.TestEnv.Stop()
		if err != nil {
			return err
		}
	}
	return nil
}

func SetupEnvironment() *TestEnvironment {
	testEnvInstance := &TestEnvironment{}
	testEnvInstance.TestEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "config", "crd", "bases"),
			filepath.Join("..", "testconfig")},
		ErrorIfCRDPathMissing: true,
	}
	cfg, err := testEnvInstance.TestEnv.Start()
	if err != nil {
		panic(err)
	}
	testEnvInstance.Cfg = cfg
	testEnvInstance.Ctx, testEnvInstance.Cancel = context.WithCancel(context.TODO())
	return testEnvInstance
}

func TeardownEnvironment(envInstance *TestEnvironment) error {
	if envInstance.TestEnv != nil {
		err := envInstance.TestEnv.Stop()
		if err != nil {
			return err
		}
	}
	return nil
}
