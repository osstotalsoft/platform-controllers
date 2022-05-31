package controllers

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"

	vault "github.com/hashicorp/vault/api"
	auth "github.com/hashicorp/vault/api/auth/kubernetes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"totalsoft.ro/platform-controllers/pkg/apis/configuration/v1alpha1"
	clientset "totalsoft.ro/platform-controllers/pkg/generated/clientset/versioned"
	clientsetScheme "totalsoft.ro/platform-controllers/pkg/generated/clientset/versioned/scheme"
	informers "totalsoft.ro/platform-controllers/pkg/generated/informers/externalversions/configuration/v1alpha1"
	listers "totalsoft.ro/platform-controllers/pkg/generated/listers/configuration/v1alpha1"

	csiClientset "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
	csiInformers "sigs.k8s.io/secrets-store-csi-driver/pkg/client/informers/externalversions/apis/v1"
	csiListers "sigs.k8s.io/secrets-store-csi-driver/pkg/client/listers/apis/v1"

	apisv1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
)

const (
	secretsControllerAgentName = "secrets-controller"
	requeueInterval            = 2 * time.Minute
)

var getSecrets = getSecretWithKubernetesAuth

type secretSpec struct {
	Path string `json:"platformRef"`
	Key  string `json:"domain"`
}
type SecretsController struct {
	csiClientset             csiClientset.Interface
	configurationClientset   clientset.Interface
	spcInformer              csiInformers.SecretProviderClassInformer
	secretsAggregateInformer informers.SecretsAggregateInformer

	spcLister               csiListers.SecretProviderClassLister
	spcSynced               cache.InformerSynced
	secretAggregatesLister  listers.SecretsAggregateLister
	secretsAggregatesSynced cache.InformerSynced

	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.

	recorder record.EventRecorder
	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
}

