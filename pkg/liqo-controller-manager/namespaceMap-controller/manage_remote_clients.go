package namespaceMap_controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryV1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
)

// This function returns a rest.Config from a Kubeconfig contained in a Secret
func (r *NamespaceMapReconciler) getKubeConfig(reference *corev1.ObjectReference) (*rest.Config, error) {

	if reference == nil {
		return nil, fmt.Errorf("must specify reference")
	}

	secret := &corev1.Secret{}
	if err := r.Get(context.TODO(), types.NamespacedName{Namespace: reference.Namespace, Name: reference.Name}, secret); err != nil {
		klog.Errorf("unable to get Secret '%s'", reference.Name)
		return nil, err
	}

	kubeconfig := func() (*clientcmdapi.Config, error) {
		return clientcmd.Load(secret.Data["kubeconfig"])
	}
	return clientcmd.BuildConfigFromKubeconfigGetter("", kubeconfig)
}

// This function creates a new controller-runtime Client for a remote cluster, if it isn't already present
// in RemoteClients Struct of NamespaceMap Controller. The secret's reference is taken from the ForeignCluster
// associated with the remote cluster.
func (r *NamespaceMapReconciler) checkRemoteClientPresence(clusterId string) error {
	if r.RemoteClients == nil {
		r.RemoteClients = map[string]client.Client{}
	}

	if _, ok := r.RemoteClients[clusterId]; !ok {
		fcl := &discoveryV1alpha1.ForeignClusterList{}
		if err := r.List(context.TODO(), fcl, client.MatchingLabels{clusterIdForeign: clusterId}); err != nil {
			klog.Errorf("%s --> Unable to List ForeignClusters", err)
			return err
		}

		if len(fcl.Items) != 1 {
			return fmt.Errorf("there are %d ForeignClusters for the remote cluster '%s' , error", len(fcl.Items), clusterId)
		}

		config, err := r.getKubeConfig(fcl.Items[0].Status.Outgoing.IdentityRef)
		if err != nil {
			klog.Errorf("unable to get rest.config for cluster '%s'", clusterId)
			return err
		}

		if r.RemoteClients[clusterId], err = client.New(config, client.Options{Scheme: r.Scheme, Mapper: r.Mapper}); err != nil {
			klog.Errorf("unable to create client for cluster '%s'", clusterId)
			return err
		}
	}
	return nil
}
