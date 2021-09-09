// Copyright 2019-2021 The Liqo Authors
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

package outgoing

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	discoveryv1beta1 "k8s.io/api/discovery/v1beta1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"

	liqonetIpam "github.com/liqotech/liqo/pkg/liqonet/ipam"
	"github.com/liqotech/liqo/pkg/utils"
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors"
	ri "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/reflectorsInterfaces"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options"
)

var endpointsliceLabels = map[string]string{
	"endpointslice.kubernetes.io/managed-by": "endpoint-reflector.liqo.io",
}

type EndpointSlicesReflector struct {
	ri.APIReflector

	VirtualNodeName options.ReadOnlyOption
	IpamClient      liqonetIpam.IpamClient
}

func (r *EndpointSlicesReflector) SetSpecializedPreProcessingHandlers() {
	r.SetPreProcessingHandlers(ri.PreProcessingHandlers{
		IsAllowed:  r.isAllowed,
		AddFunc:    r.PreAdd,
		UpdateFunc: r.PreUpdate,
		DeleteFunc: r.PreDelete})
}

func (r *EndpointSlicesReflector) HandleEvent(e interface{}) {
	event := e.(watch.Event)
	ep, ok := event.Object.(*discoveryv1beta1.EndpointSlice)
	if !ok {
		klog.Error("REFLECTION: cannot cast object to EndpointSlice")
		return
	}
	key := r.Keyer(ep.Namespace, ep.Name)
	klog.V(3).Infof("REFLECTION: received %v for EndpointSlice %v", event.Type, key)

	switch event.Type {
	case watch.Added:
		_, err := r.GetForeignClient().DiscoveryV1beta1().EndpointSlices(ep.Namespace).Create(context.TODO(), ep, metav1.CreateOptions{})
		if kerrors.IsAlreadyExists(err) {
			klog.V(4).Infof("REFLECTION: The remote endpointslices %v/%v has not been created because already existing", ep.Namespace, ep.Name)
			break
		}
		if err != nil {
			klog.Errorf("REFLECTION: Error while creating the remote EndpointSlice %v - ERR: %v", key, err)
		} else {
			klog.V(3).Infof("REFLECTION: remote EndpointSlice %v correctly created", key)
		}

	case watch.Modified:
		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			_, newErr := r.GetForeignClient().DiscoveryV1beta1().EndpointSlices(ep.Namespace).Update(context.TODO(), ep, metav1.UpdateOptions{})
			return newErr
		}); err != nil {
			klog.Errorf("REFLECTION: Error while updating the remote EndpointSlice %v - ERR: %v", key, err)
		} else {
			klog.V(3).Infof("REFLECTION: remote EndpointSlice %v correctly updated", key)
		}

	case watch.Deleted:
		if err := r.GetForeignClient().DiscoveryV1beta1().EndpointSlices(ep.Namespace).Delete(context.TODO(), ep.Name, metav1.DeleteOptions{}); err != nil {
			klog.Errorf("REFLECTION: Error while deleting the remote EndpointSlice %v - ERR: %v", key, err)
		} else {
			klog.V(3).Infof("REFLECTION: remote EndpointSlice %v correctly deleted", key)
		}
	}
}

func (r *EndpointSlicesReflector) PreAdd(obj interface{}) (interface{}, watch.EventType) {
	epLocal := obj.(*discoveryv1beta1.EndpointSlice).DeepCopy()
	nattedNs, err := r.NattingTable().NatNamespace(epLocal.Namespace)
	if err != nil {
		klog.Error(err)
		return nil, watch.Added
	}

	labels := map[string]string{
		"kubernetes.io/service-name": epLocal.Labels["kubernetes.io/service-name"],
	}
	for k, v := range endpointsliceLabels {
		labels[k] = v
	}

	var svc *corev1.Service
	svcName := epLocal.OwnerReferences[0].Name
	key := r.Keyer(nattedNs, svcName)

	retriable := func(err error) bool {
		return kerrors.IsNotFound(err)
	}
	fn := func() error {
		svc, err = r.GetForeignClient().CoreV1().Services(nattedNs).Get(context.TODO(), svcName, metav1.GetOptions{})
		return err
	}
	if err = retry.OnError(retry.DefaultBackoff, retriable, fn); err != nil {
		klog.Errorf("error while retrieving service %v in endpointslices reflector - ERR: %v", key, err)
		reflectors.Blacklist[apimgmt.EndpointSlices][key] = struct{}{}
		return nil, watch.Added
	}

	svcOwnerRef := []metav1.OwnerReference{
		{
			APIVersion: "v1",
			Kind:       "Service",
			Name:       svcName,
			UID:        svc.UID,
		},
	}

	epsRemote := &discoveryv1beta1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:            epLocal.Name,
			Namespace:       nattedNs,
			Labels:          labels,
			OwnerReferences: svcOwnerRef,
		},
		AddressType: discoveryv1beta1.AddressTypeIPv4,
		Endpoints:   filterEndpoints(epLocal, r.IpamClient, string(r.VirtualNodeName.Value())),
		Ports:       epLocal.Ports,
	}

	return epsRemote, watch.Added
}

