package main

import (
	"flag"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog/v2"
	"totalsoft.ro/platform-controllers/internal/controllers"
	"totalsoft.ro/platform-controllers/internal/controllers/provisioning"
	"totalsoft.ro/platform-controllers/internal/controllers/provisioning/migration"
	"totalsoft.ro/platform-controllers/internal/controllers/provisioning/provisioners/pulumi"
	messaging "totalsoft.ro/platform-controllers/internal/messaging"
	"totalsoft.ro/platform-controllers/pkg/generated/clientset/versioned"
	"totalsoft.ro/platform-controllers/pkg/signals"
)

var (
	kubeConfig     *rest.Config
	kubeConfigPath string
)

func syncKlogFlags(klogFlags *flag.FlagSet) {
	// Sync the glog and klog flags.
	flag.CommandLine.VisitAll(func(f1 *flag.Flag) {
		f2 := klogFlags.Lookup(f1.Name)
		if f2 != nil {
			value := f1.Value.String()
			f2.Value.Set(value)
		}
	})
}

// InitFlags is for explicitly initializing the flags.
func InitFlags() {
	flag.Set("alsologtostderr", "true")
	if home := homedir.HomeDir(); home != "" {
		flag.StringVar(&kubeConfigPath, "kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		flag.StringVar(&kubeConfigPath, "kubeconfig", "", "absolute path to the kubeconfig file")
	}

	flag.Parse()
	klogFlags := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(klogFlags)
	syncKlogFlags(klogFlags)
}

// GetConfig gets a kubernetes rest config.
func getConfig() *rest.Config {
	if kubeConfig != nil {
		return kubeConfig
	}

	conf, err := rest.InClusterConfig()
	if err != nil {
		conf, err = clientcmd.BuildConfigFromFlags("", kubeConfigPath)
		if err != nil {
			klog.Fatalf("Error building provisioning clientset: %s", err)
		}
	}

	return conf
}

func main() {
	InitFlags()

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg := getConfig()
	clientset, err := versioned.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building provisioning clientset: %s", err)
	}
	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err)
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})

	controller := provisioning.NewProvisioningController(clientset, pulumi.Create,
		migration.KubeJobsMigrationForTenant(kubeClient, controllers.PlatformNamespaceFilter), eventBroadcaster, messaging.DefaultMessagingPublisher())
	if err = controller.Run(5, stopCh); err != nil {
		klog.Fatalf("Error running controller: %s", err)
	}

	<-stopCh
}
