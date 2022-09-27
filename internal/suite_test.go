package internal

import (
	"context"
	"github.com/arikkfir/kude-controller/internal/v1alpha1"
	"os"
	"path/filepath"
	"runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	cfg          *rest.Config
	k8sClient    client.Client
	testEnv      *envtest.Environment
	k8sCtx       context.Context
	k8sCancelCtx context.CancelFunc
)

const (
	k8sVersion = "1.25.0"
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Controller Suite", []Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func() {
	log.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	k8sCtx, k8sCancelCtx = context.WithCancel(context.TODO())
	var err error
	var workdir string

	By("bootstrapping test environment")
	workdir, err = os.Getwd()
	Expect(err).To(BeNil())
	workdir = filepath.Clean(filepath.Join(workdir, ".."))
	testEnv = &envtest.Environment{
		AttachControlPlaneOutput: false,
		BinaryAssetsDirectory:    filepath.Join(workdir, "bin", "k8s", strings.Join([]string{k8sVersion, runtime.GOOS, runtime.GOARCH}, "-")),
		CRDDirectoryPaths:        []string{filepath.Join(workdir, "config", "crd")},
		ErrorIfCRDPathMissing:    true,
	}

	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = v1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	err = (&GitRepositoryReconciler{}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(k8sCtx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()

}, 60)

var _ = AfterSuite(func() {
	k8sCancelCtx()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
