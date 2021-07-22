package utils_test

import (
	"context"
	"net"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/liqotech/liqo/pkg/liqonet/utils"
)

const (
	invalidValue      = "invalidValue"
	CIDRAddressNetErr = "CIDR address"
	labelKey          = "net.liqo.io/LabelKey"
	labelValue        = "LabelValue"
	annotationKey     = "net.liqo.io/AnnotationKey"
	annotationValue   = "AnnotationValue"
	interfaceName     = "dummy-link"
)

var (
	// corev1.Pod impements the client.Object interface.
	testPod *corev1.Pod
)

var _ = Describe("Liqonet", func() {
	JustBeforeEach(func() {
		testPod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					labelKey: labelValue,
				},
				Annotations: map[string]string{
					annotationKey: annotationValue,
				},
			}}
	})

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

	Describe("testing AddAnnotationToObj function", func() {
		Context("when annotations map is nil", func() {
			It("should create the map and return true", func() {
				testPod.Annotations = nil
				ok := utils.AddAnnotationToObj(testPod, annotationKey, annotationValue)
				Expect(ok).Should(BeTrue())
				Expect(len(testPod.GetAnnotations())).Should(BeNumerically("==", 1))
			})
		})

		Context("when annotation already exists", func() {
			It("annotation is the same, should return false", func() {
				ok := utils.AddAnnotationToObj(testPod, annotationKey, annotationValue)
				Expect(ok).Should(BeFalse())
				Expect(len(testPod.GetAnnotations())).Should(BeNumerically("==", 1))
			})

			It("annotation value is outdated", func() {
				const newValue = "differentValue"
				ok := utils.AddAnnotationToObj(testPod, annotationKey, newValue)
				Expect(ok).Should(BeTrue())
				Expect(len(testPod.GetAnnotations())).Should(BeNumerically("==", 1))
				value, ok := testPod.GetAnnotations()[annotationKey]
				Expect(value).Should(Equal(newValue))
				Expect(ok).Should(BeTrue())
			})
		})

		Context("when annotation with given key does not exist", func() {
			It("should return true", func() {
				const newKey = "newTestingKey"
				ok := utils.AddAnnotationToObj(testPod, newKey, annotationValue)
				Expect(ok).Should(BeTrue())
				Expect(len(testPod.GetAnnotations())).Should(BeNumerically("==", 2))
				value, ok := testPod.GetAnnotations()[annotationKey]
				Expect(value).Should(Equal(annotationValue))
				Expect(ok).Should(BeTrue())
			})
		})
	})

	Describe("testing GetAnnotationValueFromObj function", func() {
		Context("when annotations map is nil", func() {
			It("should return an empty string", func() {
				testPod.Annotations = nil
				value := utils.GetAnnotationValueFromObj(testPod, annotationKey)
				Expect(value).Should(Equal(""))
			})
		})

		Context("annotation with the given key exists", func() {
			It("should return the correct value", func() {
				value := utils.GetAnnotationValueFromObj(testPod, annotationKey)
				Expect(value).Should(Equal(annotationValue))
			})
		})

		Context("annotation with the given key does not exist", func() {
			It("should return an empty string", func() {
				value := utils.GetAnnotationValueFromObj(testPod, "notExistinKey")
				Expect(value).Should(Equal(""))
			})
		})
	})

	Describe("testing AddLabelToObj function", func() {
		Context("when label map is nil", func() {
			It("should create the map and return true", func() {
				testPod.Labels = nil
				ok := utils.AddLabelToObj(testPod, labelKey, labelValue)
				Expect(ok).Should(BeTrue())
				Expect(len(testPod.GetLabels())).Should(BeNumerically("==", 1))
			})
		})

		Context("when label already exists", func() {
			It("label is the same, should return false", func() {
				ok := utils.AddLabelToObj(testPod, labelKey, labelValue)
				Expect(ok).Should(BeFalse())
				Expect(len(testPod.GetLabels())).Should(BeNumerically("==", 1))
			})

			It("label value is outdated", func() {
				newValue := "differentValue"
				ok := utils.AddLabelToObj(testPod, labelKey, newValue)
				Expect(ok).Should(BeTrue())
				Expect(len(testPod.GetAnnotations())).Should(BeNumerically("==", 1))
				value, ok := testPod.GetLabels()[labelKey]
				Expect(value).Should(Equal(newValue))
				Expect(ok).Should(BeTrue())
			})
		})

		Context("when label with given key does not exist", func() {
			It("should return true", func() {
				newKey := "newTestingKey"
				ok := utils.AddLabelToObj(testPod, newKey, labelValue)
				Expect(ok).Should(BeTrue())
				Expect(len(testPod.GetLabels())).Should(BeNumerically("==", 2))
				value, ok := testPod.GetLabels()[newKey]
				Expect(value).Should(Equal(labelValue))
				Expect(ok).Should(BeTrue())
			})
		})
	})

	Describe("testing GetLabelValueFromObj function", func() {
		Context("when label map is nil", func() {
			It("should return an empty string", func() {
				testPod.Labels = nil
				value := utils.GetLabelValueFromObj(testPod, labelKey)
				Expect(value).Should(Equal(""))
			})
		})

		Context("label with the given key exists", func() {
			It("should return the correct value", func() {
				value := utils.GetLabelValueFromObj(testPod, labelKey)
				Expect(value).Should(Equal(labelValue))
			})
		})

		Context("label with the given key does not exist", func() {
			It("should return an empty string", func() {
				value := utils.GetLabelValueFromObj(testPod, "nonExistingKey")
				Expect(value).Should(Equal(""))
			})
		})
	})

	Describe("testing DeleteIfaceByName function", func() {
		Context("when network interface exists", func() {
			BeforeEach(func() {
				// Create dummy link.
				err := netlink.LinkAdd(&netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: interfaceName}})
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("should return nil", func() {
				err := utils.DeleteIFaceByName(interfaceName)
				Expect(err).Should(BeNil())
				_, err = netlink.LinkByName(interfaceName)
				Expect(err).Should(MatchError("Link not found"))
			})
		})

		Context("when network interface does not exist", func() {
			It("should return nil", func() {
				err := utils.DeleteIFaceByName("not-existing")
				Expect(err).Should(BeNil())
			})
		})
	})
})
