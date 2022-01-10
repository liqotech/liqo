// Copyright 2019-2022 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package authservice

import (
	"context"
	"net/http"
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

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/auth"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	peeringroles "github.com/liqotech/liqo/pkg/peering-roles"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils/apiserver"
)

// cluster-role
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=list
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=core,resourceNames="aws-auth",resources=configmaps,verbs=get;update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch
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
	clientset      kubernetes.Interface
	secretInformer cache.SharedIndexInformer

	authenticationEnabled bool

	credentialsValidator credentialsValidator
	localCluster         discoveryv1alpha1.ClusterIdentity
	namespaceManager     tenantnamespace.Manager
	identityProvider     identitymanager.IdentityProvider

	apiServerConfig apiserver.Config

	peeringPermission peeringroles.PeeringPermission
}

// NewAuthServiceCtrl creates a new Auth Controller.
func NewAuthServiceCtrl(config *rest.Config, namespace string,
	awsConfig identitymanager.AwsConfig, resyncTime time.Duration,
	apiServerConfig apiserver.Config, authEnabled, useTLS bool,
	localCluster discoveryv1alpha1.ClusterIdentity) (*Controller, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	// Complete the configuration retrieval, if necessary
	if err = apiServerConfig.Complete(config, clientset); err != nil {
		return nil, err
	}

	informerFactory := informers.NewSharedInformerFactoryWithOptions(clientset, resyncTime, informers.WithNamespace(namespace))

	secretInformer := informerFactory.Core().V1().Secrets().Informer()
	secretInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{})

	informerFactory.Start(wait.NeverStop)
	informerFactory.WaitForCacheSync(wait.NeverStop)

	namespaceManager := tenantnamespace.NewTenantNamespaceManager(clientset)

	var idProvider identitymanager.IdentityProvider
	if awsConfig.IsEmpty() {
		idProvider = identitymanager.NewCertificateIdentityProvider(
			context.Background(), clientset, localCluster, namespaceManager)
	} else {
		idProvider = identitymanager.NewIAMIdentityProvider(
			clientset, localCluster, &awsConfig, namespaceManager)
	}

	return &Controller{
		namespace:        namespace,
		clientset:        clientset,
		secretInformer:   secretInformer,
		localCluster:     localCluster,
		namespaceManager: namespaceManager,
		identityProvider: idProvider,

		apiServerConfig: apiServerConfig,

		authenticationEnabled: authEnabled,
		credentialsValidator:  &tokenValidator{},
	}, nil
}

// Start starts the authentication service.
func (authService *Controller) Start(address string, useTLS bool, certPath, keyPath string) error {
	if err := authService.configureToken(); err != nil {
		return err
	}

	// populate the lists of ClusterRoles to bind in the different peering states.
	// populate the lists of ClusterRoles to bind in the different peering states
	permissions, err := peeringroles.GetPeeringPermission(context.TODO(), authService.clientset)
	if err != nil {
		klog.Errorf("Unable to populate peering permission: %w", err)
		return err
	}
	authService.peeringPermission = *permissions

	router := httprouter.New()

	router.POST(auth.CertIdentityURI, authService.identity)
	router.GET(auth.IdsURI, authService.ids)

	if useTLS {
		err = http.ListenAndServeTLS(address, certPath, keyPath, router)
	} else {
		err = http.ListenAndServe(address, router)
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

func (authService *Controller) getTokenManager() tokenManager {
	return authService
}
