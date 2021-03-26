package namespaceController

import (
	"bytes"
	"context"
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

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		NamespaceName = "namespace-test"
		timeout       = time.Second * 30  // valore alto per non avere fallimenti di questo tipo
		interval      = time.Millisecond * 250
	)

	var (
		namespace  *corev1.Namespace
		buffer     *bytes.Buffer
		flags      *flag.FlagSet
	)


	BeforeEach(func() {
		buffer = &bytes.Buffer{}
		flags = &flag.FlagSet{}
		klog.InitFlags(flags)
		_ = flags.Set("logtostderr", "false")
		_ = flags.Set("v", "2")
		klog.SetOutput(buffer)
		buffer.Reset()
	})




	// ADD LABELS CASES
	Context("When adding some labels", func() {

		ctx := context.Background()

		BeforeEach(func() {
			namespace = &corev1.Namespace{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Namespace",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      NamespaceName,
					Labels: map[string]string{
						"random":     "random-value",
					},

				},
			}

			By("Try to create new Namespace " + NamespaceName)
			Expect(k8sClient.Create(ctx, namespace)).Should(Succeed())
			buffer.Reset() // without this doen't work
		})

		AfterEach(func(){
			By("Try to delete namespace "+NamespaceName)
			// non riesco a eliminare il namespace qua, in nessun modo
			//Expect(k8sClient.Delete(ctx, namespace)).Should(Succeed())
			buffer.Reset() // without this doen't work
		})







		It("Adding a new label which is not relevant for us", func() {
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
			patch := []byte(`{"metadata":{"labels":{"random-2": "random-value2"}}}`)
			Expect(k8sClient.Patch(ctx, createdNamespace,client.RawPatch(types.StrategicMergePatchType, patch))).Should(Succeed())


			// il problema grosso è che può essere che il controllore vada troppo lento, c'è un modo per sincronizzare con channel (?)
			By("Try to catch right log.")
			Eventually(func() bool {
				klog.Flush()
				return strings.Contains(buffer.String(), "---------------------- Delete all unnecessary mapping in NNT")
			}, timeout, interval).Should(BeTrue())


			By("Try to delete namespace.")
			Expect(k8sClient.Delete(ctx, createdNamespace)).Should(Succeed())

			//Eventually(func() bool {
			//	err := k8sClient.Delete(ctx, createdNamespace)
			//	if err != nil &&  !errors.IsNotFound(err) {
			//		return false
			//	}
			//	return true
			//}, timeout, interval).Should(BeTrue())


			By("Try to catch right log.")
			Eventually(func() bool {
				klog.Flush()
				return strings.Contains(buffer.String(), "---------------------- I have to delete all entries from NNT, if present")
			}, timeout, interval).Should(BeTrue())

		})








		It("Adding a mapping.liqo.io label", func() {
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
			patch := []byte(`{"metadata":{"labels":{"mapping.liqo.io": "random"}}}`)
			Expect(k8sClient.Patch(ctx, createdNamespace,client.RawPatch(types.StrategicMergePatchType, patch))).Should(Succeed())

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

			//Eventually(func() bool {
			//	err := k8sClient.Delete(ctx, createdNamespace)
			//	if err != nil &&  !errors.IsNotFound(err) {
			//		return false
			//	}
			//	return true
			//}, timeout, interval).Should(BeTrue())

			By("Try to catch log.")
			Eventually(func() bool {
				klog.Flush()
				return strings.Contains(buffer.String(), "---------------------- I have to delete all entries from NNT, if present")
			}, timeout, interval).Should(BeTrue())

		})


		It("Adding 'mapping.liqo.io' label and 'offloading.liqo.io' label", func() {
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
			patch := []byte(`{"metadata":{"labels":{"mapping.liqo.io": "random","offloading.liqo.io":""}}}`)
			Expect(k8sClient.Patch(ctx, createdNamespace,client.RawPatch(types.StrategicMergePatchType, patch))).Should(Succeed())

			By("Try to catch right log.")
			Eventually(func() bool {
				klog.Flush()
				return strings.Contains(buffer.String(), "---------------------- I have to create remote namespaces on all virtual nodes, if they aren't already present")
			}, timeout, interval).Should(BeTrue())

			Expect(strings.Contains(buffer.String(), "---------------------- Delete all unnecessary mapping in NNT")).ShouldNot(BeTrue())

			By("Try to delete namespace.")
			Expect(k8sClient.Delete(ctx, createdNamespace)).Should(Succeed())

			//Eventually(func() bool {
			//	err := k8sClient.Delete(ctx, createdNamespace)
			//	if err != nil &&  !errors.IsNotFound(err) {
			//		return false
			//	}
			//	return true
			//}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				klog.Flush()
				return strings.Contains(buffer.String(), "---------------------- I have to delete all entries from NNT, if present")
			}, timeout, interval).Should(BeTrue())

		})



		It("Adding 'mapping.liqo.io' label and 'offloading.liqo.io/cluster-1' label", func() {
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
			patch := []byte(`{"metadata":{"labels":{"mapping.liqo.io": "random","offloading.liqo.io/cluster-1":""}}}`)
			Expect(k8sClient.Patch(ctx, createdNamespace,client.RawPatch(types.StrategicMergePatchType, patch))).Should(Succeed())

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

	Context("When remove some labels", func() {

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
						"random": "random-value",
					},
				},
			}

			By("Try to create new Namespace " + NamespaceName)
			Expect(k8sClient.Create(ctx, namespace)).Should(Succeed())
			buffer.Reset() // without this doen't work
		})

		AfterEach(func() {
			By("Try to delete namespace " + NamespaceName)
			// non riesco a eliminare il namespace qua, in nessun modo
			//Expect(k8sClient.Delete(ctx, namespace)).Should(Succeed())
			buffer.Reset() // without this doen't work
		})

	})

})