package foreignclusteroperator

import (
	"context"
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

	// populate the lists of ClusterRoles to bind in the different peering states
	permissions, err := peeringroles.GetPeeringPermission(context.TODO(), clientset)
	if err != nil {
		klog.Errorf("Unable to populate peering permission: %w", err)
		os.Exit(1)
	}

	if err = (&ForeignClusterReconciler{
		Client:               mgr.GetClient(),
		LiqoNamespacedClient: namespacedClient,
		Scheme:               mgr.GetScheme(),
		clusterID:            localClusterID,
		liqoNamespace:        namespace,
		RequeueAfter:         requeueAfter,
		ConfigProvider:       discoveryCtrl,
		namespaceManager:     namespaceManager,
		identityManager:      idManager,
		peeringPermission:    *permissions,
	}).SetupWithManager(mgr); err != nil {
		klog.Error(err, "unable to create controller", "controller", "ForeignCluster")
		os.Exit(1)
	}
}
