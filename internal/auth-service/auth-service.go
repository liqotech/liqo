package auth_service

import (
	"context"
	"github.com/julienschmidt/httprouter"
	"github.com/liqotech/liqo/apis/config/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	garbage_collection "github.com/liqotech/liqo/internal/auth-service/garbage-collection"
	"github.com/liqotech/liqo/pkg/clusterID"
	"github.com/liqotech/liqo/pkg/crdClient"
	"github.com/liqotech/liqo/pkg/discovery"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strings"
	"sync"
	"time"
)

//cluster-role
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=create;delete;list;deletecollection
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=create;delete;list;deletecollection
// +kubebuilder:rbac:groups=config.liqo.io,resources=clusterconfigs,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=peeringrequests,verbs=get;create;update;delete
//role
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=secrets,verbs=create;update;get;list;watch;delete
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=configmaps,verbs=create;update;get;list;watch;delete
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=serviceaccounts,verbs=get;list;watch;create;delete;update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,namespace="do-not-care",resources=roles,verbs=create;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,namespace="do-not-care",resources=rolebindings,verbs=create;delete

type AuthServiceCtrl struct {
	namespace      string
	clientset      kubernetes.Interface
	saInformer     cache.SharedIndexInformer
	nodeInformer   cache.SharedIndexInformer
	secretInformer cache.SharedIndexInformer
	useTls         bool

	credentialsValidator credentialsValidator
	clusterId            clusterID.ClusterID

	config          *v1alpha1.AuthConfig
	discoveryConfig v1alpha1.DiscoveryConfig
	configMutex     sync.RWMutex
}

func NewAuthServiceCtrl(namespace string, kubeconfigPath string, resyncTime time.Duration, useTls bool) (*AuthServiceCtrl, error) {
	config, err := crdClient.NewKubeconfig(kubeconfigPath, &discoveryv1alpha1.GroupVersion)
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

	clusterId, err := clusterID.NewClusterID(kubeconfigPath)
	if err != nil {
		return nil, err
	}

	informerFactory.Start(wait.NeverStop)
	informerFactory.WaitForCacheSync(wait.NeverStop)

	return &AuthServiceCtrl{
		namespace:            namespace,
		clientset:            clientset,
		saInformer:           saInformer,
		nodeInformer:         nodeInformer,
		secretInformer:       secretInformer,
		clusterId:            clusterId,
		useTls:               useTls,
		credentialsValidator: &tokenValidator{},
	}, nil
}

func (authService *AuthServiceCtrl) Start(listeningPort string, certFile string, keyFile string) error {
	if err := authService.configureToken(); err != nil {
		return err
	}

	router := httprouter.New()

	router.POST("/identity", authService.role)
	router.GET("/ids", authService.ids)

	var err error
	if authService.useTls {
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
			if newSecret.Name != AuthTokenSecretName {
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
			if newSecret.Name != AuthTokenSecretName {
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

func (authService *AuthServiceCtrl) getConfigProvider() authConfigProvider {
	return authService
}

func (authService *AuthServiceCtrl) getTokenManager() tokenManager {
	return authService
}
