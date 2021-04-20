package liqonet_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/liqotech/liqo/pkg/liqonet"
)

var _ = Describe("Liqonet", func() {
	DescribeTable("MapIPToNetwork",
		func(oldIp, newPodCidr, expectedIP string, expectedErr string) {
			ip, err := liqonet.MapIPToNetwork(oldIp, newPodCidr)
			if expectedErr != "" {
				gomega.Expect(err.Error()).To(gomega.Equal(expectedErr))
			} else {
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
			}
			gomega.Expect(ip).To(gomega.Equal(expectedIP))
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
				_, err := liqonet.GetClusterID(clientset, clusterIDKey, ns, backoff)
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
				clusterID, err := liqonet.GetClusterID(clientset, clusterIDKey, "default", backoff)
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
				clusterID, err := liqonet.GetClusterID(clientset, clusterIDKey, "default", backoff)
				end := time.Now()
				Expect(err).NotTo(HaveOccurred())

				minimum_sleep := 2 * backoff.Duration
				Expect(end).Should(BeTemporally(">=", start.Add(minimum_sleep)))
				Expect(clusterID).To(Equal(clusterIDValue))
			})
		})
	})

})
