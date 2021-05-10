package clusterid

const (
	masterLabel    = "node-role.kubernetes.io/master"
	serviceAccount = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	configMapKey   = "cluster-id"
)
