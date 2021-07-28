package main

import (
	"context"

	"github.com/liqotech/liqo/cmd/virtual-kubelet/provider"
	liqoProvider "github.com/liqotech/liqo/pkg/virtualKubelet/provider"
)

func registerKubernetes(ctx context.Context, s *provider.Store) error {
	return s.Register("kubernetes", func(cfg provider.InitConfig) (provider.Provider, error) {
		return liqoProvider.NewLiqoProvider(
			ctx,
			cfg.NodeName,
			cfg.RemoteClusterID,
			cfg.HomeClusterID,
			cfg.InternalIP,
			cfg.DaemonPort,
			cfg.HomeKubeConfig,
			cfg.InformerResyncPeriod,
			cfg.LiqoIpamServer,
		)
	})
}
