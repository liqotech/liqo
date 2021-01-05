package wireguard

import (
	"fmt"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

//a simple interface that implements some of the methods of wgctrl.Client interface.
//a fake implementation is used for the unit tests
type Client interface {
	configureDevice(name string, cfg wgtypes.Config) error
	device(name string) (*wgtypes.Device, error)
	close() error
}

type wgClient struct {
	c *wgctrl.Client
}

func NewWgClient() (*wgClient, error) {
	var c *wgctrl.Client
	var err error
	if c, err = wgctrl.New(); err != nil {
		return nil, fmt.Errorf("unable to create wireguard client: %v", err)
	}
	return &wgClient{c}, nil
}

func (wgc *wgClient) configureDevice(name string, cfg wgtypes.Config) error {
	return wgc.c.ConfigureDevice(name, cfg)
}

func (wgc *wgClient) device(name string) (*wgtypes.Device, error) {
	return wgc.c.Device(name)
}

func (wgc *wgClient) close() error {
	return wgc.c.Close()
}
