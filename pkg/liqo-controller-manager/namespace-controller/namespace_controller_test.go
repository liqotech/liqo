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
		NamespaceName = "pluto"
		timeout       = time.Second * 10
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




	// tests which add labels
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

			By("By creating a new Namespace")
			Expect(k8sClient.Create(ctx, namespace)).Should(Succeed())
		})

		//AfterEach(func(){
		//	Expect(k8sClient.Delete(ctx, namespace.DeepCopyObject())).Should(Succeed())
		//	buffer.Reset() // without this doen't work
		//})

		It("Adding a new label which is not relevant for us", func() {

			namespaceLookupKey := types.NamespacedName{Name: NamespaceName}
			createdNamespace := &corev1.Namespace{}

			// We'll need to retry getting this newly created Namespace, given that creation may not immediately happen.
			Eventually(func() bool {
				err := k8sClient.Get(ctx, namespaceLookupKey, createdNamespace)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())


			By("Try to patch it.")
			// We'll need to retry patching this newly created Namespace, given that creation may not immediately happen.
			patch := []byte(`{"metadata":{"labels":{"random2": "random-value2"}}}`)
			Eventually(func() bool {
				err := k8sClient.Patch(ctx, createdNamespace,client.RawPatch(types.StrategicMergePatchType, patch))
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())


			Eventually(func() bool {
				klog.Flush()
				return strings.Contains(buffer.String(), "----------------------Delete all unnecessary mapping in NNT")
			}, timeout, interval).Should(BeTrue())


			By("Try to delete namespace.")
			k8sClient.Delete(ctx, createdNamespace)
			Eventually(func() bool {
				klog.Flush()
				return strings.Contains(buffer.String(), "---------------------- I have to delete all entries from NNT, if present")
			}, timeout, interval).Should(BeTrue())

			buffer.Reset()

		})




		It("Adding a new label which is not relevant for us", func() {

			namespaceLookupKey := types.NamespacedName{Name: NamespaceName}
			createdNamespace := &corev1.Namespace{}

			// We'll need to retry getting this newly created Namespace, given that creation may not immediately happen.
			Eventually(func() bool {
				err := k8sClient.Get(ctx, namespaceLookupKey, createdNamespace)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())


			By("Try to patch it.")
			// We'll need to retry patching this newly created Namespace, given that creation may not immediately happen.
			patch := []byte(`{"metadata":{"labels":{"mapping.liqo.io": "random"}}}`)
			Eventually(func() bool {
				err := k8sClient.Patch(ctx, createdNamespace,client.RawPatch(types.StrategicMergePatchType, patch))
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())


			Eventually(func() bool {
				klog.Flush()
				return strings.Contains(buffer.String(), "---------------------- Watch for virtual-nodes labels")
			}, timeout, interval).Should(BeTrue())


			By("Try to delete namespace.")
			k8sClient.Delete(ctx, createdNamespace)
			Eventually(func() bool {
				klog.Flush()
				return strings.Contains(buffer.String(), "---------------------- I have to delete all entries from NNT, if present")
			}, timeout, interval).Should(BeTrue())

			buffer.Reset()

		})

	})

})