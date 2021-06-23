package utils_test

import (
	"context"
	"net"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/liqotech/liqo/pkg/liqonet/utils"
)

const (
	invalidValue      = "invalidValue"
	CIDRAddressNetErr = "CIDR address"
)

var _ = Describe("Liqonet", func() {
	DescribeTable("MapIPToNetwork",
		func(oldIp, newPodCidr, expectedIP string, expectedErr string) {
			ip, err := utils.MapIPToNetwork(oldIp, newPodCidr)
			if expectedErr != "" {
				Expect(err.Error()).To(Equal(expectedErr))
			} else {
				Expect(err).ToNot(HaveOccurred())
			}
			Expect(ip).To(Equal(expectedIP))
		},
		Entry("Mapping 10.2.1.3 to 10.0.4.0/24", "10.0.4.0/24", "10.2.1.3", "10.0.4.3", ""),
		Entry("Mapping 10.2.1.128 to 10.0.4.0/24", "10.0.4.0/24", "10.2.1.128", "10.0.4.128", ""),
		Entry("Mapping 10.2.1.1 to 10.0.4.0/24", "10.0.4.0/24", "10.2.1.1", "10.0.4.1", ""),
		Entry("Mapping 10.2.127.128 to 10.0.128.0/23", "10.0.128.0/23", "10.2.127.128", "10.0.129.128", ""),
		Entry("Mapping 10.2.128.128 to 10.0.126.0/23", "10.0.127.0/23", "10.2.128.128", "10.0.127.128", ""),
		Entry("Mapping 10.2.128.128 to 10.0.126.0/25", "10.0.126.0/25", "10.2.128.128", "10.0.126.0", ""),
		Entry("Using an invalid newPodCidr", "10.0..0/25", "10.2.128.128", "", "invalid CIDR address: 10.0..0/25"),
		Entry("Using an invalid oldIp", "10.0.0.0/25", "10.2...128", "", "cannot parse oldIP"),
	)

	DescribeTable("GetFirstIP",
		func(network, expectedIP string, expectedErr *net.ParseError) {
			ip, err := utils.GetFirstIP(network)
			if expectedErr != nil {
				Expect(err).To(MatchError(expectedErr))
			} else {
				Expect(err).ToNot(HaveOccurred())
			}
			Expect(ip).To(Equal(expectedIP))
		},
		Entry("Passing an invalid network", invalidValue, "", &net.ParseError{Type: CIDRAddressNetErr, Text: invalidValue}),
		Entry("Passing an empty network", "", "", &net.ParseError{Type: CIDRAddressNetErr, Text: ""}),
		Entry("Passing an IP", "10.0.0.0", "", &net.ParseError{Type: CIDRAddressNetErr, Text: "10.0.0.0"}),
		Entry("Getting first IP of 10.0.0.0/8", "10.0.0.0/8", "10.0.0.0", nil),
		Entry("Getting first IP of 192.168.0.0/16", "192.168.0.0/16", "192.168.0.0", nil),
	)

	Describe("Getting ClusterID info from ConfigMap", func() {

		var (
			ns             string
			clusterIDKey   string
			clusterIDValue string
			configMapData  map[string]string
			configMap      corev1.ConfigMap
			backoff        wait.Backoff
			clientset      *fake.Clientset
		)

		BeforeEach(func() {
			ns = "default"
			clusterIDKey = "cluster-id"
			clusterIDValue = "fake-cluster-id"
			configMapData = make(map[string]string, 1)
			configMapData[clusterIDKey] = clusterIDValue
			configMap = corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterIDKey,
					Namespace: ns,
				},
				Data: configMapData,
			}

			// 3 attempts with 3 seconds sleep between one another
			// for a total of 6 seconds
			backoff = wait.Backoff{
				Steps:    3,
				Duration: 3 * time.Second,
				Factor:   1.0,
				Jitter:   0,
			}

			clientset = fake.NewSimpleClientset()
		})

		Context("When it has not been created yet", func() {
			It("should retry more times and eventually fail", func() {
				start := time.Now()
				_, err := utils.GetClusterID(clientset, clusterIDKey, ns, backoff)
				end := time.Now()
				Expect(err).To(HaveOccurred())

				timeout := backoff.Duration * time.Duration(backoff.Factor-1)
				Expect(end).Should(BeTemporally(">=", start.Add(timeout)))
			})
		})
		Context("When it has been already created", func() {
			It("should return immediately the clusterid value", func() {
				_, err := clientset.CoreV1().ConfigMaps(ns).Create(
					context.TODO(),
					&configMap,
					metav1.CreateOptions{},
				)
				Expect(err).NotTo(HaveOccurred())
				start := time.Now()
				clusterID, err := utils.GetClusterID(clientset, clusterIDKey, "default", backoff)
				end := time.Now()
				Expect(err).NotTo(HaveOccurred())
				Expect(clusterID).To(Equal(clusterIDValue))

				Expect(end).Should(BeTemporally("<", start.Add(backoff.Duration)))
			})
		})
		Context("When it has been created during backoff", func() {
			It("should detect the configmap and return the ClusterID value", func() {

				go func() {
					time.Sleep(4 * time.Second)
					_, err := clientset.CoreV1().ConfigMaps(ns).Create(
						context.TODO(),
						&configMap,
						metav1.CreateOptions{},
					)
					Expect(err).NotTo(HaveOccurred())
				}()
				start := time.Now()
				clusterID, err := utils.GetClusterID(clientset, clusterIDKey, "default", backoff)
				end := time.Now()
				Expect(err).NotTo(HaveOccurred())

				minimum_sleep := 2 * backoff.Duration
				Expect(end).Should(BeTemporally(">=", start.Add(minimum_sleep)))
				Expect(clusterID).To(Equal(clusterIDValue))
			})
		})
	})

	Describe("testing getOverlayIP function", func() {
		Context("when input parameter is correct", func() {
			It("should return a valid ip", func() {
				Expect(utils.GetOverlayIP("10.200.1.1")).Should(Equal("240.200.1.1"))
			})
		})

		Context("when input parameter is not correct", func() {
			It("should return an empty string", func() {
				Expect(utils.GetOverlayIP("10.200.")).Should(Equal(""))
			})
		})
	})
})
