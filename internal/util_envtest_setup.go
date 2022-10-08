package internal

import (
	"context"
	"github.com/arikkfir/kude-controller/internal/v1alpha1"
	zapr "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"strings"
	"testing"
	"time"
)

const (
	k8sVersion = "1.25.0"
)

type Setupable interface {
	SetupWithManager(mgr ctrl.Manager) error
}

type testWriter struct {
	T *testing.T
}

func (tw *testWriter) Write(p []byte) (n int, err error) {
	tw.T.Helper()
	tw.T.Logf("%s", p)
	return len(p), nil
}

func setupTestEnv(t *testing.T, reconcilers ...Setupable) (k8sClient client.Client, k8sMgr manager.Manager, cleanup func()) {
	t.Helper()

	logLevel := zapr.NewAtomicLevelAt(zapr.InfoLevel)
	opts := zap.Options{
		Development: true,
		Level:       &logLevel,
		DestWriter:  &testWriter{T: t},
		TimeEncoder: zapcore.TimeEncoderOfLayout(time.StampMilli),
	}
	logger := zap.New(zap.UseFlagOptions(&opts))
	ctx, cancel := context.WithCancel(context.Background())

	var (
		k8sConfig       *rest.Config
		testEnv         *envtest.Environment
		workDir         string
		binaryAssetsDir string
	)

	t.Log("Obtaining current work directory")
	if wd, err := os.Getwd(); err != nil {
		t.Errorf("failed to get working directory: %v", err)
	} else {
		workDir = filepath.Clean(filepath.Join(wd, ".."))
		binaryAssetsDir = filepath.Join(workDir, "bin", "k8s", strings.Join([]string{k8sVersion, runtime.GOOS, runtime.GOARCH}, "-"))
	}

	t.Log("Bootstrapping test environment")
	testEnv = &envtest.Environment{
		AttachControlPlaneOutput: false,
		BinaryAssetsDirectory:    binaryAssetsDir,
		CRDDirectoryPaths:        []string{filepath.Join(workDir, "config", "crd")},
		ErrorIfCRDPathMissing:    true,
	}

	t.Log("Starting test environment")
	if cfg, err := testEnv.Start(); err != nil {
		t.Errorf("failed to start test environment: %v", err)
	} else {
		k8sConfig = cfg
	}
	cleanup = func() {
		t.Log("Stopping test environment")
		cancel()
		if err := testEnv.Stop(); err != nil {
			t.Errorf("failed to stop test environment: %v", err)
		}
	}

	t.Log("Registering Kubernetes resource types scheme")
	if err := v1alpha1.AddToScheme(scheme.Scheme); err != nil {
		cleanup()
		t.Errorf("failed to register Kubernetes resource types scheme: %v", err)
	}

	t.Log("Creating Kubernetes client")
	if c, err := client.New(k8sConfig, client.Options{Scheme: scheme.Scheme}); err != nil {
		cleanup()
		t.Errorf("failed to create Kubernetes client: %v", err)
	} else {
		k8sClient = c
	}

	t.Log("Creating controller manager")
	mgrOptions := ctrl.Options{
		Logger:             logger,
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0", // On MacOS if not set to "0", firewall will request approval on every run
	}
	if mgr, err := ctrl.NewManager(k8sConfig, mgrOptions); err != nil {
		cleanup()
		t.Errorf("failed to create controller manager: %v", err)
	} else {
		k8sMgr = mgr
	}

	t.Log("Setting up reconcilers")
	for _, reconciler := range reconcilers {
		if err := reconciler.SetupWithManager(k8sMgr); err != nil {
			cleanup()
			t.Errorf("failed to setup reconciler '%s': %v", reflect.TypeOf(reconciler).Elem().Name(), err)
		}
	}

	go func() {
		t.Log("Starting controller manager")
		if err := k8sMgr.Start(ctx); err != nil {
			t.Errorf("failed to start controller manager: %v", err)
		}
	}()

	return
}
