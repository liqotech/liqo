// Package outgoing defines the permission to be enabled when we send a ResourceRequest,
// this ClusterRole has the permissions required to a remote cluster to manage
// an incoming peering (outgoing for the local cluster),
// when the Pods will be offloaded from the local cluster
package outgoing

// +kubebuilder:rbac:groups=net.liqo.io,resources=networkconfigs,verbs=get;update;patch;list;watch;delete;create
// +kubebuilder:rbac:groups=net.liqo.io,resources=networkconfigs/status,verbs=get;update;patch;list;watch;delete;create

// +kubebuilder:rbac:groups=sharing.liqo.io,resources=resourceoffers,verbs=get;update;patch;list;watch;delete;create
// +kubebuilder:rbac:groups=sharing.liqo.io,resources=resourceoffers/status,verbs=get;update;patch;list;watch;delete;create
