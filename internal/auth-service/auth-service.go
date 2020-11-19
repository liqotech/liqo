package auth_service

import (
	"context"
	"github.com/julienschmidt/httprouter"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/crdClient"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
	"net/http"
	"strings"
	"time"
)

type AuthServiceCtrl struct {
	namespace      string
	clientset      kubernetes.Interface
	saInformer     cache.SharedIndexInformer
	nodeInformer   cache.SharedIndexInformer
	secretInformer cache.SharedIndexInformer
}

func NewAuthServiceCtrl(namespace string, kubeconfigPath string, resyncTime time.Duration) (*AuthServiceCtrl, error) {
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
	saInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{})

	nodeInformer := informerFactory.Core().V1().Nodes().Informer()
	nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{})

	secretInformer := informerFactory.Core().V1().Secrets().Informer()
	secretInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{})

	informerFactory.Start(wait.NeverStop)
	informerFactory.WaitForCacheSync(wait.NeverStop)

	return &AuthServiceCtrl{
		namespace:      namespace,
		clientset:      clientset,
		saInformer:     saInformer,
		nodeInformer:   nodeInformer,
		secretInformer: secretInformer,
	}, nil
}

func (authService *AuthServiceCtrl) Start(listeningPort string, certFile string, keyFile string) error {
	if err := authService.configureToken(); err != nil {
		return err
	}

	router := httprouter.New()

	router.POST("/role", authService.role)

	err := http.ListenAndServeTLS(strings.Join([]string{":", listeningPort}, ""), certFile, keyFile, router)
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
