package cmd

import (
	flag "github.com/spf13/pflag"

	"github.com/liqotech/liqo/pkg/liqoctl/install/aks"
	"github.com/liqotech/liqo/pkg/liqoctl/install/eks"
	"github.com/liqotech/liqo/pkg/liqoctl/install/gke"
	"github.com/liqotech/liqo/pkg/liqoctl/install/kubeadm"
)

var providers = []string{"kubeadm", "eks", "gke", "aks"}

var providerInitFunc = map[string]func(*flag.FlagSet){
	"kubeadm": kubeadm.GenerateFlags,
	"eks":     eks.GenerateFlags,
	"gke":     gke.GenerateFlags,
	"aks":     aks.GenerateFlags,
}
