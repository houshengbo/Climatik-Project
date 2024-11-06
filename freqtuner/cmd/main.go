package main

import (
	"flag"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	freqtunerv1alpha1 "github.com/Climatik-Project/Climatik-Project/freqtuner/api/v1alpha1"
	"github.com/Climatik-Project/Climatik-Project/freqtuner/controllers"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(freqtunerv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
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
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	if os.Getenv("DEBUG") == "true" {
		setupLog.Info("Debug mode enabled")
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	nodeName, err := os.Hostname()
	if err != nil {
		setupLog.Error(err, "failed to get hostname")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                server.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "freqtuner-leader-election",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	reconciler := &controllers.NodeFrequenciesReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		NodeName: nodeName,
	}

	if err = reconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "NodeFrequenciesReconciler")
		os.Exit(1)
	}

	// Start the manager and initialize CR in a goroutine
	ctx := ctrl.SetupSignalHandler()
	go func() {
		// Wait for cache to sync
		if ok := mgr.GetCache().WaitForCacheSync(ctx); !ok {
			setupLog.Error(nil, "failed to wait for caches to sync")
			return
		}
		setupLog.Info("Cache synced, initializing NodeFrequencies CR")
		if err := reconciler.InitializeNodeFrequenciesCRs(ctx); err != nil {
			setupLog.Error(err, "failed to initialize NodeFrequencies CR")
		}
	}()

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
