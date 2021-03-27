package namespaceController

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"time"
)

var _ = Describe("Namespace controller", func() {

	const (
		NamespaceName = "namespace-test"
		timeout       = time.Second * 30 // controller thread sometimes doesn't start
		interval      = time.Millisecond * 250
		mappingLabel       = "mapping.liqo.io"
		offloadingLabel    = "offloading.liqo.io"
		randomLabel        = "random-random"
		offloadingSingleClusterLabel    = "offloading.liqo.io/cluster-1" // also virtual node will have this one

	)

	var (
		namespace *corev1.Namespace
		buffer    *bytes.Buffer
		flags     *flag.FlagSet
	)

	type patchStringValue struct {
		Op    string `json:"op"`
		Path  string `json:"path"`
		Value string `json:"value"`
	}

	BeforeEach(func() {
		buffer = &bytes.Buffer{}
		flags = &flag.FlagSet{}
		klog.InitFlags(flags)
		_ = flags.Set("logtostderr", "false")
		_ = flags.Set("v", "2")
		klog.SetOutput(buffer)
		buffer.Reset()
	})

	// TODO: create also virtual node in the test environment
	// TODO: synchronize in a better way than logs, maybe with channels (if possible), with log and time checking
	//       behaviour is unpredictable, controller's thread sometimes still sleeping instead of logging some messages
	//       so the other thread which is running test cases, will fail.

	AfterEach(func() {
		By("Try to delete namespace " + NamespaceName)
		// TODO: possible unique deletion of namespace, now if one test fails also the other fails
		//createdNamespace:= &corev1.Namespace{}
		//k8sClient.Get(ctx, types.NamespacedName{Name: NamespaceName}, createdNamespace)
		//Expect(k8sClient.Delete(ctx, createdNamespace)).Should(Succeed())
		buffer.Reset()
	})

	Context("Adding some labels", func() {

		ctx := context.Background()

		BeforeEach(func() {
			namespace = &corev1.Namespace{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Namespace",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: NamespaceName,
					Labels: map[string]string{
						"random": "",
					},
				},
			}

			By("Try to create new Namespace " + NamespaceName)
			Expect(k8sClient.Create(ctx, namespace)).Should(Succeed())
		})





		It("Adding a new label which is not relevant for us", func() {
			createdNamespace := &corev1.Namespace{}

			By("Try to get namespace " + NamespaceName)
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: NamespaceName}, createdNamespace)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("Try to patch it.")
			payload := []patchStringValue{{
				Op:    "add",
				Path:  "/metadata/labels/" + randomLabel,
				Value:  "",
			}}
			payloadBytes, _ := json.Marshal(payload)
			Expect(k8sClient.Patch(ctx, createdNamespace,client.RawPatch(types.JSONPatchType, payloadBytes))).Should(Succeed())

			By("Try to catch right log.")
			Eventually(func() bool {
				klog.Flush()
				return strings.Contains(buffer.String(), "---------------------- Delete all unnecessary mapping in NNT")
			}, timeout, interval).Should(BeTrue())

			By("Try to delete namespace.")
			Expect(k8sClient.Delete(ctx, createdNamespace)).Should(Succeed())

			By("Try to catch right log.")
			Eventually(func() bool {
				klog.Flush()
				return strings.Contains(buffer.String(), "---------------------- I have to delete all entries from NNT, if present")
			}, timeout, interval).Should(BeTrue())

		})

		It("Adding a mapping.liqo.io label", func() {
			createdNamespace := &corev1.Namespace{}

			By("Try to get namespace " + NamespaceName)
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: NamespaceName}, createdNamespace)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("Try to patch it.")
			payload := []patchStringValue{{
				Op:    "add",
				Path:  "/metadata/labels/" + mappingLabel,
				Value:  "",
			}}
			payloadBytes, _ := json.Marshal(payload)
			Expect(k8sClient.Patch(ctx, createdNamespace,client.RawPatch(types.JSONPatchType, payloadBytes))).Should(Succeed())

			By("Try to catch right log.")
			Eventually(func() bool {
				klog.Flush()
				return strings.Contains(buffer.String(), "---------------------- Watch for virtual-nodes labels")
			}, timeout, interval).Should(BeTrue())

			By("Try to catch right log.")
			Eventually(func() bool {
				klog.Flush()
				return strings.Contains(buffer.String(), "---------------------- Delete all unnecessary mapping in NNT")
			}, timeout, interval).Should(BeTrue())

			By("Try to delete namespace.")
			Expect(k8sClient.Delete(ctx, createdNamespace)).Should(Succeed())


			By("Try to catch log.")
			Eventually(func() bool {
				klog.Flush()
				return strings.Contains(buffer.String(), "---------------------- I have to delete all entries from NNT, if present")
			}, timeout, interval).Should(BeTrue())

		})



		It("Adding 'mapping.liqo.io' label and 'offloading.liqo.io' label", func() {
			createdNamespace := &corev1.Namespace{}

			By("Try to get namespace " + NamespaceName)
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: NamespaceName}, createdNamespace)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("Try to patch it with " + mappingLabel)
			payload := []patchStringValue{{
				Op:    "add",
				Path:  "/metadata/labels/" + mappingLabel,
				Value:  "",
			}}
			payloadBytes, _ := json.Marshal(payload)
			Expect(k8sClient.Patch(ctx, createdNamespace,client.RawPatch(types.JSONPatchType, payloadBytes))).Should(Succeed())
			buffer.Reset()

			By("Try to patch it with " + offloadingLabel)
			payload = []patchStringValue{{
				Op:    "add",
				Path:  "/metadata/labels/" + offloadingLabel,
				Value:  "",
			}}
			payloadBytes, _ = json.Marshal(payload)
			Expect(k8sClient.Patch(ctx, createdNamespace,client.RawPatch(types.JSONPatchType, payloadBytes))).Should(Succeed())

			By("Try to catch right log.")
			Eventually(func() bool {
				klog.Flush()
				return strings.Contains(buffer.String(), "---------------------- I have to create remote namespaces on all virtual nodes, if they aren't already present")
			}, timeout, interval).Should(BeTrue())


			By("Try to delete namespace.")
			Expect(k8sClient.Delete(ctx, createdNamespace)).Should(Succeed())


			Eventually(func() bool {
				klog.Flush()
				return strings.Contains(buffer.String(), "---------------------- I have to delete all entries from NNT, if present")
			}, timeout, interval).Should(BeTrue())

		})


		It("Adding 'mapping.liqo.io' label and 'offloading.liqo.io/cluster-1' label", func() {
			createdNamespace := &corev1.Namespace{}

			By("Try to get namespace " + NamespaceName)
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: NamespaceName}, createdNamespace)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("Try to patch it.")
			patch := []byte(`{"metadata":{"labels":{"mapping.liqo.io": "random","offloading.liqo.io/cluster-1":""}}}`)
			Expect(k8sClient.Patch(ctx, createdNamespace, client.RawPatch(types.StrategicMergePatchType, patch))).Should(Succeed())


			By("Try to catch right log.")
			Eventually(func() bool {
				klog.Flush()
				return strings.Contains(buffer.String(), "---------------------- Watch for virtual-nodes labels")
			}, timeout, interval).Should(BeTrue())

			By("Try to catch right log.")
			Eventually(func() bool {
				klog.Flush()
				return strings.Contains(buffer.String(), "---------------------- Create namespace for that remote cluter")
			}, timeout, interval).Should(BeTrue())

			By("Try to catch right log.")
			Eventually(func() bool {
				klog.Flush()
				return strings.Contains(buffer.String(), "---------------------- Delete all unnecessary mapping in NNT")
			}, timeout, interval).Should(BeTrue())

			By("Try to delete namespace.")
			Expect(k8sClient.Delete(ctx, createdNamespace)).Should(Succeed())

			Eventually(func() bool {
				klog.Flush()
				return strings.Contains(buffer.String(), "---------------------- I have to delete all entries from NNT, if present")
			}, timeout, interval).Should(BeTrue())

		})

	})





	Context("Deleting and updating some labels", func() {

		ctx := context.Background()


		BeforeEach(func() {
			namespace = &corev1.Namespace{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Namespace",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: NamespaceName,
					Labels: map[string]string{
						"random": "",
						mappingLabel : "",
						offloadingLabel : "",
						offloadingSingleClusterLabel : "",
					},
				},
			}

			By("Try to create new Namespace " + NamespaceName)
			Expect(k8sClient.Create(ctx, namespace)).Should(Succeed())
		})


		It("Deleting 'mapping.liqo.io' label", func() {
			createdNamespace:= &corev1.Namespace{}

			By("Try to get namespace " + NamespaceName)
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: NamespaceName}, createdNamespace)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("Try to patch it.")
			payload := []patchStringValue{{
				Op:    "remove",
				Path:  "/metadata/labels/" + mappingLabel,
				Value:  "",
			}}
			payloadBytes, _ := json.Marshal(payload)
			Expect(k8sClient.Patch(ctx, createdNamespace,client.RawPatch(types.JSONPatchType, payloadBytes))).Should(Succeed())

			By("Try to catch right log.")
			Eventually(func() bool {
				klog.Flush()
				return strings.Contains(buffer.String(), "---------------------- Delete all unnecessary mapping in NNT")
			}, timeout, interval).Should(BeTrue())


			By("Try to delete namespace.")
			Expect(k8sClient.Delete(ctx, createdNamespace)).Should(Succeed())


			Eventually(func() bool {
				klog.Flush()
				return strings.Contains(buffer.String(), "---------------------- I have to delete all entries from NNT, if present")
			}, timeout, interval).Should(BeTrue())

		})


		It("Deleting 'offloading.liqo.io' label", func() {
			createdNamespace:= &corev1.Namespace{}

			By("Try to get namespace " + NamespaceName)
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: NamespaceName}, createdNamespace)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())


			By("Try to patch it.")
			payload := []patchStringValue{{
				Op:    "remove",
				Path:  "/metadata/labels/" + offloadingLabel,
				Value:  "",
			}}
			payloadBytes, _ := json.Marshal(payload)
			Expect(k8sClient.Patch(ctx, createdNamespace,client.RawPatch(types.JSONPatchType, payloadBytes))).Should(Succeed())


			By("Try to catch right log.")
			Eventually(func() bool {
				klog.Flush()
				return strings.Contains(buffer.String(), "---------------------- Watch for virtual-nodes labels")
			}, timeout, interval).Should(BeTrue())

			By("Try to catch right log.")
			Eventually(func() bool {
				klog.Flush()
				return strings.Contains(buffer.String(), "---------------------- Create namespace for that remote cluter")
			}, timeout, interval).Should(BeTrue())

			By("Try to delete namespace.")
			Expect(k8sClient.Delete(ctx, createdNamespace)).Should(Succeed())


			Eventually(func() bool {
				klog.Flush()
				return strings.Contains(buffer.String(), "---------------------- I have to delete all entries from NNT, if present")
			}, timeout, interval).Should(BeTrue())

		})

		It("Deleting not relevant label for our case", func() {
			createdNamespace:= &corev1.Namespace{}

			By("Try to get namespace " + NamespaceName)
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: NamespaceName}, createdNamespace)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())


			By("Try to patch it.")
			payload := []patchStringValue{{
				Op:    "remove",
				Path:  "/metadata/labels/random",
				Value:  "",
			}}
			payloadBytes, _ := json.Marshal(payload)
			Expect(k8sClient.Patch(ctx, createdNamespace,client.RawPatch(types.JSONPatchType, payloadBytes))).Should(Succeed())


			By("Try to catch right log.")
			Eventually(func() bool {
				klog.Flush()
				return strings.Contains(buffer.String(), "---------------------- I have to create remote namespaces on all virtual nodes, if they aren't already present")
			}, timeout, interval).Should(BeTrue())


			By("Try to delete namespace.")
			Expect(k8sClient.Delete(ctx, createdNamespace)).Should(Succeed())


			Eventually(func() bool {
				klog.Flush()
				return strings.Contains(buffer.String(), "---------------------- I have to delete all entries from NNT, if present")
			}, timeout, interval).Should(BeTrue())

		})


		It("Updating 'mapping.liqo.io' label", func() {
			createdNamespace:= &corev1.Namespace{}

			By("Try to get namespace " + NamespaceName)
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: NamespaceName}, createdNamespace)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("Try to patch it.")
			payload := []patchStringValue{{
				Op:    "replace",
				Path:  "/metadata/labels/" + mappingLabel,
				Value:  "random-new",
			}}
			payloadBytes, _ := json.Marshal(payload)
			Expect(k8sClient.Patch(ctx, createdNamespace,client.RawPatch(types.JSONPatchType, payloadBytes))).Should(Succeed())

			By("Try to catch right log.")
			Eventually(func() bool {
				klog.Flush()
				return strings.Contains(buffer.String(), "---------------------- I have to create remote namespaces on all virtual nodes, if they aren't already present")
			}, timeout, interval).Should(BeTrue())


			By("Try to delete namespace.")
			Expect(k8sClient.Delete(ctx, createdNamespace)).Should(Succeed())


			Eventually(func() bool {
				klog.Flush()
				return strings.Contains(buffer.String(), "---------------------- I have to delete all entries from NNT, if present")
			}, timeout, interval).Should(BeTrue())

		})



	})

})
