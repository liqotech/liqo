package namespacemapctrl

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog"

	"github.com/liqotech/liqo/pkg/clusterid"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	tenantcontrolnamespace "github.com/liqotech/liqo/pkg/tenantControlNamespace"
)

// getKubeConfig returns a rest.Config from a Kubeconfig contained in a Secret.
func (r *NamespaceMapReconciler) getKubeConfig(reference *corev1.ObjectReference) (*rest.Config, error) {
	if reference == nil {
		return nil, fmt.Errorf("must specify reference")
	}
	secret := &corev1.Secret{}
	if err := r.Get(context.TODO(), types.NamespacedName{
		Namespace: reference.Namespace,
		Name:      reference.Name,
	}, secret); err != nil {
		klog.Errorf("unable to get Secret '%s'", reference.Name)
		return nil, err
	}

	kubeconfig := func() (*clientcmdapi.Config, error) {
		return clientcmd.Load(secret.Data["kubeconfig"])
	}
	return clientcmd.BuildConfigFromKubeconfigGetter("", kubeconfig)
}

// checkRemoteClientPresence creates a new controller-runtime Client for a remote cluster, if it isn't already present
// in RemoteClients Struct of NamespaceMap Controller.
func (r *NamespaceMapReconciler) checkRemoteClientPresence(remoteClusterID string) error {
	if r.RemoteClients == nil {
		r.RemoteClients = map[string]kubernetes.Interface{}
	}

	if _, ok := r.RemoteClients[remoteClusterID]; !ok {
		clusterID := clusterid.NewStaticClusterID(r.LocalClusterID)
		tenantNamespaceManager := tenantcontrolnamespace.NewTenantControlNamespaceManager(r.IdentityManagerClient)
		identityManager := identitymanager.NewCertificateIdentityManager(r.IdentityManagerClient, clusterID, tenantNamespaceManager)
		restConfig, err := identityManager.GetConfig(remoteClusterID, "")
		if err != nil {
			klog.Error(err)
			return err
		}

		if r.RemoteClients[remoteClusterID], err = kubernetes.NewForConfig(restConfig); err != nil {
			klog.Errorf("%s -> unable to create client for cluster '%s'", err, remoteClusterID)
			return err
		}
	}
	return nil
}
