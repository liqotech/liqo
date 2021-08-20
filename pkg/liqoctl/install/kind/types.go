package kind

import "github.com/liqotech/liqo/pkg/liqoctl/install/kubeadm"

// Kind contains the kubeadm struct and inherits all the methods implemented by the provider.
type Kind struct {
	kubeadm.Kubeadm
}
