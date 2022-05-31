package main

import (
	"flag"
	"path/filepath"
	"time"

	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/homedir"
	csiclientset "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
	csiinformers "sigs.k8s.io/secrets-store-csi-driver/pkg/client/informers/externalversions"
	"totalsoft.ro/platform-controllers/internal/controllers"
	clientset "totalsoft.ro/platform-controllers/pkg/generated/clientset/versioned"
	informers "totalsoft.ro/platform-controllers/pkg/generated/informers/externalversions"
	"totalsoft.ro/platform-controllers/pkg/signals"
)

var (
	kubeConfig     *rest.Config
	kubeConfigPath string
)

func main() {
	InitFlags()

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg := getConfig()

	csiClient, err := csiclientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	platformClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building example clientset: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err)
	}

	csiInformerFactory := csiinformers.NewSharedInformerFactory(csiClient, time.Second*30)
	platformInformerFactory := informers.NewSharedInformerFactory(platformClient, time.Second*30)

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})

	controller := controllers.NewSecretsController(csiClient, platformClient,
		csiInformerFactory.Secretsstore().V1().SecretProviderClasses(),
		platformInformerFactory.Configuration().V1alpha1().SecretsAggregates(),
		eventBroadcaster)

	// notice that there is no need to run Start methods in a separate goroutine. (i.e. go kubeInformerFactory.Start(stopCh)
	// Start method is non-blocking and runs all registered informers in a dedicated goroutine.
	csiInformerFactory.Start(stopCh)
	platformInformerFactory.Start(stopCh)

	if err = controller.Run(2, stopCh); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}
}

// InitFlags is for explicitly initializing the flags.
func InitFlags() {
	flag.Set("alsologtostderr", "true")
	if home := homedir.HomeDir(); home != "" {
		flag.StringVar(&kubeConfigPath, "kubeConfigPath", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		flag.StringVar(&kubeConfigPath, "kubeConfigPath", "", "absolute path to the kubeconfig file")
	}

	klog.InitFlags(nil)

	flag.Parse()
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
