package liqonet_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/liqotech/liqo/pkg/liqonet"
)

var _ = Describe("Liqonet", func() {

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
