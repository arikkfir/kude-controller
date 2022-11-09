package main

import (
	"context"
	"flag"
	"fmt"
	"go.uber.org/zap/zapcore"
	"k8s.io/client-go/rest"
	"os"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/arikkfir/kude-controller/internal"
	"github.com/arikkfir/kude-controller/internal/v1alpha1"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func run(k8sConfig *rest.Config, metricsAddr string, enableLeaderElection bool, probeAddr string, opts zap.Options, ctx context.Context) error {

	// Apply logger
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Create the manager
	mgr, err := ctrl.NewManager(k8sConfig, ctrl.Options{
		Scheme:                        scheme,
		MetricsBindAddress:            metricsAddr,
		Port:                          9443,
		HealthProbeBindAddress:        probeAddr,
		LeaderElection:                enableLeaderElection,
		LeaderElectionID:              "7e5314ff.kude.kfirs.com",
		LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		return fmt.Errorf("unable to start manager: %w", err)
	}

	// Setup GitRepository reconciler
	if err := (&internal.GitRepositoryReconciler{WorkDir: "/data"}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller '%s': %w", "GitRepository", err)
	}
	if err := (&internal.KubectlBundleReconciler{}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller '%s': %w", "KubectlBundle", err)
	}
	//+kubebuilder:scaffold:builder

	// Add health probes
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up health check: %w", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up ready check: %w", err)
	}

	// Run!
	setupLog.Info("Starting manager")
	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("problem running manager: %w", err)
	}

	return nil
}

func main() {

	// Flags
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
		TimeEncoder: zapcore.TimeEncoderOfLayout(time.StampMilli),
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	// Run
	if err := run(ctrl.GetConfigOrDie(), metricsAddr, enableLeaderElection, probeAddr, opts, ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "Operator failed")
		os.Exit(1)
	}
}
