package manager

import (
	"flag"
	"net/http"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/kong/go-kong/kong"

	"github.com/kong/kubernetes-ingress-controller/pkg/sendconfig"
	konghqcomv1 "github.com/kong/kubernetes-ingress-controller/railgun/apis/configuration/v1"
	"github.com/kong/kubernetes-ingress-controller/railgun/apis/configuration/v1alpha1"
	configurationv1alpha1 "github.com/kong/kubernetes-ingress-controller/railgun/apis/configuration/v1alpha1"
	"github.com/kong/kubernetes-ingress-controller/railgun/controllers"
	kongctrl "github.com/kong/kubernetes-ingress-controller/railgun/controllers/configuration"
	//+kubebuilder:scaffold:imports
)

var (
	// KongURL is the url at which the admin API of the running proxy can be reached by controllers.
	KongURL string

	// Kubeconfig is an override available for path to the kubeconfig wanted for the cluster the controller manager should operate on.
	Kubeconfig string

	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(konghqcomv1.AddToScheme(scheme))
	utilruntime.Must(configurationv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func Run() {
	// environmental overrides
	// TODO: we might want to change how this works in the future, rather than just assuming the default ns
	if v := os.Getenv(controllers.CtrlNamespaceEnv); v == "" {
		os.Setenv(controllers.CtrlNamespaceEnv, controllers.DefaultNamespace)
	}

	// kong specific flags which can be programmatically overridden
	flag.StringVar(&KongURL, "kong-url", "http://localhost:8001", "The URL where the Kong Admin API for the proxy can be reached.")

	// controller-manager configuration overrides
	var filterTag string
	var concurrency int
	var secretName string
	var secretNamespace string
	flag.StringVar(&filterTag, "kong-filter-tag", "managed-by-railgun", "TODO")
	flag.IntVar(&concurrency, "kong-concurrency", 10, "TODO")
	flag.StringVar(&secretName, "secret-name", "kong-config", "TODO")
	flag.StringVar(&secretNamespace, "secret-namespace", controllers.DefaultNamespace, "TODO")

	// other kong specific flags with defaults
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	// logging options
	opts := zap.Options{
		Development: true, // FIXME - environment awareness needed
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	setupLog.Info("finding cluster configuration")
	var cfg *rest.Config
	var err error
	if Kubeconfig != "" {
		cfg, err = clientcmd.RESTConfigFromKubeConfig([]byte{})
	} else {
		cfg, err = ctrl.GetConfig()
	}
	if err != nil {
		setupLog.Error(err, "unable to determine kubeconfig")
		os.Exit(1)
	}

	setupLog.Info("configuring the controller manager")
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "5b374a9e.konghq.com",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	setupLog.Info("generating a kong client to communicate with the proxy's Admin API")
	kongClient, err := kong.NewClient(&KongURL, http.DefaultClient)
	if err != nil {
		setupLog.Error(err, "unable to create kongClient")
		os.Exit(1)
	}

	/* TODO: re-enable once fixed
	if err = (&kongctrl.KongIngressReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("KongIngress"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "KongIngress")
		os.Exit(1)
	}
	if err = (&kongctrl.KongClusterPluginReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("KongClusterPlugin"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "KongClusterPlugin")
		os.Exit(1)
	}
	if err = (&kongctrl.KongPluginReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("KongPlugin"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "KongPlugin")
		os.Exit(1)
	}
	if err = (&kongctrl.KongConsumerReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("KongConsumer"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "KongConsumer")
		os.Exit(1)
	}
	*/

	if err = (&kongctrl.SecretReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("Secret"),
		Scheme: mgr.GetScheme(),
		Params: kongctrl.SecretReconcilerParams{
			WatchName:      secretName,
			WatchNamespace: secretNamespace,
			KongConfig: sendconfig.Kong{
				URL:         KongURL,
				FilterTags:  []string{filterTag},
				Concurrency: concurrency,
				Client:      kongClient,
			},
		},
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Secret")
		os.Exit(1)
	}

	// TODO - we've got a couple places in here and below where we "short circuit" controllers if the relevant API isn't available.
	// This is convenient for testing, but maintainers should reconsider this before we release KIC 2.0.
	// SEE: https://github.com/Kong/kubernetes-ingress-controller/issues/1101
	if err := kongctrl.SetupIngressControllers(mgr); err != nil {
		setupLog.Error(err, "unable to create controllers", "controllers", "Ingress")
		os.Exit(1)
	}

	// TODO - similar to above, we're short circuiting here. It's convenient, but let's discuss if this is what we want ultimately.
	// SEE: https://github.com/Kong/kubernetes-ingress-controller/issues/1101
	udpIngressAvailable, err := kongctrl.IsAPIAvailable(mgr, &v1alpha1.UDPIngress{})
	if !udpIngressAvailable {
		setupLog.Error(err, "API configuration.konghq.com/v1alpha1/UDPIngress is not available, skipping controller")
	} else {
		if err = (&kongctrl.KongV1UDPIngressReconciler{
			Client: mgr.GetClient(),
			Log:    ctrl.Log.WithName("controllers").WithName("UDPIngress"),
			Scheme: mgr.GetScheme(),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "UDPIngress")
			os.Exit(1)
		}
	}

	//+kubebuilder:scaffold:builder

	setupLog.Info("enabling health checks")
	if err := mgr.AddHealthzCheck("health", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("check", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
