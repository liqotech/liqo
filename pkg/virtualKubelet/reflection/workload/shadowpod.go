package workload

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

// RemoteShadowNamespacedKeyer returns a shadowpod keyer associated with the given namespace, retrieving the
// object name from its metadata.
func RemoteShadowNamespacedKeyer(namespace, nodename string) func(metadata metav1.Object) []types.NamespacedName {
	return func(metadata metav1.Object) []types.NamespacedName {
		label, ok := metadata.GetLabels()[forge.LiqoOriginClusterNodeName]
		klog.V(4).Infof("RemoteShadowNamespaceKeyer: Comparing %q with %q", label, nodename)
		if ok && label == nodename {
			return []types.NamespacedName{{Namespace: namespace, Name: metadata.GetName()}}
		}
		return []types.NamespacedName{}
	}
}
