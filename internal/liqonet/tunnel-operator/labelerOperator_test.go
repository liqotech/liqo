// Copyright 2019-2022 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tunneloperator

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	// IP given to the labelerController.
	labelerCurrentPodIP = "10.1.1.1"
	// IP given to pods that simulate other replicas of the operator.
	labelerOtherPodIP = "10.2.2.2"
	labelerTestPod    *corev1.Pod
	labelerNamespace  = "labeler-namespace"
	labelerPodName    = "labeler-test-pod"
	labelerSvcName    = "labeler-test-svc"
	labelerTestSvc    *corev1.Service
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
		// Declare test pod.
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
		// Declare test service.
		labelerTestSvc = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      labelerSvcName,
				Namespace: labelerReq.Namespace,
				Labels: map[string]string{
					podNameLabelKey:      podNameLabelValue,
					podComponentLabelKey: podComponentLabelValue,
				},
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					"app": "test",
				},
				Ports: []corev1.ServicePort{
					{
						Port: 80,
						TargetPort: intstr.IntOrString{
							Type:   intstr.Int,
							IntVal: 80,
						},
					},
				},
			},
		}
		lbc = &LabelerController{
			PodIP:  labelerCurrentPodIP,
			Client: k8sClient,
		}
	})
	JustAfterEach(func() {
		// Remove the existing pod.
		Eventually(func() error {
			labelerReq.Name = labelerPodName
			pod := new(corev1.Pod)
			err := k8sClient.Get(context.TODO(), labelerReq.NamespacedName, pod)
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
			if apierrors.IsNotFound(err) {
				return nil
			}
			return k8sClient.Delete(context.TODO(), pod)
		}).Should(BeNil())
		// Remove the existing service.
		Eventually(func() error {
			svcList := new(corev1.ServiceList)
			labelsSelector := client.MatchingLabels{
				podComponentLabelKey: podComponentLabelValue,
				podNameLabelKey:      podNameLabelValue,
			}
			err := lbc.List(context.TODO(), svcList, labelsSelector)
			if err != nil {
				return err
			}
			for i := range svcList.Items {
				err = k8sClient.Delete(context.TODO(), &svcList.Items[i])
				if err != nil {
					return err
				}
			}
			return nil
		}).Should(BeNil())
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
			It("pod does not have the label, should label the pod and annotate the service", func() {
				// Create the service.
				Eventually(func() error { return k8sClient.Create(context.TODO(), labelerTestSvc) }).Should(BeNil())
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
				// Check that the service has been annotated.
				Eventually(func() error {
					labelerReq.Name = labelerSvcName
					svc := new(corev1.Service)
					err := k8sClient.Get(context.TODO(), labelerReq.NamespacedName, svc)
					if err != nil {
						return err
					}
					if svc.GetAnnotations()[serviceAnnotationKey] != labelerCurrentPodIP {
						return fmt.Errorf(" error: annotation %s is different than %s", svc.GetAnnotations()[serviceAnnotationKey], labelerCurrentPodIP)
					}
					return nil
				}).Should(BeNil())
			})

			It("pod does have the label, should not change the pod", func() {
				Eventually(func() error { return k8sClient.Create(context.TODO(), labelerTestSvc) }).Should(BeNil())
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

			It("pod does have the label but service does not exist, should return an error", func() {
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
				Eventually(func() error { _, err := lbc.Reconcile(context.TODO(), labelerReq); return err }).
					Should(MatchError("expected number of services for the gateway is {1}, instead we found {0}"))

			})

		})

		Context("when the pod is not the current one", func() {
			It("pod is already in standby, does nothing", func() {
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
				_, err := lbc.Reconcile(context.TODO(), labelerReq)
				Expect(err).Should(BeNil())
			})
		})
	})

	Describe("testing annotateGatewayService function", func() {
		Context("service does exist", func() {
			It("only one service exists, should annotate it", func() {
				// Create the service.
				Eventually(func() error { return k8sClient.Create(context.TODO(), labelerTestSvc) }).Should(BeNil())
				Eventually(func() error { err := lbc.annotateGatewayService(context.TODO()); return err }).Should(BeNil())
				// Check that the service has been annotated
				Eventually(func() error {
					labelerReq.Name = labelerSvcName
					svc := new(corev1.Service)
					err := k8sClient.Get(context.TODO(), labelerReq.NamespacedName, svc)
					if err != nil {
						return err
					}
					if svc.GetAnnotations()[serviceAnnotationKey] != labelerCurrentPodIP {
						return fmt.Errorf(" error: annotation %s is different than %s", svc.GetAnnotations()[serviceAnnotationKey], labelerCurrentPodIP)
					}
					return nil
				}).Should(BeNil())
			})

			It("only one service exists and is annotated, should return nil", func() {
				// Add annotation to service.
				labelerTestSvc.SetAnnotations(map[string]string{
					serviceAnnotationKey: labelerCurrentPodIP,
				})
				// Create the service.
				Eventually(func() error { return k8sClient.Create(context.TODO(), labelerTestSvc) }).Should(BeNil())
				Eventually(func() error { err := lbc.annotateGatewayService(context.TODO()); return err }).Should(BeNil())
				// Check that the service has been annotated
				Eventually(func() error {
					labelerReq.Name = labelerSvcName
					svc := new(corev1.Service)
					err := k8sClient.Get(context.TODO(), labelerReq.NamespacedName, svc)
					if err != nil {
						return err
					}
					if svc.GetAnnotations()[serviceAnnotationKey] != labelerCurrentPodIP {
						return fmt.Errorf(" error: annotation %s is different than %s", svc.GetAnnotations()[serviceAnnotationKey], labelerCurrentPodIP)
					}
					return nil
				}).Should(BeNil())
			})

			It("more then one services exists, should return error", func() {
				// Second svc.
				secondSvc := labelerTestSvc.DeepCopy()
				// Create the first service.
				Eventually(func() error { return k8sClient.Create(context.TODO(), labelerTestSvc) }).Should(BeNil())
				// Create a second service.
				secondSvc.Name = "second-svc-test"
				Eventually(func() error { return k8sClient.Create(context.TODO(), secondSvc) }).Should(BeNil())
				Eventually(func() error { err := lbc.annotateGatewayService(context.TODO()); return err }).
					Should(MatchError("expected number of services for the gateway is {1}, instead we found {2}"))
			})
		})

		Context("svc does not exist", func() {
			It("should return error", func() {
				Eventually(func() error { err := lbc.annotateGatewayService(context.TODO()); return err }).
					Should(MatchError("expected number of services for the gateway is {1}, instead we found {0}"))
			})
		})
	})
})
