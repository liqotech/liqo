package api

import (
"context"
	corev1 "k8s.io/api/core/v1"
	discoveryv1beta1 "k8s.io/api/discovery/v1beta1"
metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
"k8s.io/apimachinery/pkg/watch"
"k8s.io/klog"
)

type EndpointSlicesReflector struct {
	GenericAPIReflector
}

func (r *EndpointSlicesReflector) SetPreProcessingHandlers() {
	r.PreProcessingHandlers = PreProcessingHandlers{
		addFunc:    r.PreAdd,
		updateFunc: r.PreUpdate,
		deleteFunc: r.PreDelete,
	}
}

func (r *EndpointSlicesReflector) HandleEvent(e interface{}) {
	var err error

	event := e.(watch.Event)
	cm, ok := event.Object.(*discoveryv1beta1.EndpointSlice)
	if !ok {
		klog.Error("REFLECTION: cannot cast object to EndpointSlice")
		return
	}
	klog.V(3).Infof("REFLECTION: received %v for EndpointSlice %v", event.Type, cm.Name)

	switch event.Type {
	case watch.Added:
		if _, err := r.ForeignClient.DiscoveryV1beta1().EndpointSlices(cm.Namespace).Create(context.TODO(), cm, metav1.CreateOptions{}); err != nil {
			klog.Errorf("REFLECTION: Error while creating the remote EndpointSlice %v/%v - ERR: %v", cm.Namespace, cm.Name, err)
		}
		klog.V(3).Infof("REFLECTION: remote EndpointSlice %v-%v correctly created", cm.Namespace, cm.Name)

	case watch.Modified:
		if _, err = r.ForeignClient.DiscoveryV1beta1().EndpointSlices(cm.Namespace).Update(context.TODO(), cm, metav1.UpdateOptions{}); err != nil {
			klog.Errorf("REFLECTION: Error while updating the remote EndpointSlice %v/%v - ERR: %v", cm.Namespace, cm.Name, err)
		}
		klog.V(3).Infof("REFLECTION: remote EndpointSlice %v-%v correctly updated", cm.Namespace, cm.Name)

	case watch.Deleted:
		if err := r.ForeignClient.DiscoveryV1beta1().EndpointSlices(cm.Namespace).Delete(context.TODO(), cm.Name, metav1.DeleteOptions{}); err != nil {
			klog.Errorf("REFLECTION: Error while deleting the remote EndpointSlice %v/%v - ERR: %v", cm.Namespace, cm.Name, err)
		}
		klog.V(3).Infof("REFLECTION: remote EndpointSlice %v-%v correctly deleted", cm.Namespace, cm.Name)
	}
}

func (r *EndpointSlicesReflector) PreAdd(obj interface{}) interface{} {
	epLocal := obj.(discoveryv1beta1.EndpointSlice)
	nattedNs, err := r.NamespaceNatting.NatNamespace(epLocal.Namespace, false)
	if err != nil {
		klog.Error(err)
		return nil
	}



	epsRemote := discoveryv1beta1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      epLocal.Name,
			Namespace: nattedNs,
			Labels:    make(map[string]string),
		},
		AddressType: discoveryv1beta1.AddressTypeIPv4,
        Endpoints: filterEndpoints(&epLocal),
        Ports: epLocal.Ports,

	}

	return epsRemote
}

func (r *EndpointSlicesReflector) PreUpdate(newObj, _ interface{}) interface{} {
	cmNewLocal := newObj.(discoveryv1beta1.EndpointSlice)

	nattedNs, err := r.NamespaceNatting.NatNamespace(cmNewLocal.Namespace, false)
	if err != nil {
		klog.Error(err)
		return nil
	}

	cmOldRemote, err := r.ForeignClient.DiscoveryV1beta1().EndpointSlices(nattedNs).Get(context.TODO(), cmNewLocal.Name, metav1.GetOptions{})
	if err != nil {
		klog.Error(err)
		return nil
	}
	cmNewLocal.SetNamespace(nattedNs)
	cmNewLocal.SetResourceVersion(cmOldRemote.ResourceVersion)
	cmNewLocal.SetUID(cmOldRemote.UID)

	return newObj
}

func (r *EndpointSlicesReflector) PreDelete(obj interface{}) interface{} {
	cmLocal := obj.(discoveryv1beta1.EndpointSlice)
	nattedNs, err := r.NamespaceNatting.NatNamespace(cmLocal.Namespace, false)
	if err != nil {
		klog.Error(err)
		return nil
	}
	cmLocal.Namespace = nattedNs

	return cmLocal
}

func filterEndpoints(slice *discoveryv1beta1.EndpointSlice) []discoveryv1beta1.Endpoint {
	var epList []discoveryv1beta1.Endpoint
	// Two possibilities: (1) exclude all virtual nodes (2)
    myVirtualNode := "liqo-6575d0b9-6fba-4f7d-b890-a6417009cb64"
	for _, v := range slice.Endpoints {
       if v.Topology["kubernetes.io/hostname:"] != myVirtualNode {
			v := discoveryv1beta1.Endpoint{
				Addresses:  ChangePodIp("10.0.0.0/16", v.Addresses[0]),
				Conditions: v.Conditions,
				Hostname:   nil,
				TargetRef:  nil,
				Topology:   map[string]string {
					"kubernetes.io/hostname": "my-cluster-id",
			},
			}
		   epList = append(epList, v)
	   }
	}
	return epList
}

func ChangePodIp(cidr string,ip string) []string{
	return ip
}