package kubeadm

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	providerPrefix             = "kubeadm"
	serviceCIDRParameterFilter = `--service-cluster-ip-range=.*`
	podCIDRParameterFilter     = `--cluster-cidr=.*`
	kubeSystemNamespaceName    = "kube-system"
)

var kubeControllerManagerLabels = map[string]string{"component": "kube-controller-manager", "tier": "control-plane"}

// Kubeadm contains the parameters required to install Liqo on a kubeadm cluster and a dedicated client to fetch
// those values.
type Kubeadm struct {
	APIServer   string
	Config      *rest.Config
	PodCIDR     string
	ServiceCIDR string
	K8sClient   kubernetes.Interface
}