func NewSecretsController(
	csiClientset csiClientset.Interface,
	configurationClientset clientset.Interface,
	spcInformer csiInformers.SecretProviderClassInformer,
	secretsAggregateInformer informers.SecretsAggregateInformer,
	eventBroadcaster record.EventBroadcaster,
) *SecretsController {
	controller := &SecretsController{
		csiClientset:             csiClientset,
		configurationClientset:   configurationClientset,
		spcInformer:              spcInformer,
		spcLister:                spcInformer.Lister(),
		spcSynced:                spcInformer.Informer().HasSynced,
		secretsAggregateInformer: secretsAggregateInformer,
		secretAggregatesLister:   secretsAggregateInformer.Lister(),
		secretsAggregatesSynced:  secretsAggregateInformer.Informer().HasSynced,

		recorder:  &record.FakeRecorder{},
		workqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "secrets"),
	}

	utilruntime.Must(clientsetScheme.AddToScheme(scheme.Scheme))
	if eventBroadcaster != nil {
		controller.recorder = eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: secretsControllerAgentName})
	}

	klog.Info("Setting up event handlers")

	// Set up an event handler for when SecretsAggregate resources change
	addSecretsAggregateHandlers(secretsAggregateInformer, controller.enqueueDomain)

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *SecretsController) Run(workers int, stopCh <-chan struct{}) error {

	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting secrets controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.spcSynced, c.secretsAggregatesSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	// Launch two workers to process SecretsAggregate resources
	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

// Fetches a key-value secret (kv-v2) after authenticating to Vault with a Kubernetes service account.
// For a more in-depth setup explanation, please see the relevant readme in the hashicorp/vault-examples repo.
func getSecretWithKubernetesAuth(platform, domain, role string) ([]secretSpec, error) {
	// If set, the VAULT_ADDR environment variable will be the address that
	// your pod uses to communicate with Vault.
	config := vault.DefaultConfig() // modify for more granular configuration

	client, err := vault.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize Vault client: %w", err)
	}

	// 	jwt := `
	// eyJhbGciOiJSUzI1NiIsImtpZCI6IkFxSFVTUzNpd0JNRnUxTWRWYUVsZndKVUdqZUs5WTA0bjhtMGFnR1U3WkEifQ.eyJhdWQiOlsiaHR0cHM6Ly90ZXN0a3ViZS1kbnMtODZmOGY2NGQuaGNwLndlc3RldXJvcGUuYXptazhzLmlvIiwiXCJ0ZXN0a3ViZS1kbnMtOD
	// ZmOGY2NGQuaGNwLndlc3RldXJvcGUuYXptazhzLmlvXCIiXSwiZXhwIjoxNjg1NDQ1NTIxLCJpYXQiOjE2NTM5MDk1MjEsImlzcyI6Imh0dHBzOi8vdGVzdGt1YmUtZG5zLTg2ZjhmNjRkLmhjcC53ZXN0ZXVyb3BlLmF6bWs4cy5pbyIsImt1YmVybmV0ZXMuaW8iOns
	// ibmFtZXNwYWNlIjoiZGVmYXVsdCIsInBvZCI6eyJuYW1lIjoidGVzdC12YXVsdC1tdWx0aXRlbmFudC1jc2ktNWNiODRiZDdjNC1wbGNjciIsInVpZCI6IjJiMDY4ZmY1LTA2OWUtNDdmOS1hNGFhLWQwMjIzN2M1OTg5NiJ9LCJzZXJ2aWNlYWNjb3VudCI6eyJuYW1l
	// IjoidmF1bHQiLCJ1aWQiOiJlMWFiMDcxZi02OTMwLTRmOTMtYjkxNy1iZTA0MTc0OWYxMDIifSwid2FybmFmdGVyIjoxNjUzOTEzMTI4fSwibmJmIjoxNjUzOTA5NTIxLCJzdWIiOiJzeXN0ZW06c2VydmljZWFjY291bnQ6ZGVmYXVsdDp2YXVsdCJ9.FaUCA4tfrl55
	// 4wcyvZZd2KKlXEaLiHy30JNn2JMsopNCzPFu3VaHPWA-Em3HYVusjdtlired969UPQf8CVvWaR6gxkaXeoxXvcAkJxd43QWwTbXJLIfx4G8XLDkCj0D2cKUPvDCUEvqc3Rg5jmkjhVSpUikhls47eqnYLnEVwFdsasqYxEkEiOR1YCYn3Z7g21lhu9N0ZBvFE_QTlbVJk
	// AnzGSFkAMhTpIxqerWeSEEfY7w7K_Lj394kbhBvSKgO1EyKXOtLjjNlQHNyUR7qzDpqjOHT1YJi1LlUpPfMC-rzJeYtQddM8PYQiB8icnynkakQC29FxpjZv9fe4RNhw2zKJqS8LopLFXhW_6ovJKXoS9vZ_KuGjtXD2yyiZU69MhzMF8DLGG3jHSIVc8Qs-Cw4OjBuI0
	// KJ3cWZC9ES-MfQVp4x--UPjc71ru-_PWPvYSRSeiiExzXSok0f6J6e9Gka4rTHdGIUoKdsb0njIIE_x1zBL526idRNvrB370_3Ovb8QvfU4TJpHj1Fy6Lp-luTaLxmtSesY2U1HnkjjL6O6I889TuTnyDuee-8YJUPzLjrBLFCSO0e7_66uu7gob_n794Xanh5zOeOM38
	// 3HUQMDYVbTD_Ok_MJqTPeSRDsJtgDD9TSDGwV-iwv7baW-f6XVAdAZjct8uasnsNTTk8`

	// The service-account token will be read from the path where the token's
	// Kubernetes Secret is mounted. By default, Kubernetes will mount it to
	// /var/run/secrets/kubernetes.io/serviceaccount/token
	k8sAuth, err := auth.NewKubernetesAuth(role) //, auth.WithServiceAccountToken(jwt))
	if err != nil {
		return nil, fmt.Errorf("unable to initialize Kubernetes auth method: %w", err)
	}

	authInfo, err := client.Auth().Login(context.TODO(), k8sAuth)
	if err != nil {
		return nil, fmt.Errorf("unable to log in with Kubernetes auth: %w", err)
	}
	if authInfo == nil {
		return nil, fmt.Errorf("no auth info was returned after login")
	}

	secretList := []secretSpec{}

	var listSecrets func(string, string) error
	listSecrets = func(secretEngine, parentPath string) (err error) {
		listedPathsSecret, err := client.Logical().List(fmt.Sprintf("%s/metadata/%s", secretEngine, parentPath))
		if err != nil {
			return fmt.Errorf("unable to list secrets: %w", err)
		}

		isLeaf := listedPathsSecret == nil
		if isLeaf {
			secretPath := fmt.Sprintf("%s/data/%s", secretEngine, parentPath)
			secret, err := client.Logical().Read(secretPath)
			if err != nil {
				return fmt.Errorf("unable to read secret: %w", err)
			}
			if secret == nil {
				return nil
			}
			secretData, ok := secret.Data["data"].(map[string]interface{})
			if !ok {
				return fmt.Errorf("data type assertion failed: %T %#v", secret.Data["data"], secret.Data["data"])
			}
			for secretKey := range secretData {
				secretList = append(secretList, secretSpec{Path: secretPath, Key: secretKey})
			}
			return nil
		}

		listedPaths, ok := listedPathsSecret.Data["keys"].([]interface{})
		if !ok {
			return fmt.Errorf("keys type assertion failed: %T %#v", listedPathsSecret.Data["keys"], listedPathsSecret.Data["keys"])
		}

		for _, path := range listedPaths {
			pathString, ok := path.(string)
			if !ok {
				return fmt.Errorf("key type assertion failed: %T %#v", path, path)
			}

			if err := listSecrets(secretEngine, parentPath+pathString); err != nil {
				return err
			}
		}
		return nil
	}

	err = listSecrets(platform, domain+"/")
	return secretList, err
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *SecretsController) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *SecretsController) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Foo resource to be synced.
		if err := c.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)

		//Requeue the processing after the configured interval
		c.workqueue.AddAfter(key, requeueInterval)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the SecretsAggregate resource
