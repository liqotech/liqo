package outgoing

import (
	"context"
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	ri "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/reflectorsInterfaces"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	discoveryv1beta1 "k8s.io/api/discovery/v1beta1"
	kerror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"
)

var endpointsliceLabels = map[string]string{
	"endpointslice.kubernetes.io/managed-by": "endpoint-reflector.liqo.io",
}

type EndpointSlicesReflector struct {
	ri.APIReflector

	LocalRemappedPodCIDR options.ReadOnlyOption
	VirtualNodeName      options.ReadOnlyOption
}

func (r *EndpointSlicesReflector) SetSpecializedPreProcessingHandlers() {
	r.SetPreProcessingHandlers(ri.PreProcessingHandlers{
		IsAllowed:  r.isAllowed,
		AddFunc:    r.PreAdd,
		UpdateFunc: r.PreUpdate,
		DeleteFunc: r.PreDelete})
}

func (r *EndpointSlicesReflector) HandleEvent(e interface{}) {
	var err error

	event := e.(watch.Event)
	cm, ok := event.Object.(*discoveryv1beta1.EndpointSlice)
	if !ok {
		klog.Error("REFLECTION: cannot cast object to EndpointSlice")
		return
	}
	key := r.Keyer(cm.Namespace, cm.Name)
	klog.V(3).Infof("REFLECTION: received %v for EndpointSlice %v", event.Type, key)

	switch event.Type {
	case watch.Added:
		if _, err := r.GetForeignClient().DiscoveryV1beta1().EndpointSlices(cm.Namespace).Create(context.TODO(), cm, metav1.CreateOptions{}); err != nil {
			klog.Errorf("REFLECTION: Error while creating the remote EndpointSlice %v - ERR: %v", key, err)
		}
		klog.V(3).Infof("REFLECTION: remote EndpointSlice %v correctly created", key)

	case watch.Modified:
		if _, err = r.GetForeignClient().DiscoveryV1beta1().EndpointSlices(cm.Namespace).Update(context.TODO(), cm, metav1.UpdateOptions{}); err != nil {
			klog.Errorf("REFLECTION: Error while updating the remote EndpointSlice %v - ERR: %v", key, err)
		} else {
			klog.V(3).Infof("REFLECTION: remote EndpointSlice %v correctly updated", key)
		}

	case watch.Deleted:
		if err := r.GetForeignClient().DiscoveryV1beta1().EndpointSlices(cm.Namespace).Delete(context.TODO(), cm.Name, metav1.DeleteOptions{}); err != nil {
			klog.Errorf("REFLECTION: Error while deleting the remote EndpointSlice %v - ERR: %v", key, err)
		} else {
			klog.V(3).Infof("REFLECTION: remote EndpointSlice %v correctly deleted", key)
		}
	}
}

func (r *EndpointSlicesReflector) PreAdd(obj interface{}) interface{} {
	epLocal := obj.(*discoveryv1beta1.EndpointSlice).DeepCopy()
	nattedNs, err := r.NattingTable().NatNamespace(epLocal.Namespace, false)
	if err != nil {
		klog.Error(err)
		return nil
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
		return kerror.IsNotFound(err)
	}
	fn := func() error {
		svc, err = r.GetForeignClient().CoreV1().Services(nattedNs).Get(context.TODO(), svcName, metav1.GetOptions{})
		return err
	}
	if err = retry.OnError(retry.DefaultBackoff, retriable, fn); err != nil {
		klog.Errorf("error while retrieving service %v in endppointslices reflector - ERR: %v", key, err)
		blacklist[apimgmt.EndpointSlices][key] = true
		return nil
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
		Endpoints:   filterEndpoints(epLocal, string(r.LocalRemappedPodCIDR.Value()), string(r.VirtualNodeName.Value())),
		Ports:       epLocal.Ports,
	}

	return epsRemote
}

func (r *EndpointSlicesReflector) PreUpdate(newObj, _ interface{}) interface{} {
	endpointSliceHome := newObj.(*discoveryv1beta1.EndpointSlice).DeepCopy()
	endpointSliceName := endpointSliceHome.Name

	nattedNs, err := r.NattingTable().NatNamespace(endpointSliceHome.Namespace, false)
	if err != nil {
		klog.Error(err)
		return nil
	}
	oldRemoteObj, err := r.GetCacheManager().GetForeignNamespacedObject(apimgmt.EndpointSlices, nattedNs, endpointSliceName)
	if err != nil {
		err = errors.Wrapf(err, "endpointslices %v/%v", nattedNs, endpointSliceName)
		klog.Error(err)
		return nil
	}
	RemoteEpSlice := oldRemoteObj.(*discoveryv1beta1.EndpointSlice).DeepCopy()

	RemoteEpSlice.SetNamespace(nattedNs)
	RemoteEpSlice.SetResourceVersion(RemoteEpSlice.ResourceVersion)
	RemoteEpSlice.SetUID(RemoteEpSlice.UID)

	RemoteEpSlice.Endpoints = filterEndpoints(endpointSliceHome, string(r.LocalRemappedPodCIDR.Value()), string(r.VirtualNodeName.Value()))
	RemoteEpSlice.Ports = endpointSliceHome.Ports

	return RemoteEpSlice
}

func (r *EndpointSlicesReflector) PreDelete(obj interface{}) interface{} {
	endpointSliceLocal := obj.(*discoveryv1beta1.EndpointSlice)
	nattedNs, err := r.NattingTable().NatNamespace(endpointSliceLocal.Namespace, false)
	if err != nil {
		klog.Error(err)
		return nil
	}
	endpointSliceLocal.Namespace = nattedNs

	return endpointSliceLocal
}

func filterEndpoints(slice *discoveryv1beta1.EndpointSlice, podCidr string, nodeName string) []discoveryv1beta1.Endpoint {
	var epList []discoveryv1beta1.Endpoint
	// Two possibilities: (1) exclude all virtual nodes (2)
	for _, v := range slice.Endpoints {
		t := v.Topology["kubernetes.io/hostname"]
		if t != nodeName {
			newEp := discoveryv1beta1.Endpoint{
				Addresses:  []string{forge.ChangePodIp(podCidr, v.Addresses[0])},
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
	foreignNamespace, err := r.NattingTable().NatNamespace(localNamespace, false)
	if err != nil {
		klog.Error(err)
		return
	}

	objects, err := r.GetCacheManager().ResyncListForeignNamespacedObject(apimgmt.EndpointSlices, foreignNamespace)
	if err != nil {
		klog.Error(err)
		return
	}

	retriable := func(err error) bool {
		switch kerror.ReasonForError(err) {
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

func (r *EndpointSlicesReflector) isAllowed(obj interface{}) bool {
	eps, ok := obj.(*discoveryv1beta1.EndpointSlice)
	if !ok {
		klog.Error("cannot convert obj to service")
		return false
	}
	key := r.Keyer(eps.Namespace, eps.Name)
	_, ok = blacklist[apimgmt.EndpointSlices][key]
	if ok {
		klog.V(4).Infof("endpointslice %v blacklisted", key)
	}
	return !ok
}
