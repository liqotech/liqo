package auth_service

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
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/liqotech/liqo/apis/config/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	garbage_collection "github.com/liqotech/liqo/internal/auth-service/garbage-collection"
	"github.com/liqotech/liqo/pkg/auth"
	"github.com/liqotech/liqo/pkg/clusterID"
	"github.com/liqotech/liqo/pkg/crdClient"
	"github.com/liqotech/liqo/pkg/discovery"
	"github.com/liqotech/liqo/pkg/identityManager"
	peeringRoles "github.com/liqotech/liqo/pkg/peering-roles"
	"github.com/liqotech/liqo/pkg/tenantControlNamespace"
)

//cluster-role
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=create;delete;list;deletecollection;get
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=create;delete;list;deletecollection
// +kubebuilder:rbac:groups=config.liqo.io,resources=clusterconfigs,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=peeringrequests,verbs=get;create;update;delete
// +kubebuilder:rbac:groups=certificates.k8s.io,resources=certificatesigningrequests,verbs=get;create;list;watch
// +kubebuilder:rbac:groups=certificates.k8s.io,resources=certificatesigningrequests/approval,verbs=update
// +kubebuilder:rbac:groups=certificates.k8s.io,resources=signers,verbs=approve
// tenant control namespace management
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;create;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=create;deletecollection
//role
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=secrets,verbs=create;update;get;list;watch;delete
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=configmaps,verbs=create;update;get;list;watch;delete
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=serviceaccounts,verbs=get;list;watch;create;delete;update;patch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,namespace="do-not-care",resources=roles,verbs=create;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,namespace="do-not-care",resources=rolebindings,verbs=create;delete

// AuthServiceCtrl is the controller for the Authentication Service.
type AuthServiceCtrl struct {
	namespace      string
	restConfig     *rest.Config
	clientset      kubernetes.Interface
	saInformer     cache.SharedIndexInformer
	nodeInformer   cache.SharedIndexInformer
	secretInformer cache.SharedIndexInformer
	useTLS         bool

	credentialsValidator credentialsValidator
	localClusterID       clusterID.ClusterID
	namespaceManager     tenantControlNamespace.TenantControlNamespaceManager
	identityManager      identityManager.IdentityManager

	config          *v1alpha1.AuthConfig
	apiServerConfig *v1alpha1.ApiServerConfig
	discoveryConfig v1alpha1.DiscoveryConfig
	configMutex     sync.RWMutex

	peeringPermission peeringRoles.PeeringPermission
}

// NewAuthServiceCtrl creates a new Auth Controller.
func NewAuthServiceCtrl(namespace, kubeconfigPath string, resyncTime time.Duration, useTLS bool) (*AuthServiceCtrl, error) {
	config, err := crdClient.NewKubeconfig(kubeconfigPath, &discoveryv1alpha1.GroupVersion, nil)
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	informerFactory := informers.NewSharedInformerFactoryWithOptions(clientset, resyncTime, informers.WithNamespace(namespace))

	saInformer := informerFactory.Core().V1().ServiceAccounts().Informer()
	saInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			if sa, ok := newObj.(*v1.ServiceAccount); ok {
				if !sa.GetDeletionTimestamp().IsZero() && controllerutil.ContainsFinalizer(sa, discovery.GarbageCollection) {
					garbage_collection.OnDeleteServiceAccount(clientset, sa)
				}
			} else {
				klog.Error("Error decoding ServiceAccount")
			}
		},
	})

	nodeInformer := informerFactory.Core().V1().Nodes().Informer()
	nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{})

	secretInformer := informerFactory.Core().V1().Secrets().Informer()
	secretInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{})

	localClusterID, err := clusterID.NewClusterID(kubeconfigPath)
	if err != nil {
		return nil, err
	}

	informerFactory.Start(wait.NeverStop)
	informerFactory.WaitForCacheSync(wait.NeverStop)

	namespaceManager := tenantControlNamespace.NewTenantControlNamespaceManager(clientset)
	idManager := identityManager.NewCertificateIdentityManager(clientset, localClusterID, namespaceManager)

	return &AuthServiceCtrl{
		namespace:            namespace,
		restConfig:           config,
		clientset:            clientset,
		saInformer:           saInformer,
		nodeInformer:         nodeInformer,
		secretInformer:       secretInformer,
		localClusterID:       localClusterID,
		namespaceManager:     namespaceManager,
		identityManager:      idManager,
		useTLS:               useTLS,
		credentialsValidator: &tokenValidator{},
	}, nil
}

// Start starts the authentication service.
func (authService *AuthServiceCtrl) Start(listeningPort, certFile, keyFile string) error {
	if err := authService.configureToken(); err != nil {
		return err
	}

	// populate the lists of ClusterRoles to bind in the different peering states.
	if err := authService.populatePermission(); err != nil {
		return err
	}

	router := httprouter.New()

	router.POST("/identity/certificate", authService.identity)
	router.POST("/identity", authService.role)
	router.GET("/ids", authService.ids)

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

func (authService *AuthServiceCtrl) configureToken() error {
	if err := authService.createToken(); err != nil {
		return err
	}

	authService.secretInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			newSecret, ok := newObj.(*v1.Secret)
			if !ok {
				return
			}
			if newSecret.Name != authTokenSecretName {
				return
			}

			if _, err := authService.getTokenFromSecret(newSecret); err != nil {
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
			if newSecret.Name != authTokenSecretName {
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

func (authService *AuthServiceCtrl) getConfigProvider() auth.AuthConfigProvider {
	return authService
}

func (authService *AuthServiceCtrl) getTokenManager() tokenManager {
	return authService
}

// populatePermission populates the list of ClusterRoles to bind in the different peering phases reading the ClusterConfig CR.
func (authService *AuthServiceCtrl) populatePermission() error {
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
