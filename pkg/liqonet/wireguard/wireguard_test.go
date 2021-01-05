package wireguard_test

import (
	"github.com/liqotech/liqo/pkg/liqonet/wireguard"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("Wireguard", func() {
	type peerConfig struct {
		pubKey        string
		endpointIP    string
		listeningPort string
		allowedIPs    []string
		keepAlive     *time.Duration
	}
	var (
		wg            *wireguard.Wireguard
		errorVerified bool
		correctConfig = peerConfig{
			pubKey:        pubKey.String(),
			endpointIP:    "10.0.0.0",
			listeningPort: "12345",
			allowedIPs:    []string{"10.1.0.0/24", "10.2.0.0/24"},
			keepAlive:     &ka,
		}
	)
	BeforeEach(func() {
		var err error
		c, _ := wireguard.NewWgClientFake(devName)
		n := wireguard.NewNetLinkerFake(false, false, false)
		wg, err = wireguard.NewWireguard(wgConfig, c, n)
		Expect(err).NotTo(HaveOccurred())
	})

	DescribeTable("Adding wireguard peer", func(configFunc func() peerConfig, expectedBool bool) {
		c := configFunc()
		err := wg.AddPeer(c.pubKey, c.endpointIP, c.listeningPort, c.allowedIPs, c.keepAlive)
		if err != nil {
			errorVerified = true
		} else {
			errorVerified = false
		}

		Expect(errorVerified).Should(Equal(expectedBool))
	},
		Entry("wrong endpoint IP format", func() peerConfig { c := correctConfig; c.endpointIP = "192.168.1.555"; return c }, true),
		Entry("wrong public key", func() peerConfig { c := correctConfig; c.pubKey = "s"; return c }, true),
		Entry("wrong port number", func() peerConfig { c := correctConfig; c.listeningPort = "L3333"; return c }, true),
		Entry("wrong allowed IPs format", func() peerConfig { c := correctConfig; c.allowedIPs = append(c.allowedIPs, "10.1.1.1"); return c }, true),
		Entry("right configuration for peer", func() peerConfig { return correctConfig }, false),
		Entry("peer exists but with different configuration", func() peerConfig {
			c := correctConfig
			err := wg.AddPeer(c.pubKey, c.endpointIP, c.listeningPort, c.allowedIPs, c.keepAlive)
			Expect(err).NotTo(HaveOccurred())
			c.listeningPort = "1111"
			return c
		}, false),
	)

	DescribeTable("Removing wireguard peer", func(pubKey string, expectedBool bool) {
		err := wg.AddPeer(correctConfig.pubKey, correctConfig.endpointIP, correctConfig.listeningPort, correctConfig.allowedIPs, correctConfig.keepAlive)
		Expect(err).NotTo(HaveOccurred())
		err = wg.RemovePeer(pubKey)
		if err != nil {
			errorVerified = true
		} else {
			errorVerified = false
		}

		Expect(errorVerified).Should(Equal(expectedBool))
	},
		Entry("peer exists", pubKey.String(), false),
		//if the peer does not exist wgctrl.client does not return an error
		Entry("peer does not exist", "14TNJjZxo6MM1RiAxA93BYljs46orAjxhXCzlLIZkU8=", false),
		Entry("wrong public key format", "94TNJjZxo6MM1RiAx", true),
	)

	It("Getting device name", func() {
		dn := wg.GetDeviceName()
		Expect(dn).Should(Equal(devName))
	})

	It("Getting link index", func() {
		li := wg.GetLinkIndex()
		Expect(li).Should(Equal(123))
	})

	It("Getting device public key", func() {
		pbk := wg.GetPubKey()
		Expect(pbk).Should(Equal(pubKey.String()))
	})
})

var _ = Describe("Wireguard newWireguard functions", func() {

	var wgClient wireguard.Client

	BeforeEach(func() {
		wgClient, _ = wireguard.NewWgClientFake(devName)
	})

	DescribeTable("Creating new wireguard device", func(testFunc func() bool, expectedBool bool) {
		outcome := testFunc()
		Expect(outcome).Should(Equal(expectedBool))
	},
		Entry("Correctly creating a wireguard instance", func() bool {
			n := wireguard.NewNetLinkerFake(false, false, false)
			_, err := wireguard.NewWireguard(wgConfig, wgClient, n)
			Expect(err).NotTo(HaveOccurred())
			return false
		}, false),
		Entry("Error while creating wireguard instance", func() bool {
			n := wireguard.NewNetLinkerFake(true, false, false)
			_, err := wireguard.NewWireguard(wgConfig, wgClient, n)
			Expect(err).To(HaveOccurred())
			return true
		}, true),
		Entry("Error while adding IP address to wireguard device", func() bool {
			n := wireguard.NewNetLinkerFake(false, true, false)
			_, err := wireguard.NewWireguard(wgConfig, wgClient, n)
			Expect(err).To(HaveOccurred())
			return true
		}, true),
		Entry("Error while setting mtu to wireguard device", func() bool {
			n := wireguard.NewNetLinkerFake(false, false, true)
			_, err := wireguard.NewWireguard(wgConfig, wgClient, n)
			Expect(err).To(HaveOccurred())
			return true
		}, true),
	)

})