func (r *EndpointSlicesReflector) PreUpdate(newObj, _ interface{}) (interface{}, watch.EventType) {
	endpointSliceHome := newObj.(*discoveryv1beta1.EndpointSlice).DeepCopy()
	endpointSliceName := endpointSliceHome.Name

	nattedNs, err := r.NattingTable().NatNamespace(endpointSliceHome.Namespace)
	if err != nil {
		klog.Error(err)
		return nil, watch.Modified
	}
	oldRemoteObj, err := r.GetCacheManager().GetForeignNamespacedObject(apimgmt.EndpointSlices, nattedNs, endpointSliceName)
	if kerrors.IsNotFound(err) {
		klog.Info("endpointslices preupdate routine: calling preAdd...")
		return r.PreAdd(newObj)
	}

	if err != nil {
		err = errors.Wrapf(err, "endpointslices %v/%v", nattedNs, endpointSliceName)
		klog.Error(err)
		return nil, watch.Modified
	}
	RemoteEpSlice := oldRemoteObj.(*discoveryv1beta1.EndpointSlice).DeepCopy()

	RemoteEpSlice.Endpoints = filterEndpoints(endpointSliceHome, r.IpamClient, string(r.VirtualNodeName.Value()))
	RemoteEpSlice.Ports = endpointSliceHome.Ports

	return RemoteEpSlice, watch.Modified
}

func (r *EndpointSlicesReflector) PreDelete(obj interface{}) (interface{}, watch.EventType) {
	clusterID := strings.TrimPrefix(string(r.VirtualNodeName.Value()), "liqo-")
	endpointSliceLocal := obj.(*discoveryv1beta1.EndpointSlice)
	nattedNs, err := r.NattingTable().NatNamespace(endpointSliceLocal.Namespace)
	if err != nil {
		klog.Error(err)
		return nil, watch.Deleted
	}
	endpointSliceLocal.Namespace = nattedNs

	for _, endpoint := range endpointSliceLocal.Endpoints {
		_, err := r.IpamClient.UnmapEndpointIP(context.Background(), &liqonetIpam.UnmapRequest{ClusterID: clusterID, Ip: endpoint.Addresses[0]})
		if err != nil {
			klog.Error(err)
		}
	}

	return endpointSliceLocal, watch.Deleted
}

func filterEndpoints(slice *discoveryv1beta1.EndpointSlice, ipamClient liqonetIpam.IpamClient, nodeName string) []discoveryv1beta1.Endpoint {
	var epList []discoveryv1beta1.Endpoint
	// Two possibilities: (1) exclude all virtual nodes (2)
	for _, v := range slice.Endpoints {
		t := v.Topology["kubernetes.io/hostname"]
		if t != nodeName {
			response, err := ipamClient.MapEndpointIP(context.Background(),
				&liqonetIpam.MapRequest{ClusterID: utils.GetClusterIDFromNodeName(nodeName), Ip: v.Addresses[0]})
			if err != nil {
				klog.Error(err)
			}
			newEp := discoveryv1beta1.Endpoint{
				Addresses:  []string{response.GetIp()},
				Conditions: v.Conditions,
				Hostname:   nil,
				TargetRef:  nil,
				Topology:   nil,
			}
			epList = append(epList, newEp)
		}
	}
	return epList
}

func (r *EndpointSlicesReflector) CleanupNamespace(localNamespace string) {
	foreignNamespace, err := r.NattingTable().NatNamespace(localNamespace)
	if err != nil {
		klog.Error(err)
		return
	}

	objects, err := r.GetCacheManager().ListForeignNamespacedObject(apimgmt.EndpointSlices, foreignNamespace)
	if err != nil {
		klog.Error(err)
		return
	}

	retriable := func(err error) bool {
		switch kerrors.ReasonForError(err) {
		case metav1.StatusReasonNotFound:
			return false
		default:
			klog.Warningf("retrying while deleting endpointslice because of- ERR; %v", err)
			return true
		}
	}
	for _, obj := range objects {
		eps := obj.(*discoveryv1beta1.EndpointSlice)
		if err := retry.OnError(retry.DefaultBackoff, retriable, func() error {
			return r.GetForeignClient().DiscoveryV1beta1().EndpointSlices(foreignNamespace).Delete(context.TODO(), eps.Name, metav1.DeleteOptions{})
		}); err != nil {
			klog.Errorf("Error while deleting remote endpointslice %v/%v", eps.Namespace, eps.Name)
		}
	}
}

func (r *EndpointSlicesReflector) isAllowed(_ context.Context, obj interface{}) bool {
	eps, ok := obj.(*discoveryv1beta1.EndpointSlice)
	if !ok {
		klog.Error("cannot convert obj to service")
		return false
	}
	key := r.Keyer(eps.Namespace, eps.Name)
	_, ok = reflectors.Blacklist[apimgmt.EndpointSlices][key]
	if ok {
		klog.V(4).Infof("endpointslice %v blacklisted", key)
	}
	return !ok
}