// with the current status of the resource.
func (c *SecretsController) syncHandler(key string) error {
	// Convert the namespace::domain string into a distinct namespace and domain
	platform, domain, err := decodeDomainKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	outputSpcName := fmt.Sprintf("%s-%s-aggregate", platform, domain)

	domainAndPlatformLabelSelector, err :=
		labels.ValidatedSelectorFromSet(map[string]string{
			domainLabelName:   domain,
			platformLabelName: platform,
		})

	if err != nil {
		utilruntime.HandleError(err)
		return nil
	}

	// Get the SecretAggregates resource with this namespace::domain
	secretAggregates, err := c.secretAggregatesLister.SecretsAggregates("").List(labels.Everything())
	if err != nil {
		return err
	}

	n := 0
	for _, db := range secretAggregates {
		if db.Spec.PlatformRef == platform && db.Spec.Domain == domain {
			secretAggregates[n] = db
			n++
		}
	}
	secretAggregates = secretAggregates[:n]

	if len(secretAggregates) != 1 {
		// Cleanup if SecretsAggregate was invalidated
		if len(secretAggregates) == 0 {
			domainSpcs, err := c.spcLister.SecretProviderClasses("").List(domainAndPlatformLabelSelector)
			if err == nil {
				for _, domainSpc := range domainSpcs {
					if domainSpc.Name == outputSpcName {
						c.csiClientset.SecretsstoreV1().SecretProviderClasses(domainSpc.Namespace).Delete(context.TODO(), domainSpc.Name, metav1.DeleteOptions{})
					}
				}
			}
		}
		msg := fmt.Sprintf("there should be exactly one SecretsAggregate for a platform %s and domain %s. Found: %d", platform, domain, len(secretAggregates))

		for _, secretsAggregate := range secretAggregates {
			c.recorder.Event(secretsAggregate, corev1.EventTypeWarning, ErrorSynced, msg)
			c.updateStatus(secretsAggregate, false, "Aggregation failed: "+msg)
		}

		utilruntime.HandleError(fmt.Errorf(msg))
		return nil
	}

	secretsAggregate := secretAggregates[0]
	secretsAggregate, err = c.resetStatus(secretsAggregate)
	if err != nil {
		return err
	}

	role := fmt.Sprintf("%s-readonly", platform)

	globalSecrets, err := getSecrets(platform, globalDomainLabelValue, role)
	if err != nil {
		c.updateStatus(secretsAggregate, false, err.Error())
		return err
	}

	secrets, err := getSecrets(platform, domain, role)
	if err != nil {
		c.updateStatus(secretsAggregate, false, err.Error())
		return err
	}

	secrets = append(globalSecrets, secrets...)

	aggregatedSpc := c.aggregateSecrets(secretsAggregate, secrets, outputSpcName, role)

	// Get the output SPC
	outputSpc, err := c.spcLister.SecretProviderClasses(secretsAggregate.Namespace).Get(outputSpcName)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		outputSpc, err = c.csiClientset.SecretsstoreV1().SecretProviderClasses(secretsAggregate.Namespace).Create(context.TODO(), aggregatedSpc, metav1.CreateOptions{})
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		c.recorder.Event(secretsAggregate, corev1.EventTypeWarning, ErrorSynced, err.Error())
		c.updateStatus(secretsAggregate, false, "Aggregation failed"+err.Error())
		return err
	}

	// If the SPC is not controlled by this SecretsAggregate resource, we should log
	// a warning to the event recorder and return error msg.
	if !metav1.IsControlledBy(outputSpc, secretsAggregate) {
		msg := fmt.Sprintf(MessageResourceExists, outputSpc.Name)
		c.recorder.Event(secretsAggregate, corev1.EventTypeWarning, ErrResourceExists, msg)
		c.updateStatus(secretsAggregate, false, "Aggregation failed"+err.Error())
		return nil
	}

	// If the existing SPC data differs from the aggregation result we
	// should update the SPC resource.
	if !reflect.DeepEqual(aggregatedSpc.Spec.Parameters, outputSpc.Spec.Parameters) {
		klog.V(4).Infof("Secret values changed")
		err = c.csiClientset.SecretsstoreV1().SecretProviderClasses(secretsAggregate.Namespace).Delete(context.TODO(), aggregatedSpc.Name, metav1.DeleteOptions{})
		if err == nil {
			_, err = c.csiClientset.SecretsstoreV1().SecretProviderClasses(secretsAggregate.Namespace).Create(context.TODO(), aggregatedSpc, metav1.CreateOptions{})
		}
	}

	// If an error occurs during Update, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		c.recorder.Event(secretsAggregate, corev1.EventTypeWarning, ErrorSynced, err.Error())
		c.updateStatus(secretsAggregate, false, "Aggregation failed: "+err.Error())
		return err
	}

	// Finally, we update the status block of the SecretsAggregate resource to reflect the
	// current state of the world
	c.updateStatus(secretsAggregate, true, SuccessSynced)
	c.recorder.Event(secretsAggregate, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *SecretsController) aggregateSecrets(secretAggregate *v1alpha1.SecretsAggregate, secrets []secretSpec, outputName, role string) *apisv1.SecretProviderClass {
	mergedSecrets := map[string]secretSpec{}
	for _, secret := range secrets {
		if existingValue, ok := mergedSecrets[secret.Key]; ok {
			msg := fmt.Sprintf("Key %s already exists with value %s. It will be replaced by secret %s with value %s", secret.Key, existingValue, secret.Key, secret)
			c.recorder.Event(secretAggregate, corev1.EventTypeWarning, ErrResourceExists, msg)
		}
		mergedSecrets[secret.Key] = secret
	}
	var orderedSecretKeys []string
	for key := range mergedSecrets {
		orderedSecretKeys = append(orderedSecretKeys, key)
	}
	sort.Strings(orderedSecretKeys)

	var sb strings.Builder
	for _, secretKey := range orderedSecretKeys {
		objString := fmt.Sprintf(`
- objectName: "%s"
  secretPath: "%s"
  secretKey: "%s"`, secretKey, mergedSecrets[secretKey].Path, mergedSecrets[secretKey].Key)
		sb.WriteString(objString)
	}
	objectsString := sb.String()

	vaultAddress := os.Getenv("VAULT_ADDR") //"http://vault.default:8200"

	return &apisv1.SecretProviderClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: outputName,
			Labels: map[string]string{
				domainLabelName:   secretAggregate.Spec.Domain,
				platformLabelName: secretAggregate.Spec.PlatformRef,
			},
			Namespace: secretAggregate.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(secretAggregate, v1alpha1.SchemeGroupVersion.WithKind("SecretsAggregate")),
			},
		},
		Spec: apisv1.SecretProviderClassSpec{
			Provider: "vault",
			Parameters: map[string]string{
				"objects":      objectsString,
				"roleName":     role,
				"vaultAddress": vaultAddress,
			},
		},
	}
}

