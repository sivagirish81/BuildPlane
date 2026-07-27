package main

import (
	"flag"
	"net/http"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	buildplanev1alpha1 "github.com/sivagirish/buildplane/services/operator/api/v1alpha1"
	"github.com/sivagirish/buildplane/services/operator/internal/controller"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(buildplanev1alpha1.AddToScheme(scheme))
}

func main() {
	var metricsAddr string
	var probeAddr string
	var leaderElection bool
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "address for metrics")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "address for health probes")
	flag.BoolVar(&leaderElection, "leader-elect", false, "enable leader election")
	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	managerOptions := ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         leaderElection,
		LeaderElectionID:       "buildplane-operator.buildplane.io",
	}
	if watchNamespace := os.Getenv("WATCH_NAMESPACE"); watchNamespace != "" {
		managerOptions.Cache.DefaultNamespaces = map[string]cache.Config{
			watchNamespace: {},
		}
	}

	manager, err := ctrl.NewManager(ctrl.GetConfigOrDie(), managerOptions)
	if err != nil {
		ctrl.Log.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err := (&controller.BuildPlaneRuntimeReconciler{
		Client: manager.GetClient(),
	}).SetupWithManager(manager); err != nil {
		ctrl.Log.Error(err, "unable to create BuildPlaneRuntime controller")
		os.Exit(1)
	}

	if err := manager.AddHealthzCheck("healthz", func(_ *http.Request) error { return nil }); err != nil {
		ctrl.Log.Error(err, "unable to add health check")
		os.Exit(1)
	}
	if err := manager.AddReadyzCheck("readyz", func(_ *http.Request) error { return nil }); err != nil {
		ctrl.Log.Error(err, "unable to add readiness check")
		os.Exit(1)
	}

	ctrl.Log.Info("starting buildplane operator")
	if err := manager.Start(ctrl.SetupSignalHandler()); err != nil {
		ctrl.Log.Error(err, "operator stopped")
		os.Exit(1)
	}
}
