// Package basic defines the permission to be enabled with the creation
// of the Tenant Control Namespace,
// this ClusterRole has the basic permissions to give to a remote cluster
package basic

// +kubebuilder:rbac:groups=discovery.liqo.io,resources=resourcerequests,verbs=get;update;patch;list;watch;delete
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=resourcerequests/status,verbs=get;update;patch;list;watch;delete
