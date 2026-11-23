package main

import (
	"crypto/tls"
	"flag"
	"os"
	"strings"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kubeconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
	"github.com/PrefectHQ/prefect-operator/internal/controller"
	"github.com/PrefectHQ/prefect-operator/internal/portforward"
	"github.com/PrefectHQ/prefect-operator/internal/prefect"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(prefectiov1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var enableHTTP2 bool
	var defaultResyncInterval time.Duration
	var reconcileTimeout time.Duration
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.DurationVar(&defaultResyncInterval, "default-resync-interval", 5*time.Minute,
		"How often to re-check each Prefect resource against the Prefect API to correct "+
			"out-of-band drift, unless a resource sets its own spec.interval.")
	flag.DurationVar(&reconcileTimeout, "reconcile-timeout", 2*time.Minute,
		"How long one reconcile may run before its context is canceled and it is retried. "+
			"0 means no limit.")
	// Without --zap-* flags, logs are JSON at info level. `make run` passes
	// --zap-devel for console output at debug level.
	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.V(1).Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	tlsOpts := []func(*tls.Config){}
	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: tlsOpts,
	})

	mgrOpts := managerOptions(reconcileTimeout)
	mgrOpts.Metrics = metricsserver.Options{
		BindAddress: metricsAddr,
		TLSOpts:     tlsOpts,
	}
	mgrOpts.WebhookServer = webhookServer
	mgrOpts.HealthProbeBindAddress = probeAddr
	mgrOpts.LeaderElection = enableLeaderElection
	mgrOpts.LeaderElectionID = "fd9f6399.prefect.io"
	// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
	// when the Manager ends. This requires the binary to immediately end when the
	// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
	// speeds up voluntary leader transitions as the new leader don't have to wait
	// LeaseDuration time first.
	//
	// In the default scaffold provided, the program ends immediately after
	// the manager stops, so would be fine to enable this option. However,
	// if you are doing or is intended to do any operation such as perform cleanups
	// after the manager stops then its usage might be unsafe.
	mgrOpts.LeaderElectionReleaseOnCancel = true

	restConfig, kubectl, err := clusterConfig()
	if err != nil {
		setupLog.Error(err, "unable to load the kubeconfig")
		os.Exit(1)
	}
	if kubectl.Context != "" {
		setupLog.Info("port-forwards to in-cluster Prefect servers use this kubeconfig context", "context", kubectl.Context)
	}

	mgr, err := ctrl.NewManager(restConfig, mgrOpts)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	prefectClients := prefect.NewClientFactory(mgr.GetClient(), kubectl)
	recorder := mgr.GetEventRecorder("prefect-operator")

	if err = (&controller.PrefectServerReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PrefectServer")
		os.Exit(1)
	}

	if err = (&controller.PrefectWorkPoolReconciler{
		Client:                mgr.GetClient(),
		Scheme:                mgr.GetScheme(),
		NewPrefectClient:      prefectClients,
		DefaultResyncInterval: defaultResyncInterval,
		Recorder:              recorder,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PrefectWorkPool")
		os.Exit(1)
	}

	if err = (&controller.PrefectDeploymentReconciler{
		Client:                mgr.GetClient(),
		Scheme:                mgr.GetScheme(),
		NewPrefectClient:      prefectClients,
		DefaultResyncInterval: defaultResyncInterval,
		Recorder:              recorder,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PrefectDeployment")
		os.Exit(1)
	}

	if err = (&controller.PrefectAutomationReconciler{
		Client:                mgr.GetClient(),
		Scheme:                mgr.GetScheme(),
		NewPrefectClient:      prefectClients,
		DefaultResyncInterval: defaultResyncInterval,
		Recorder:              recorder,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PrefectAutomation")
		os.Exit(1)
	}

	if err = (&controller.PrefectWorkQueueReconciler{
		Client:                mgr.GetClient(),
		Scheme:                mgr.GetScheme(),
		NewPrefectClient:      prefectClients,
		DefaultResyncInterval: defaultResyncInterval,
		Recorder:              recorder,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PrefectWorkQueue")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}

	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

// clusterConfig returns the manager's REST config. In the cluster, the
// returned Kubectl is empty, because the operator reaches Prefect servers
// through their Services. Outside the cluster, it reads the kubeconfig's
// current context once and builds both the REST config and the Kubectl from
// that context. A later `kubectl config use-context` then changes neither, so
// port-forwards stay on the cluster that the operator watches. Both read the
// kubeconfig that the --kubeconfig flag names, or the default kubeconfig when
// the flag is empty.
func clusterConfig() (*rest.Config, portforward.Kubectl, error) {
	if _, err := rest.InClusterConfig(); err == nil {
		cfg, err := kubeconfig.GetConfig()
		return cfg, portforward.Kubectl{}, err
	}
	kubectl, err := portforward.PinCurrentContext(flag.Lookup(kubeconfig.KubeconfigFlagName).Value.String())
	if err != nil {
		return nil, portforward.Kubectl{}, err
	}
	cfg, err := kubeconfig.GetConfigWithContext(kubectl.Context)
	return cfg, kubectl, err
}

// managerOptions returns the manager options that control how the operator
// reads Kubernetes objects. The client reads the kinds in
// prefect.UncachedObjects from the API server, because the operator's RBAC
// grants get on them but not list or watch.
func managerOptions(reconcileTimeout time.Duration) ctrl.Options {
	return ctrl.Options{
		Scheme: scheme,
		Cache: cache.Options{
			DefaultNamespaces: getWatchNamespaces(),
		},
		Client: client.Options{
			Cache: &client.CacheOptions{DisableFor: prefect.UncachedObjects()},
		},
		Controller: config.Controller{
			ReconciliationTimeout: reconcileTimeout,
		},
	}
}

// getWatchNamespaces returns the namespace(s) the operator should be watching for changes.
//
// It checks the "WATCH_NAMESPACES" environment variable, which can contain either:
// - an empty string: ""
// - or a list of comma-separated namespaces to watch: "my-app,my-other-app"
func getWatchNamespaces() map[string]cache.Config {
	namespaces, found := os.LookupEnv("WATCH_NAMESPACES")
	if !found {
		setupLog.Info("WATCH_NAMESPACES not configured, watching all namespaces")

		return map[string]cache.Config{}
	}

	namespacesToWatch := make(map[string]cache.Config)
	for ns := range strings.SplitSeq(namespaces, ",") {
		namespacesToWatch[ns] = cache.Config{}
	}

	return namespacesToWatch
}
