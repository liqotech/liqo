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

package routeoperator

import (
	"context"
	"net"
	"sync"
	"syscall"

	"github.com/vishvananda/netlink"
	corev1 "k8s.io/api/core/v1"
	k8sApiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	liqoerrors "github.com/liqotech/liqo/pkg/liqonet/errors"
	"github.com/liqotech/liqo/pkg/liqonet/overlay"
	liqoutils "github.com/liqotech/liqo/pkg/liqonet/utils"
)

var (
	// vxlanMACAddressKey annotation key for the mac address of vxlan interface.
	vxlanMACAddressKey = "net.liqo.io/vxlan.mac.address"
)

// OverlayController reconciles pods objects, in our case the route operators pods.
type OverlayController struct {
	client.Client
	vxlanDev   *overlay.VxlanDevice
	podIP      string
	nodesLock  *sync.RWMutex
	vxlanPeers map[string]*overlay.Neighbor
	// For each nodeName contains its IP addr.
	vxlanNodes map[string]string
	// Given the namespace/podName it contains the pod name where the pod is running.
	podToNode map[string]string
}

// Reconcile for a given pod it checks if it is our pod or not. If it is our pod than annotates
// it with mac address of the current vxlan device. If it is a pod running in a different node
// then based on the type of event:
// event.Create/Update it adds the peer to the vxlan overlay network if it does not exist.
// event.Delete it removes the peer from the vxlan overlay network if it does exist.
func (ovc *OverlayController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var pod corev1.Pod
	var err error
	err = ovc.Get(ctx, req.NamespacedName, &pod)
	if err != nil && !k8sApiErrors.IsNotFound(err) {
		klog.Errorf("an error occurred while getting pod {%s}: %v", req.NamespacedName, err)
		return ctrl.Result{}, err
	}
	if k8sApiErrors.IsNotFound(err) {
		// Remove the peer.
		deleted, err := ovc.delPeer(req)
		if err != nil {
			return ctrl.Result{}, err
		}
		if deleted {
			klog.Infof("successfully removed peer {%s} from vxlan overlay network", req.String())
		}
		return ctrl.Result{}, nil
	}
	// If it is our pod than add the mac address annotation.
	if ovc.podIP == pod.Status.PodIP {
		if liqoutils.AddAnnotationToObj(&pod, vxlanMACAddressKey, ovc.vxlanDev.Link.HardwareAddr.String()) {
			if err := ovc.Update(ctx, &pod); err != nil {
				klog.Errorf("an error occurred while adding mac address annotation to pod {%s}: %v", req.String(), err)
				return ctrl.Result{}, err
			}
			klog.Infof("successfully annotated pod {%s} with mac address {%s}", req.String(), ovc.vxlanDev.Link.HardwareAddr.String())
		}
		return ctrl.Result{}, nil
	}

	// If it is not our pod, then add peer to the vxlan network.
	added, err := ovc.addPeer(req, &pod)
	if err != nil {
		klog.Errorf("an error occurred while adding peer {%s} with IP address {%s} and MAC address {%s} to the vxlan overlay network: %v",
			req.String(), pod.Status.PodIP, liqoutils.GetAnnotationValueFromObj(&pod, vxlanMACAddressKey), err)
		return ctrl.Result{}, err
	}
	if added {
		klog.Errorf("successfully added peer {%s} with IP address {%s} and MAC address {%s} to the vxlan overlay network",
			req.String(), pod.Status.PodIP, liqoutils.GetAnnotationValueFromObj(&pod, vxlanMACAddressKey))
	}
	return ctrl.Result{}, nil
}

// NewOverlayController returns a new controller ready to be setup and started with the controller manager.
func NewOverlayController(podIP string, vxlanDevice *overlay.VxlanDevice, nodesLock *sync.RWMutex,
	vxlanNodes map[string]string, cl client.Client) (*OverlayController, error) {
	if vxlanDevice == nil {
		return nil, &liqoerrors.WrongParameter{
			Reason:    liqoerrors.NotNil,
			Parameter: "vxlanDevice",
		}
	}
	return &OverlayController{
		Client:     cl,
		vxlanDev:   vxlanDevice,
		podIP:      podIP,
		nodesLock:  nodesLock,
		vxlanPeers: map[string]*overlay.Neighbor{},
		vxlanNodes: vxlanNodes,
		podToNode:  map[string]string{},
	}, nil
}