// resetStatus resets the conditions of the SecretsAggregate to meta.Condition
// of type meta.ReadyCondition with status 'Unknown' and meta.ProgressingReason
// reason and message. It returns the modified SecretsAggregate.
func (c *SecretsController) resetStatus(secretsAggregate *v1alpha1.SecretsAggregate) (*v1alpha1.SecretsAggregate, error) {
	secretsAggregate = secretsAggregate.DeepCopy()
	secretsAggregate.Status.Conditions = []metav1.Condition{}
	newCondition := metav1.Condition{
		Type:    ReadyCondition,
		Status:  metav1.ConditionUnknown,
		Reason:  ProgressingReason,
		Message: "aggregation in progress",
	}
	apimeta.SetStatusCondition(&secretsAggregate.Status.Conditions, newCondition)

	return c.configurationClientset.ConfigurationV1alpha1().SecretsAggregates(secretsAggregate.Namespace).UpdateStatus(context.TODO(), secretsAggregate, metav1.UpdateOptions{})
}

func (c *SecretsController) updateStatus(secretAggregate *v1alpha1.SecretsAggregate, isReady bool, message string) {
	secretAggregate = secretAggregate.DeepCopy()

	var conditionStatus metav1.ConditionStatus
	var reason string
	if isReady {
		conditionStatus = metav1.ConditionTrue
		reason = SucceededReason
	} else {
		conditionStatus = metav1.ConditionFalse
		reason = FailedReason
	}

	secretAggregate.Status.Conditions = []metav1.Condition{}
	newCondition := metav1.Condition{
		Type:    ReadyCondition,
		Status:  conditionStatus,
		Reason:  reason,
		Message: message,
	}
	apimeta.SetStatusCondition(&secretAggregate.Status.Conditions, newCondition)

	_, err := c.configurationClientset.ConfigurationV1alpha1().SecretsAggregates(secretAggregate.DeepCopy().Namespace).UpdateStatus(context.TODO(), secretAggregate, metav1.UpdateOptions{})

	if err != nil {
		utilruntime.HandleError(err)
	}
}

