package wireguard

import (
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/liqotech/liqo/internal/utils/errdefs"
)

type wgClientFake struct {
	dev wgtypes.Device
}

func NewWgClientFake(deviceName string) (*wgClientFake, error) {
	return &wgClientFake{wgtypes.Device{Name: deviceName}}, nil
}

func (wgc *wgClientFake) configureDevice(name string, cfg wgtypes.Config) error {
	if wgc.dev.Name != name {
		return errdefs.NotFoundf("device named %s not found", name)
	}
	for _, peer := range cfg.Peers {
		if !peer.Remove {
			wgc.dev.Peers = append(wgc.dev.Peers, wgtypes.Peer{
				PublicKey:                   peer.PublicKey,
				Endpoint:                    peer.Endpoint,
				PersistentKeepaliveInterval: *peer.PersistentKeepaliveInterval,
				LastHandshakeTime:           time.Time{},
				ReceiveBytes:                0,
				TransmitBytes:               0,
				AllowedIPs:                  peer.AllowedIPs,
				ProtocolVersion:             0,
			})
			return nil
		}
		for i, p := range wgc.dev.Peers {
			if peer.PublicKey == p.PublicKey {
				wgc.dev.Peers[i] = wgc.dev.Peers[len(wgc.dev.Peers)-1]
				wgc.dev.Peers = wgc.dev.Peers[:len(wgc.dev.Peers)-1]
			}
		}
	}
	return nil
}

func (wgc *wgClientFake) device(name string) (*wgtypes.Device, error) {

	if wgc.dev.Name != name {
		return nil, errdefs.NotFoundf("device named %s not found", name)
	}
	return &wgc.dev, nil
}

func (wgc *wgClientFake) close() error {
	return nil
}