// addPeer for a given pod it adds the fdb entry for the current vxlan device.
// It return true when the entry does not exist and is added, false if the entry does already exist,
// and error if something goes wrong.
func (ovc *OverlayController) addPeer(req ctrl.Request, pod *corev1.Pod) (bool, error) {
	ovc.nodesLock.Lock()
	defer ovc.nodesLock.Unlock()
	peerIP := pod.Status.PodIP
	peerMAC := pod.GetAnnotations()[vxlanMACAddressKey]
	ip := net.ParseIP(peerIP)
	if ip == nil {
		return false, &liqoerrors.ParseIPError{IPToBeParsed: peerIP}
	}
	mac, err := net.ParseMAC(peerMAC)
	if err != nil {
		return false, err
	}
	peer := overlay.Neighbor{
		MAC: mac,
		IP:  ip,
	}

	added, err := ovc.vxlanDev.AddFDB(peer)
	if err != nil {
		return added, err
	}
	// This entry is needed for broadcast and multicast
	// traffic (e.g. ARP and IPv6 neighbor discovery).
	macZeros, err := net.ParseMAC("00:00:00:00:00:00")
	if err != nil {
		return false, err
	}
	peerZero := overlay.Neighbor{
		MAC: macZeros,
		IP:  ip,
	}
	addedZeros, err := ovc.vxlanDev.AddFDB(peerZero)
	if err != nil {
		return false, err
	}
	ovc.vxlanPeers[req.String()] = &peer
	ovc.vxlanNodes[pod.Spec.NodeName] = peerIP
	ovc.podToNode[req.String()] = pod.Spec.NodeName
	if added || addedZeros {
		return true, nil
	}
	return false, nil
}

// delPeer for a given pod it removes all the fdb entries for the current peer on the vxlan device.
// It return true when entries exist and are removed, false if entries do not exist,
// and error if something goes wrong.
func (ovc *OverlayController) delPeer(req ctrl.Request) (bool, error) {
	var deleted bool
	ovc.nodesLock.Lock()
	defer ovc.nodesLock.Unlock()
	peer, ok := ovc.vxlanPeers[req.String()]
	if !ok {
		return false, nil
	}
	// First we list all the fdbs
	fdbs, err := netlink.NeighList(ovc.vxlanDev.Link.Index, syscall.AF_BRIDGE)
	if err != nil {
		return false, err
	}
	// Check if the entry exists and remove them.
	for i := range fdbs {
		if fdbs[i].IP.Equal(peer.IP) {
			deleted, err = ovc.vxlanDev.DelFDB(overlay.Neighbor{
				MAC: fdbs[i].HardwareAddr,
				IP:  fdbs[i].IP,
			})
			if err != nil {
				return deleted, err
			}
			klog.V(4).Infof("fdb entry with mac {%s} and dst {%s} on device {%s} has been removed",
				fdbs[i].HardwareAddr.String(), fdbs[i].IP.String(), ovc.vxlanDev.Link.Name)
		}
	}
	delete(ovc.vxlanPeers, req.String())
	delete(ovc.vxlanNodes, ovc.podToNode[req.String()])
	delete(ovc.podToNode, req.String())
	return deleted, nil
}

// podFilter used to filter out all the pods that are not instances of the route operator
// daemon set. It checks that pods are route operator instances, and has the vxlanMACAddressKey
// annotation set or that the current pod we are considering is our same pod. In this case
// we add the vxlanMACAddressKey annotation in order for the other nodes to add as to the overlay network.
func (ovc *OverlayController) podFilter(obj client.Object) bool {
	// Check if the object is a pod.
	p, ok := obj.(*corev1.Pod)
	if !ok {
		klog.Infof("object {%s} is not of type corev1.Pod", obj.GetName())
		return false
	}
	// If it is our pod then process it.
	if ovc.podIP == p.Status.PodIP {
		return true
	}
	// If it is not our pod then check if the vxlan mac address has been set.
	annotations := obj.GetAnnotations()

	// Here we make sure that only objects with the annotation set can be reconciled.
	if _, ok := annotations[vxlanMACAddressKey]; ok {
		return true
	}
	klog.V(4).Infof("route-operator pod {%s} in namespace {%s} running on node {%s} does not have {%s} annotation set",
		p.Name, p.Namespace, p.Spec.NodeName, vxlanMACAddressKey)
	return false
}

// SetupWithManager used to set up the controller with a given manager.
func (ovc *OverlayController) SetupWithManager(mgr ctrl.Manager) error {
	p := predicate.NewPredicateFuncs(ovc.podFilter)
	return ctrl.NewControllerManagedBy(mgr).For(&corev1.Pod{}).WithEventFilter(p).
		Complete(ovc)
}
