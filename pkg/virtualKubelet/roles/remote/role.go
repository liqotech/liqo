// Package remote defines the ClusterRole containing the permissions required by the virtual kubelet in the remote cluster.
package remote

// +kubebuilder:rbac:groups="",resources=configmaps;services;secrets;pods,verbs=get;list;watch;update;patch;delete;create
// +kubebuilder:rbac:groups="",resources=pods/status;services/status,verbs=get;update;patch;list;watch;delete;create

// +kubebuilder:rbac:groups=apps,resources=replicasets,verbs=get;list;watch;update;create;delete
// +kubebuilder:rbac:groups=discovery.k8s.io,resources=endpointslices,verbs=get;list;watch;create;update;patch;delete
