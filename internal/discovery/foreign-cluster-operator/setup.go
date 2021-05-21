package foreignclusteroperator

import (
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	nettypes "github.com/liqotech/liqo/apis/net/v1alpha1"
	advtypes "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/internal/discovery"
	"github.com/liqotech/liqo/pkg/auth"
	"github.com/liqotech/liqo/pkg/clusterid"
	crdclient "github.com/liqotech/liqo/pkg/crdClient"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	peeringroles "github.com/liqotech/liqo/pkg/peering-roles"
	tenantcontrolnamespace "github.com/liqotech/liqo/pkg/tenantControlNamespace"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = discoveryv1alpha1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

// StartOperator setups the ForeignCluster operator.
func StartOperator(
	mgr manager.Manager, namespace string, requeueAfter time.Duration,
	discoveryCtrl *discovery.Controller, kubeconfigPath string, useNewAuth bool) {
	config, err := crdclient.NewKubeconfig(kubeconfigPath, &discoveryv1alpha1.GroupVersion, nil)
	if err != nil {
		klog.Error(err, "unable to get kube config")
		os.Exit(1)
	}
	discoveryClient, err := crdclient.NewFromConfig(config)
	if err != nil {
		klog.Error(err, "unable to create crd client")
		os.Exit(1)
	}
	localClusterID, err := clusterid.NewClusterID(kubeconfigPath)
	if err != nil {
		klog.Error(err, "unable to get clusterid")
		os.Exit(1)
	}

	advClient, err := advtypes.CreateAdvertisementClient(kubeconfigPath, nil, true, nil)
	if err != nil {
		klog.Error(err, "unable to create local client for Advertisement")
		os.Exit(1)
	}

	networkClient, err := nettypes.CreateTunnelEndpointClient(kubeconfigPath)
	if err != nil {
		klog.Error(err, "unable to create local client for Networking")
		os.Exit(1)
	}

	namespaceManager := tenantcontrolnamespace.NewTenantControlNamespaceManager(discoveryClient.Client())
	idManager := identitymanager.NewCertificateIdentityManager(discoveryClient.Client(), localClusterID, namespaceManager)

	if err = (getForeignClusterReconciler(
		mgr,
		namespace,
		discoveryClient,
		advClient,
		networkClient,
		localClusterID,
		requeueAfter,
		discoveryCtrl,
		discoveryCtrl,
		namespaceManager,
		idManager,
		useNewAuth,
	)).SetupWithManager(mgr); err != nil {
		klog.Error(err, "unable to create controller", "controller", "ForeignCluster")
		os.Exit(1)
	}
}

func getForeignClusterReconciler(mgr manager.Manager,
	namespace string,
	client, advertisementClient, networkClient *crdclient.CRDClient,
	localClusterID clusterid.ClusterID,
	requeueAfter time.Duration,
	configProvider discovery.ConfigProvider,
	authConfigProvider auth.ConfigProvider,
	namespaceManager tenantcontrolnamespace.TenantControlNamespaceManager,
	idManager identitymanager.IdentityManager,
	useNewAuth bool) *ForeignClusterReconciler {
	reconciler := &ForeignClusterReconciler{
		Client:              mgr.GetClient(),
		Scheme:              mgr.GetScheme(),
		Namespace:           namespace,
		crdClient:           client,
		advertisementClient: advertisementClient,
		networkClient:       networkClient,
		clusterID:           localClusterID,
		ForeignConfig:       nil,
		RequeueAfter:        requeueAfter,
		ConfigProvider:      configProvider,
		AuthConfigProvider:  authConfigProvider,
		namespaceManager:    namespaceManager,
		identityManager:     idManager,
		useNewAuth:          useNewAuth,
	}

	// populate the lists of ClusterRoles to bind in the different peering states
	if err := reconciler.populatePermission(); err != nil {
		klog.Errorf("unable to populate peering permission: %v", err)
		os.Exit(1)
	}

	return reconciler
}

// populatePermission populates the list of ClusterRoles to bind in the different peering phases reading the ClusterConfig CR.
func (r *ForeignClusterReconciler) populatePermission() error {
	peeringPermission, err := peeringroles.GetPeeringPermission(r.crdClient.Client(), r.AuthConfigProvider)
	if err != nil {
		klog.Error(err)
		return err
	}

	if peeringPermission != nil {
		r.peeringPermission = *peeringPermission
	}
	return nil
}