func (c *SecretsController) enqueueDomain(platform string, domain string) {
	if domain == globalDomainLabelValue {
		secretAggregates, err := c.secretAggregatesLister.SecretsAggregates("").List(labels.Everything())
		if err != nil {
			return
		}
		for _, secretAggregate := range secretAggregates {
			if secretAggregate.Spec.PlatformRef == platform {
				key := encodeDomainKey(platform, secretAggregate.Spec.Domain)
				c.workqueue.Add(key)
			}
		}
	} else {
		key := encodeDomainKey(platform, domain)
		c.workqueue.Add(key)
	}

}

func addSecretsAggregateHandlers(informer informers.SecretsAggregateInformer, handler func(platform string, domain string)) {
	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			comp := obj.(*v1alpha1.SecretsAggregate)
			if platform, domain, ok := getSecretsAggregatePlatformAndDomain(comp); ok {
				klog.V(4).InfoS("SecretsAggregate added", "name", comp.Name, "namespace", comp.Namespace, "platform", platform, "domain", domain)
				handler(platform, domain)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldComp := oldObj.(*v1alpha1.SecretsAggregate)
			newComp := newObj.(*v1alpha1.SecretsAggregate)
			oldPlatform, oldDomain, oldOk := getSecretsAggregatePlatformAndDomain(oldComp)
			newPlatform, newDomain, newOk := getSecretsAggregatePlatformAndDomain(newComp)
			targetChanged := oldPlatform != newPlatform || oldDomain != newDomain

			if !oldOk && !newOk {
				return
			}

			if oldOk && targetChanged {
				klog.V(4).InfoS("SecretsAggregate invalidated", "name", newComp.Name, "namespace", newComp.Namespace, "platform", oldPlatform, "domain", oldDomain)
				handler(oldPlatform, oldDomain)
			}

			if oldOk && newOk {
				if oldComp.Spec == newComp.Spec && reflect.DeepEqual(oldComp.Labels, newComp.Labels) {
					return
				}

				klog.V(4).InfoS("SecretsAggregate updated", "name", newComp.Name, "namespace", newComp.Namespace, "platform", oldPlatform, "domain", newDomain)
				handler(newPlatform, newDomain)
			}
		},
		DeleteFunc: func(obj interface{}) {
			comp := obj.(*v1alpha1.SecretsAggregate)
			if platform, domain, ok := getSecretsAggregatePlatformAndDomain(comp); ok {
				klog.V(4).InfoS("SecretsAggregate deleted", "name", comp.Name, "namespace", comp.Namespace, "platform", platform, "domain", domain)
				// Output SPC automatically deleted because it is controlled
			}
		},
	})
}

func getSecretsAggregatePlatformAndDomain(secretsAggregate *v1alpha1.SecretsAggregate) (platform string, domain string, ok bool) {
	domain = secretsAggregate.Spec.Domain
	if len(domain) == 0 {
		return "", domain, false
	}

	platform = secretsAggregate.Spec.PlatformRef
	if len(platform) == 0 {
		return platform, domain, false
	}

	return platform, domain, true
}
