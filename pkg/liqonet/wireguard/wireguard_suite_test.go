package wireguard_test

import (
	"github.com/liqotech/liqo/pkg/liqonet/wireguard"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	ka     = 1 * time.Second
	mtu    = 1350
	port   = 51194
	ipAddr = "10.1.1.1/24"

	pubKey   wgtypes.Key
	devName  = "test-device"
	wgConfig wireguard.WgConfig
)

func TestWireguard(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Wireguard Suite")
}

var _ = BeforeSuite(func() {
	priKey, err := wgtypes.GeneratePrivateKey()
	Expect(err).NotTo(HaveOccurred())
	pubKey = priKey.PublicKey()
	wgConfig = wireguard.WgConfig{
		Name:      devName,
		IPAddress: ipAddr,
		Mtu:       mtu,
		Port:      &port,
		PriKey:    &priKey,
		PubKey:    &pubKey,
	}
})
