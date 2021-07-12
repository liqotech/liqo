package tunneloperator

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	// IP given to the labelerController.
	labelerCurrentPodIP = "10.1.1.1"
	// IP given to pods that simulate other replicas of the operator.
	labelerOtherPodIP = "10.2.2.2"
	labelerTestPod    *corev1.Pod
	labelerNamespace  = "labeler-namespace"
	labelerPodName    = "labeler-test-pod"
	labelerReq        ctrl.Request
	// Controller to be tested.
	lbc *LabelerController
)

var _ = Describe("LabelerOperator", func() {
	// Before each test we create an empty pod.
	// The right fields will be filled according to each test case.
	JustBeforeEach(func() {
		labelerReq = ctrl.Request{
			NamespacedName: types.NamespacedName{
				Namespace: labelerNamespace,
				Name:      labelerPodName,
			},
		}
		// Create the test pod with the labels already set.
		labelerTestPod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      labelerReq.Name,
				Namespace: labelerReq.Namespace,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "busybox",
						Image: "busybox",
					},
				},
			},
		}
		lbc = &LabelerController{
			PodIP:  labelerCurrentPodIP,
			Client: k8sClient,
		}
	})

	Describe("testing NewOverlayOperator function", func() {
		Context("when input parameters are correct", func() {
			It("should return labeler controller ", func() {
				lbc1 := NewLabelerController(labelerCurrentPodIP, k8sClient)
				Expect(lbc1).ShouldNot(BeNil())
			})
		})
	})

	Describe("testing reconcile function", func() {
		Context("when the pod is the current one", func() {
			It("pod does not have the label, should label the pod", func() {
				Eventually(func() error { return k8sClient.Create(context.TODO(), labelerTestPod) }).Should(BeNil())
				newPod := &corev1.Pod{}
				Eventually(func() error { return k8sClient.Get(context.TODO(), labelerReq.NamespacedName, newPod) }).Should(BeNil())
				newPod.Status.PodIP = labelerCurrentPodIP
				// Set IP address of the newly created pod.
				Eventually(func() error { return k8sClient.Status().Update(context.TODO(), newPod) }).Should(BeNil())
				// Check that the IP address has been set.
				Eventually(func() error {
					err := k8sClient.Get(context.TODO(), labelerReq.NamespacedName, newPod)
					if err != nil {
						return err
					}
					if newPod.Status.PodIP != labelerCurrentPodIP {
						return fmt.Errorf("pod ip has not been set yet")
					}
					return nil
				}).Should(BeNil())
				Eventually(func() error { _, err := lbc.Reconcile(context.TODO(), labelerReq); return err }).Should(BeNil())
				Eventually(func() error {
					err := k8sClient.Get(context.TODO(), labelerReq.NamespacedName, newPod)
					if err != nil {
						return err
					}
					if newPod.GetLabels()[gatewayLabelKey] != gatewayStatusActive {
						return fmt.Errorf(" error: label %s is different than %s", newPod.GetLabels()[gatewayLabelKey], gatewayStatusActive)
					}
					return nil
				}).Should(BeNil())
			})

			It("pod does have the label, should not change the pod", func() {
				const podName = "current-pod-active"
				labelerReq.Name = podName
				labelerTestPod.Name = podName
				labelerTestPod.SetLabels(map[string]string{
					gatewayLabelKey: gatewayStatusActive,
				})
				Eventually(func() error { return k8sClient.Create(context.TODO(), labelerTestPod) }).Should(BeNil())
				newPod := &corev1.Pod{}
				Eventually(func() error { return k8sClient.Get(context.TODO(), labelerReq.NamespacedName, newPod) }).Should(BeNil())
				newPod.Status.PodIP = labelerCurrentPodIP
				// Set IP address of the newly created pod.
				Eventually(func() error { return k8sClient.Status().Update(context.TODO(), newPod) }).Should(BeNil())
				// Check that the IP address has been set.
				Eventually(func() error {
					err := k8sClient.Get(context.TODO(), labelerReq.NamespacedName, newPod)
					if err != nil {
						return err
					}
					if newPod.Status.PodIP != labelerCurrentPodIP {
						return fmt.Errorf("pod ip has not been set yet")
					}
					return nil
				}).Should(BeNil())
				Eventually(func() error { _, err := lbc.Reconcile(context.TODO(), labelerReq); return err }).Should(BeNil())
				Eventually(func() error {
					err := k8sClient.Get(context.TODO(), labelerReq.NamespacedName, newPod)
					if err != nil {
						return err
					}
					if newPod.GetLabels()[gatewayLabelKey] != gatewayStatusActive {
						return fmt.Errorf(" error: label %s is different than %s", newPod.GetLabels()[gatewayLabelKey], gatewayStatusActive)
					}
					return nil
				}).Should(BeNil())
			})
		})

		Context("when the pod is not the current one", func() {
			It("pod is already in standby, does nothing", func() {
				const podName = "other-pod-standby"
				labelerReq.Name = podName
				labelerTestPod.Name = podName
				labelerTestPod.SetLabels(map[string]string{
					gatewayLabelKey: gatewayStatusStandby,
				})
				Eventually(func() error { return k8sClient.Create(context.TODO(), labelerTestPod) }).Should(BeNil())
				newPod := &corev1.Pod{}
				Eventually(func() error { return k8sClient.Get(context.TODO(), labelerReq.NamespacedName, newPod) }).Should(BeNil())
				newPod.Status.PodIP = labelerOtherPodIP
				// Set IP address of the newly created pod.
				Eventually(func() error { return k8sClient.Status().Update(context.TODO(), newPod) }).Should(BeNil())
				// Check that the IP address has been set.
				Eventually(func() error {
					err := k8sClient.Get(context.TODO(), labelerReq.NamespacedName, newPod)
					if err != nil {
						return err
					}
					if newPod.Status.PodIP != labelerOtherPodIP {
						return fmt.Errorf("pod ip has not been set yet")
					}
					return nil
				}).Should(BeNil())
				Eventually(func() error { _, err := lbc.Reconcile(context.TODO(), labelerReq); return err }).Should(BeNil())
			})

			It("label is set to {active}, should set it to {standby} ", func() {
				const podName = "other-pod-active"
				labelerReq.Name = podName
				labelerTestPod.Name = podName
				labelerTestPod.SetLabels(map[string]string{
					gatewayLabelKey: gatewayStatusActive,
				})
				Eventually(func() error { return k8sClient.Create(context.TODO(), labelerTestPod) }).Should(BeNil())
				newPod := &corev1.Pod{}
				Eventually(func() error { return k8sClient.Get(context.TODO(), labelerReq.NamespacedName, newPod) }).Should(BeNil())
				newPod.Status.PodIP = labelerOtherPodIP
				// Set IP address of the newly created pod.
				Eventually(func() error { return k8sClient.Status().Update(context.TODO(), newPod) }).Should(BeNil())
				// Check that the IP address has been set.
				Eventually(func() error {
					err := k8sClient.Get(context.TODO(), labelerReq.NamespacedName, newPod)
					if err != nil {
						return err
					}
					if newPod.Status.PodIP != labelerOtherPodIP {
						return fmt.Errorf("pod ip has not been set yet")
					}
					return nil
				}).Should(BeNil())
				Eventually(func() error { _, err := lbc.Reconcile(context.TODO(), labelerReq); return err }).Should(BeNil())
				Eventually(func() error {
					err := k8sClient.Get(context.TODO(), labelerReq.NamespacedName, newPod)
					if err != nil {
						return err
					}
					if newPod.GetLabels()[gatewayLabelKey] != gatewayStatusStandby {
						return fmt.Errorf(" error: label %s is different than %s string", newPod.GetLabels()[gatewayLabelKey], gatewayStatusStandby)
					}
					return nil
				}).Should(BeNil())
			})
		})

		Context("pod does not exist", func() {
			It("shold return nil", func() {
				const podName = "pod-does-not-exist"
				labelerReq.Name = podName
				labelerTestPod.Name = podName
				_, err := lbc.Reconcile(context.TODO(), labelerReq)
				Expect(err).Should(BeNil())
			})
		})

	})
})
