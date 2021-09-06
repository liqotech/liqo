package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/liqotech/liqo/pkg/liqoctl/install"
	"github.com/liqotech/liqo/pkg/liqoctl/install/aks"
	"github.com/liqotech/liqo/pkg/liqoctl/install/eks"
	"github.com/liqotech/liqo/pkg/liqoctl/install/gke"
	"github.com/liqotech/liqo/pkg/liqoctl/install/k3s"
	"github.com/liqotech/liqo/pkg/liqoctl/install/kubeadm"
)

const (
	installShortHelp = "Install Liqo on a selected %s cluster"
	installLongHelp  = "Install Liqo on a selected %s cluster"
)

var providers = []string{"kubeadm", "k3s", "eks", "gke", "aks"}

var providerInitFunc = map[string]func(*cobra.Command){
	"kubeadm": kubeadm.GenerateFlags,
	"kind":    kubeadm.GenerateFlags,
	"k3s":     k3s.GenerateFlags,
	"eks":     eks.GenerateFlags,
	"gke":     gke.GenerateFlags,
	"aks":     aks.GenerateFlags,
}

func getCommand(ctx context.Context, provider string) (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   provider,
		Short: fmt.Sprintf(installShortHelp, provider),
		Long:  fmt.Sprintf(installLongHelp, provider),
		Run: func(cmd *cobra.Command, args []string) {
			install.HandleInstallCommand(ctx, cmd, os.Args[0], provider)
		},
	}

	initFunc, ok := providerInitFunc[provider]
	if !ok {
		return nil, fmt.Errorf("initFunc not found for provider %s not found", provider)
	}

	initFunc(cmd)

	return cmd, nil
}
