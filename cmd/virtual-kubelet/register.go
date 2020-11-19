package main

import (
	"github.com/liqotech/liqo/cmd/virtual-kubelet/internal/provider"
	liqoProvider "github.com/liqotech/liqo/pkg/virtualKubelet/provider"
)

func registerKubernetes(s *provider.Store) error {
	return s.Register("kubernetes", func(cfg provider.InitConfig) (provider.Provider, error) { //nolint:errcheck
		return liqoProvider.NewLiqoProvider(
			cfg.NodeName,
			cfg.ClusterId,
			cfg.HomeClusterId,
			cfg.InternalIP,
			cfg.DaemonPort,
			cfg.ConfigPath,
			cfg.RemoteKubeConfig,
		)
	})
}
