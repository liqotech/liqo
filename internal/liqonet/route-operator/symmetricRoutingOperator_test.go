// Copyright 2019-2023 The Liqo Authors
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

package routeoperator

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	liqoerrors "github.com/liqotech/liqo/pkg/liqonet/errors"
	liqonetutils "github.com/liqotech/liqo/pkg/liqonet/utils"
)

var (
	srcRoutingTableID = 1000
	// Name of the node where the operator is running.
	srcNodeName = "src-operator-node"
	srcNodeIP   = "10.200.1.2"
	// Name of the node where the test pod is running.
	srcPodNodeName = "src-pod-node"
	srcPodName     = "src-test-pod"
	srcPodIP       = "10.234.0.1"
	srcNamespace   = "src-namespace"
	srcRouteDst    = "10.245.0.1/32"

	srcTestPod *corev1.Pod
	srcReq     ctrl.Request
	srcRoute   *netlink.Route
	src        *SymmetricRoutingController
)

var _ = Describe("SymmetricRoutingOperator", func() {
	JustBeforeEach(func() {
		srcReq = ctrl.Request{
			NamespacedName: types.NamespacedName{
				Namespace: srcNamespace,
				Name:      srcPodName,
			},
		}
		// Reset the test pod to the default values.
		srcTestPod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      srcReq.Name,
				Namespace: srcReq.Namespace,
			},
			Spec: corev1.PodSpec{
				NodeName: srcPodNodeName,
				Containers: []corev1.Container{
					{
						Name:            "busybox",
						Image:           "busybox",
						ImagePullPolicy: corev1.PullIfNotPresent,
						Command: []string{
							"sleep",
							"3600",
						},
					},
				},
			},
		}
		gw := net.ParseIP(liqonetutils.GetOverlayIP(srcNodeIP))
		Expect(gw).NotTo(BeNil())
		_, dst, err := net.ParseCIDR(srcRouteDst)
		Expect(err).To(BeNil())
		srcRoute = &netlink.Route{
			LinkIndex: vxlanDevice.Link.Attrs().Index,
			Dst:       dst,
			Gw:        gw,
			Table:     srcRoutingTableID,
		}
		// Add route.
		err = netlink.RouteAdd(srcRoute)
		Expect(err).To(BeNil())

		// Create the symmetric routing operator.
		src = &SymmetricRoutingController{
			Client:         k8sClient,
			vxlanDev:       vxlanDevice,
			nodeName:       srcNodeName,
			routingTableID: srcRoutingTableID,
			nodesLock:      &sync.RWMutex{},
			vxlanNodes:     map[string]string{srcNodeName: srcNodeIP},
			routes:         map[string]string{},
		}
	})
	JustAfterEach(func() {
		err := netlink.RouteDel(srcRoute)
		if err != nil && !errors.Is(err, unix.ESRCH) {
			Expect(err).Should(BeNil())
		}

	})
	Describe("testing NewSymmetricRoutingOperator function", func() {
		Context("when input parameters are not correct", func() {
			It("vxlan device is not correct, should return nil and error", func() {
				src, err := NewSymmetricRoutingOperator(srcNodeName, srcRoutingTableID, nil, &sync.RWMutex{}, nil, k8sClient)
				Expect(err).Should(MatchError(&liqoerrors.WrongParameter{Parameter: "vxlanDevice", Reason: liqoerrors.NotNil}))
				Expect(src).Should(BeNil())
			})

			It("routingTableID parameter out of range: a negative number", func() {
				src, err := NewSymmetricRoutingOperator(srcNodeName, -244, vxlanDevice, &sync.RWMutex{}, nil, k8sClient)
				Expect(err).Should(Equal(&liqoerrors.WrongParameter{Parameter: "routingTableID", Reason: liqoerrors.GreaterOrEqual + strconv.Itoa(0)}))
				Expect(src).Should(BeNil())
			})

			It("routingTableID parameter out of range: superior to max value ", func() {
				src, err := NewSymmetricRoutingOperator(srcNodeName, unix.RT_TABLE_MAX+1, vxlanDevice, &sync.RWMutex{}, nil, k8sClient)
				Expect(err).Should(Equal(&liqoerrors.WrongParameter{Parameter: "routingTableID", Reason: liqoerrors.MinorOrEqual + strconv.Itoa(unix.RT_TABLE_MAX)}))
				Expect(src).Should(BeNil())
			})
		})

		Context("when input parameters are correct", func() {
			It("should return symmetrinc routing controller and nil", func() {
				src, err := NewSymmetricRoutingOperator(srcNodeName, srcRoutingTableID, vxlanDevice, &sync.RWMutex{}, nil, k8sClient)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(src).ShouldNot(BeNil())
			})
		})
	})

	Describe("testing reconcile function", func() {
		Context("adding route for new pod", func() {
			It("ip for destination node has not been set, should return error", func() {
				srcTestPod.Name = "add-route-no-gw-existing"
				srcReq.Name = "add-route-no-gw-existing"
				Eventually(func() error { return k8sClient.Create(context.TODO(), srcTestPod) }).Should(BeNil())
				newPod := &corev1.Pod{}
				Eventually(func() error { return k8sClient.Get(context.TODO(), srcReq.NamespacedName, newPod) }).Should(BeNil())
				newPod.Status.PodIP = "10.1.11.1"
				Eventually(func() error { return k8sClient.Status().Update(context.TODO(), newPod) }).Should(BeNil())
				// Make sure that the field has already been updated on the testing api-server.
				Eventually(func() error {
					err := k8sClient.Get(context.TODO(), srcReq.NamespacedName, newPod)
					if err != nil {
						return err
					}
					if newPod.Status.PodIP != "10.1.11.1" {
						return fmt.Errorf("pod ip has not been updated yet on the testing api-server")
					}
					return nil
				}).Should(BeNil())
				Eventually(func() error { _, err := src.Reconcile(context.TODO(), srcReq); return err }).Should(MatchError("ip not set"))
				_, ok := src.routes[srcReq.String()]
				Expect(ok).Should(BeFalse())
			})

			It("route does not exist, should add it and return nil", func() {
				srcTestPod.Name = "add-route-no-existing"
				srcTestPod.Spec.NodeName = srcNodeName
				srcReq.Name = "add-route-no-existing"
				Eventually(func() error { return k8sClient.Create(context.TODO(), srcTestPod) }).Should(BeNil())
				newPod := &corev1.Pod{}
				Eventually(func() error { return k8sClient.Get(context.TODO(), srcReq.NamespacedName, newPod) }).Should(BeNil())
				newPod.Status.PodIP = "10.1.11.1"
				Eventually(func() error { return k8sClient.Status().Update(context.TODO(), newPod) }).Should(BeNil())
				// Make sure that the field has already been updated on the testing api-server.
				Eventually(func() error {
					err := k8sClient.Get(context.TODO(), srcReq.NamespacedName, newPod)
					if err != nil {
						return err
					}
					if newPod.Status.PodIP != "10.1.11.1" {
						return fmt.Errorf("pod ip has not been updated yet on the testing api-server")
					}
					return nil
				}).Should(BeNil())
				Eventually(func() error { _, err := src.Reconcile(context.TODO(), srcReq); return err }).Should(BeNil())
				_, ok := src.routes[srcReq.String()]
				Expect(ok).Should(BeTrue())
			})
		})

		Context("removing route for a deleted pod", func() {
			It("route does not exist", func() {
				srcTestPod.Name = "del-route-no-existing"
				srcReq.Name = "del-route-no-existing"
				Eventually(func() error { _, err := src.Reconcile(context.TODO(), srcReq); return err }).Should(BeNil())
			})

			It("route does exist", func() {
				srcTestPod.Name = "del-route-existing"
				srcReq.Name = "del-route-existing"
				src.routes[srcReq.String()] = srcRouteDst
				Eventually(func() error { _, err := src.Reconcile(context.TODO(), srcReq); return err }).Should(BeNil())
				_, ok := src.routes[srcReq.String()]
				Expect(ok).Should(BeFalse())
			})
		})
	})

	Describe("testing addRoute function", func() {
		Context("when ip of the node where the pod runs has not been set", func() {
			It("should return false and error", func() {
				added, err := src.addRoute(srcReq, srcTestPod)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("ip not set"))
				Expect(added).Should(BeFalse())
			})
		})

		Context("when route does not exist", func() {
			It("should insert the route, return true and nil", func() {
				// Prepare pod with the right values.
				srcTestPod.Status.PodIP = srcPodIP
				// Insert node ip of the node where the testing pod is running.
				src.vxlanNodes[srcPodNodeName] = srcNodeIP
				// Add route.
				added, err := src.addRoute(srcReq, srcTestPod)
				Expect(err).NotTo(HaveOccurred())
				Expect(added).Should(BeTrue())
				_, ok := src.routes[srcReq.String()]
				Expect(ok).Should(BeTrue())
				_, dstNet, err := net.ParseCIDR(srcPodIP + "/32")
				Expect(err).To(BeNil())
				// List rules wit destination net the one of the pod.
				routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{Table: srcRoutingTableID, Dst: dstNet}, netlink.RT_FILTER_TABLE|netlink.RT_FILTER_DST)
				Expect(err).Should(BeNil())
				Expect(len(routes)).Should(BeNumerically("==", 1))
				Expect(routes[0].Dst.String()).Should(Equal(dstNet.String()))
				Expect(routes[0].Gw.String()).Should(Equal(liqonetutils.GetOverlayIP(srcNodeIP)))
			})
		})

		Context("when route does exist", func() {
			It("route remains the same, return false and nil", func() {
				// Prepare pod with the right values.
				tmpTokens := strings.Split(srcRouteDst, "/")
				srcTestPod.Status.PodIP = tmpTokens[0]
				// Insert node ip of the node where the testing pod is running.
				src.vxlanNodes[srcPodNodeName] = srcNodeIP
				// Add route.
				added, err := src.addRoute(srcReq, srcTestPod)
				Expect(err).NotTo(HaveOccurred())
				Expect(added).Should(BeFalse())
				_, ok := src.routes[srcReq.String()]
				Expect(ok).Should(BeTrue())
				// List rules wit destination net the one of the pod.
				routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, srcRoute, netlink.RT_FILTER_TABLE|netlink.RT_FILTER_DST)
				Expect(err).Should(BeNil())
				Expect(len(routes)).Should(BeNumerically("==", 1))
				Expect(routes[0].Dst.String()).Should(Equal(srcRouteDst))
				Expect(routes[0].Gw.String()).Should(Equal(liqonetutils.GetOverlayIP(srcNodeIP)))

			})
		})
	})

	Describe("testing delRoute function", func() {
		Context("when route does not exist", func() {
			It("should return false and nil", func() {
				deleted, err := src.delRoute(srcReq)
				Expect(err).NotTo(HaveOccurred())
				Expect(deleted).Should(BeFalse())
				_, ok := src.routes[srcReq.String()]
				Expect(ok).Should(BeFalse())
			})
		})

		Context("when route does exist", func() {
			It("should remove the route and return true and nil", func() {
				src.routes[srcReq.String()] = srcRouteDst
				deleted, err := src.delRoute(srcReq)
				Expect(err).NotTo(HaveOccurred())
				Expect(deleted).Should(BeTrue())
				_, ok := src.routes[srcReq.String()]
				Expect(ok).Should(BeFalse())
				routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, srcRoute, netlink.RT_FILTER_TABLE|netlink.RT_FILTER_DST)
				Expect(err).Should(BeNil())
				Expect(len(routes)).Should(BeNumerically("==", 0))
			})
		})
	})
})
