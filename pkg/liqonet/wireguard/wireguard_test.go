package wireguard_test

import (
	"context"

	wg "github.com/liqotech/liqo/pkg/liqonet/tunnel/wireguard"

	//wg "github.com/liqotech/liqo/pkg/liqonet/tunnel/wireguard"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/liqotech/liqo/pkg/liqonet/wireguard"
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

var _ = Describe("Wireguard utils", func() {
	var (
		priK, pubK wgtypes.Key
		secretName = "test-secret"
		namespace  = "test-namespace"
		client     k8s.Interface
		err        error
	)
	//for each test we call the same function with different inputs and check the returned values
	//it is called for each test after the "BeforeEach" clauses which are used to configure the test case
	//so, for each test the fake client is created containing a secret. After that the function is called
	//thanks to the "JustBeforeEach" clause, and in the "It" section we check the results are as expected
	JustBeforeEach(func() {
		priK, pubK, err = wireguard.GetKeys(secretName, namespace, client)
	})
	Describe("Getting keys from secret", func() {
		Context("The secret does not exist", func() {
			BeforeEach(func() {
				client = fake.NewSimpleClientset(getSecret("non-existing", namespace, priKey.String(), pubKey.String()))
			})

			It("Should create a new secret and generate a new pair of keys", func() {
				s, e := client.CoreV1().Secrets(namespace).Get(context.Background(), secretName, v1.GetOptions{})
				Expect(e).NotTo(HaveOccurred())
				Expect(priK.String()).To(Equal(s.StringData[wg.PrivateKey]))
				Expect(pubK.String()).To(Equal(s.StringData[wg.PublicKey]))
			})

			It("Should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("The secret exists and contains correct data", func() {
			BeforeEach(func() {
				client = fake.NewSimpleClientset(getSecret(secretName, namespace, priKey.String(), pubKey.String()))
			})

			It("Should retrieve the pair of keys from the existing secret", func() {
				s, e := client.CoreV1().Secrets(namespace).Get(context.Background(), secretName, v1.GetOptions{})
				Expect(e).NotTo(HaveOccurred())
				Expect(priK.String()).To(Equal(s.StringData[wg.PrivateKey]))
				Expect(pubK.String()).To(Equal(s.StringData[wg.PublicKey]))

			})

			It("Should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("The secret exists with an incorrect wrong publicKey", func() {
			BeforeEach(func() {
				client = fake.NewSimpleClientset(getSecret(secretName, namespace, priKey.String(), "incorrect"))
			})

			It("Should retrieve the pair of keys from the existing secret", func() {
				_, e := client.CoreV1().Secrets(namespace).Get(context.Background(), secretName, v1.GetOptions{})
				Expect(e).NotTo(HaveOccurred())
			})

			It("Should error", func() {
				Expect(err).To(HaveOccurred())
			})
		})

		Context("The secret exists with an incorrect privateKey", func() {
			BeforeEach(func() {
				client = fake.NewSimpleClientset(getSecret(secretName, namespace, "incorrect", pubKey.String()))
			})

			It("Should retrieve the pair of keys from the existing secret", func() {
				_, e := client.CoreV1().Secrets(namespace).Get(context.Background(), secretName, v1.GetOptions{})
				Expect(e).NotTo(HaveOccurred())
			})

			It("Should error", func() {
				Expect(err).To(HaveOccurred())
			})
		})

		Context("The secret exists without the privateKey entry in the data map", func() {
			BeforeEach(func() {
				s := getSecret(secretName, namespace, priKey.String(), pubKey.String())
				delete(s.Data, wg.PrivateKey)
				client = fake.NewSimpleClientset(s)
			})

			It("Should retrieve the pair of keys from the existing secret", func() {
				_, e := client.CoreV1().Secrets(namespace).Get(context.Background(), secretName, v1.GetOptions{})
				Expect(e).NotTo(HaveOccurred())
			})

			It("Should error", func() {
				Expect(err).To(HaveOccurred())
			})
		})

		Context("The secret exists without the publicKey entry in the data map", func() {
			BeforeEach(func() {
				s := getSecret(secretName, namespace, priKey.String(), pubKey.String())
				delete(s.Data, wg.PublicKey)
				client = fake.NewSimpleClientset(s)
			})

			It("Should retrieve the pair of keys from the existing secret", func() {
				_, e := client.CoreV1().Secrets(namespace).Get(context.Background(), secretName, v1.GetOptions{})
				Expect(e).NotTo(HaveOccurred())
			})

			It("Should error", func() {
				Expect(err).To(HaveOccurred())
			})
		})

	})
})
