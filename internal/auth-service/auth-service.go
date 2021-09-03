package authservice

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/apis/config/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/auth"
	"github.com/liqotech/liqo/pkg/clusterid"
	crdclient "github.com/liqotech/liqo/pkg/crdClient"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	peeringRoles "github.com/liqotech/liqo/pkg/peering-roles"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

// cluster-role
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=list
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=core,resourceNames="aws-auth",resources=configmaps,verbs=get;update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch
// +kubebuilder:rbac:groups=config.liqo.io,resources=clusterconfigs,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=certificates.k8s.io,resources=certificatesigningrequests,verbs=get;create;list;watch
// +kubebuilder:rbac:groups=certificates.k8s.io,resources=certificatesigningrequests/approval,verbs=update
// +kubebuilder:rbac:groups=certificates.k8s.io,resources=signers,verbs=approve
// tenant namespace management
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;create;delete;update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;create;deletecollection;delete
// role
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=secrets,verbs=create;update;get;list;watch;delete
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=configmaps,verbs=create;update;get;list;watch;delete

// Controller is the controller for the Authentication Service.
type Controller struct {
	namespace      string
	restConfig     *rest.Config
	clientset      kubernetes.Interface
	secretInformer cache.SharedIndexInformer
	useTLS         bool

	credentialsValidator credentialsValidator
	localClusterID       clusterid.ClusterID
	namespaceManager     tenantnamespace.Manager
	identityProvider     identitymanager.IdentityProvider

	config          *v1alpha1.AuthConfig
	apiServerConfig *v1alpha1.APIServerConfig
	discoveryConfig v1alpha1.DiscoveryConfig
	configMutex     sync.RWMutex

	peeringPermission peeringRoles.PeeringPermission
}

// NewAuthServiceCtrl creates a new Auth Controller.
func NewAuthServiceCtrl(namespace, kubeconfigPath string,
	awsConfig identitymanager.AwsConfig,
	resyncTime time.Duration, useTLS bool) (*Controller, error) {
	config, err := crdclient.NewKubeconfig(kubeconfigPath, &discoveryv1alpha1.GroupVersion, nil)
	if err != nil {
		return nil, err
	}
	restcfg.SetRateLimiter(config)
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	informerFactory := informers.NewSharedInformerFactoryWithOptions(clientset, resyncTime, informers.WithNamespace(namespace))

	secretInformer := informerFactory.Core().V1().Secrets().Informer()
	secretInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{})

	localClusterID, err := clusterid.NewClusterID(kubeconfigPath)
	if err != nil {
		return nil, err
	}

	informerFactory.Start(wait.NeverStop)
	informerFactory.WaitForCacheSync(wait.NeverStop)

	namespaceManager := tenantnamespace.NewTenantNamespaceManager(clientset)

	var idProvider identitymanager.IdentityProvider
	if awsConfig.IsEmpty() {
		idProvider = identitymanager.NewCertificateIdentityProvider(
			context.Background(), clientset, localClusterID, namespaceManager)
	} else {
		idProvider = identitymanager.NewIAMIdentityProvider(
			clientset, localClusterID, &awsConfig, namespaceManager)
	}

	return &Controller{
		namespace:        namespace,
		restConfig:       config,
		clientset:        clientset,
		secretInformer:   secretInformer,
		localClusterID:   localClusterID,
		namespaceManager: namespaceManager,
		identityProvider: idProvider,

		useTLS:               useTLS,
		credentialsValidator: &tokenValidator{},
	}, nil
}

// Start starts the authentication service.
func (authService *Controller) Start(listeningPort, certFile, keyFile string) error {
	if err := authService.configureToken(); err != nil {
		return err
	}

	// populate the lists of ClusterRoles to bind in the different peering states.
	if err := authService.populatePermission(); err != nil {
		return err
	}

	router := httprouter.New()

	router.POST(auth.CertIdentityURI, authService.identity)
	router.GET(auth.IdsURI, authService.ids)

	var err error
	if authService.useTLS {
		err = http.ListenAndServeTLS(strings.Join([]string{":", listeningPort}, ""), certFile, keyFile, router)
	} else {
		err = http.ListenAndServe(strings.Join([]string{":", listeningPort}, ""), router)
	}
	if err != nil {
		klog.Error(err)
		return err
	}
	return nil
}

func (authService *Controller) configureToken() error {
	if err := authService.createToken(); err != nil {
		return err
	}

	authService.secretInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			newSecret, ok := newObj.(*v1.Secret)
			if !ok {
				return
			}
			if newSecret.Name != auth.TokenSecretName {
				return
			}

			if _, err := auth.GetTokenFromSecret(newSecret); err != nil {
				err := authService.clientset.CoreV1().Secrets(authService.namespace).Delete(context.TODO(), newSecret.Name, metav1.DeleteOptions{})
				if err != nil {
					klog.Error(err)
					return
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			newSecret, ok := obj.(*v1.Secret)
			if !ok {
				return
			}
			if newSecret.Name != auth.TokenSecretName {
				return
			}

			if err := authService.createToken(); err != nil {
				klog.Error(err)
				return
			}
		},
	})
	return nil
}

func (authService *Controller) getConfigProvider() auth.ConfigProvider {
	return authService
}

func (authService *Controller) getTokenManager() tokenManager {
	return authService
}

// populatePermission populates the list of ClusterRoles to bind
// in the different peering phases reading the ClusterConfig CR.
func (authService *Controller) populatePermission() error {
	peeringPermission, err := peeringRoles.GetPeeringPermission(authService.clientset, authService)
	if err != nil {
		klog.Error(err)
		return err
	}

	if peeringPermission != nil {
		authService.peeringPermission = *peeringPermission
	}
	return nil
}
