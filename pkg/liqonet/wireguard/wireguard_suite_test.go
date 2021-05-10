package wireguard_test

import (
	"testing"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	wg "github.com/liqotech/liqo/pkg/liqonet/tunnel/wireguard"
	"github.com/liqotech/liqo/pkg/liqonet/wireguard"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	ka     = 1 * time.Second
	mtu    = 1350
	port   = 51194
	ipAddr = "10.1.1.1/24"

	pubKey, priKey wgtypes.Key
	devName        = "test-device"
	wgConfig       wireguard.WgConfig
	getSecret      func(name, namespace, priKey, pubKey string) *corev1.Secret
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
	getSecret = func(name, namespace, priKey, pubKey string) *corev1.Secret {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			StringData: map[string]string{wg.PublicKey: pubKey, wg.PrivateKey: priKey},
			Data:       map[string][]byte{wg.PublicKey: []byte(pubKey), wg.PrivateKey: []byte(priKey)},
		}
	}
})
