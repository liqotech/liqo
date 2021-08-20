package foreignclusteroperator

import (
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/internal/discovery"
	"github.com/liqotech/liqo/pkg/auth"
	"github.com/liqotech/liqo/pkg/clusterid"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	peeringroles "github.com/liqotech/liqo/pkg/peering-roles"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
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
	mgr manager.Manager, namespacedClient client.Client, clientset kubernetes.Interface, namespace string,
	requeueAfter time.Duration, discoveryCtrl *discovery.Controller, localClusterID clusterid.ClusterID) {
	namespaceManager := tenantnamespace.NewTenantNamespaceManager(clientset)
	idManager := identitymanager.NewCertificateIdentityManager(clientset, localClusterID, namespaceManager)

	if err := getForeignClusterReconciler(
		mgr,
		namespacedClient,
		clientset,
		localClusterID,
		namespace,
		requeueAfter,
		discoveryCtrl,
		discoveryCtrl,
		namespaceManager,
		idManager,
	).SetupWithManager(mgr); err != nil {
		klog.Error(err, "unable to create controller", "controller", "ForeignCluster")
		os.Exit(1)
	}
}

func getForeignClusterReconciler(mgr manager.Manager,
	namespacedClient client.Client,
	clientset kubernetes.Interface,
	localClusterID clusterid.ClusterID,
	namespace string,
	requeueAfter time.Duration,
	configProvider discovery.ConfigProvider,
	authConfigProvider auth.ConfigProvider,
	namespaceManager tenantnamespace.Manager,
	idManager identitymanager.IdentityManager) *ForeignClusterReconciler {
	reconciler := &ForeignClusterReconciler{
		Client:               mgr.GetClient(),
		LiqoNamespacedClient: namespacedClient,
		Scheme:               mgr.GetScheme(),
		clusterID:            localClusterID,
		liqoNamespace:        namespace,
		RequeueAfter:         requeueAfter,
		ConfigProvider:       configProvider,
		AuthConfigProvider:   authConfigProvider,
		namespaceManager:     namespaceManager,
		identityManager:      idManager,
	}

	// populate the lists of ClusterRoles to bind in the different peering states
	if err := reconciler.populatePermission(clientset); err != nil {
		klog.Errorf("unable to populate peering permission: %v", err)
		os.Exit(1)
	}

	return reconciler
}

// populatePermission populates the list of ClusterRoles to bind in the different peering phases reading the ClusterConfig CR.
func (r *ForeignClusterReconciler) populatePermission(clientset kubernetes.Interface) error {
	peeringPermission, err := peeringroles.GetPeeringPermission(clientset, r.AuthConfigProvider)
	if err != nil {
		klog.Error(err)
		return err
	}

	if peeringPermission != nil {
		r.peeringPermission = *peeringPermission
	}
	return nil
}
