package main

import (
	"github.com/liqoTech/liqo/cmd/virtual-kubelet/internal/provider"
	"github.com/liqoTech/liqo/cmd/virtual-kubelet/internal/provider/mock"
	"github.com/liqoTech/liqo/internal/kubernetes"
)

func registerMock(s *provider.Store) {
	s.Register("mock", func(cfg provider.InitConfig) (provider.Provider, error) { //nolint:errcheck
		return mock.NewMockProvider(
			cfg.ConfigPath,
			cfg.NodeName,
			cfg.OperatingSystem,
			cfg.InternalIP,
			cfg.DaemonPort,
		)
	})
}

func registerKubernetes(s *provider.Store) error {
	return s.Register("kubernetes", func(cfg provider.InitConfig) (provider.Provider, error) { //nolint:errcheck
		return kubernetes.NewKubernetesProvider(
			cfg.NodeName,
			cfg.ClusterId,
			cfg.HomeClusterId,
			cfg.OperatingSystem,
			cfg.InternalIP,
			cfg.DaemonPort,
			cfg.ConfigPath,
			cfg.RemoteKubeConfig,
		)
	})
}
