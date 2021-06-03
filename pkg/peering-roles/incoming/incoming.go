// Package incoming defines the permission to be enabled when a ResourceRequest has been accepted,
// this ClusterRole has the permissions required to a remote cluster to manage
// an outgoing peering (incoming for the local cluster),
// when the Pods will be offloaded to the local cluster
package incoming

// +kubebuilder:rbac:groups=net.liqo.io,resources=networkconfigs,verbs=get;update;patch;list;watch;delete;create;deletecollection
// +kubebuilder:rbac:groups=net.liqo.io,resources=networkconfigs/status,verbs=get;update;patch;list;watch;delete;create;deletecollection
