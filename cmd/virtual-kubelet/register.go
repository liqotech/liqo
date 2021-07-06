package main

import (
	"github.com/liqotech/liqo/cmd/virtual-kubelet/provider"
	liqoProvider "github.com/liqotech/liqo/pkg/virtualKubelet/provider"
)

func registerKubernetes(s *provider.Store) error {
	return s.Register("kubernetes", func(cfg provider.InitConfig) (provider.Provider, error) {
		return liqoProvider.NewLiqoProvider(
			cfg.NodeName,
			cfg.RemoteClusterID,
			cfg.HomeClusterID,
			cfg.InternalIP,
			cfg.DaemonPort,
			cfg.HomeKubeConfig,
			cfg.RemoteKubeConfig,
			cfg.InformerResyncPeriod,
			cfg.LiqoIpamServer,
		)
	})
}
