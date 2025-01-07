// Copyright 2019-2025 The Liqo Authors
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

package ipctrl

import (
	"context"

	v1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

// handleAssociatedService creates, updates or deletes the service associated to the IP.
func (r *IPReconciler) handleAssociatedService(ctx context.Context, ip *ipamv1alpha1.IP) error {
	// Service associated to the IP
	svc := v1.Service{ObjectMeta: metav1.ObjectMeta{
		Name:      ip.Name,
		Namespace: ip.Namespace,
	}}
	svcMutateFn := func() error {
		svc.SetLabels(labels.Merge(svc.GetLabels(), ip.Spec.ServiceTemplate.Metadata.GetLabels()))
		svc.SetAnnotations(labels.Merge(svc.GetAnnotations(), ip.Spec.ServiceTemplate.Metadata.GetAnnotations()))
		svc.Spec = ip.Spec.ServiceTemplate.Spec
		return controllerutil.SetControllerReference(ip, &svc, r.Scheme)
	}

	// EndpointSlice associated to the Service
	eps := discoveryv1.EndpointSlice{ObjectMeta: metav1.ObjectMeta{
		Name:      ip.Name,
		Namespace: ip.Namespace,
	}}
	epsMutateFn := func() error {
		eps.SetLabels(labels.Merge(eps.GetLabels(), labels.Set{discoveryv1.LabelServiceName: svc.Name}))
		eps.AddressType = discoveryv1.AddressTypeIPv4
		eps.Endpoints = []discoveryv1.Endpoint{
			{
				Addresses: []string{ip.Spec.IP.String()},
				Conditions: discoveryv1.EndpointConditions{
					Ready: ptr.To(true),
				},
			},
		}
		var ports []discoveryv1.EndpointPort
		for i := range ip.Spec.ServiceTemplate.Spec.Ports {
			ports = append(ports, discoveryv1.EndpointPort{
				Name:        &ip.Spec.ServiceTemplate.Spec.Ports[i].Name,
				Protocol:    &ip.Spec.ServiceTemplate.Spec.Ports[i].Protocol,
				Port:        &ip.Spec.ServiceTemplate.Spec.Ports[i].Port,
				AppProtocol: ip.Spec.ServiceTemplate.Spec.Ports[i].AppProtocol,
			})
		}
		eps.Ports = ports

		return controllerutil.SetControllerReference(ip, &eps, r.Scheme)
	}

	// Create service and endpointslice if the template is defined
	if ip.Spec.ServiceTemplate != nil {
		if err := enforceResource(ctx, r.Client, &svc, svcMutateFn, "service"); err != nil {
			return err
		}
		if err := enforceResource(ctx, r.Client, &eps, epsMutateFn, "endpointslice"); err != nil {
			return err
		}
	} else {
		// Service spec is not defined, delete the associated service and endpointslices if previously created
		if err := ensureResourceAbsence(ctx, r.Client, &svc, "service"); err != nil {
			return err
		}
		if err := ensureResourceAbsence(ctx, r.Client, &eps, "endpointslice"); err != nil {
			return err
		}
	}

	return nil
}

// ensureAssociatedServiceAbsence ensures that the service associated to the IP (and its endpointslices) does not exist.
func (r *IPReconciler) ensureAssociatedServiceAbsence(ctx context.Context, ip *ipamv1alpha1.IP) error {
	svc := v1.Service{ObjectMeta: metav1.ObjectMeta{
		Name:      ip.Name,
		Namespace: ip.Namespace,
	}}
	if err := ensureResourceAbsence(ctx, r.Client, &svc, "service"); err != nil {
		return err
	}

	eps := discoveryv1.EndpointSlice{ObjectMeta: metav1.ObjectMeta{
		Name:      ip.Name,
		Namespace: ip.Namespace,
	}}
	if err := ensureResourceAbsence(ctx, r.Client, &eps, "endpointslice"); err != nil {
		return err
	}

	return nil
}

// enforceResource ensures that the given resource exists.
// It either creates or update the resource.
func enforceResource(ctx context.Context, cl client.Client, obj client.Object, mutateFn controllerutil.MutateFn, resourceKind string) error {
	op, err := resource.CreateOrUpdate(ctx, cl, obj, mutateFn)
	if err != nil {
		klog.Errorf("error while creating/updating %s %q (operation: %s): %v", resourceKind, obj.GetName(), op, err)
		return err
	}
	klog.Infof("%s %q correctly enforced (operation: %s)", resourceKind, obj.GetName(), op)
	return nil
}

// ensureResourceAbsence ensures that the given resource does not exist.
// If the resource does not exist, it does nothing.
func ensureResourceAbsence(ctx context.Context, cl client.Client, obj client.Object, resourceKind string) error {
	err := cl.Get(ctx, types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, obj)
	switch {
	case err != nil && !apierrors.IsNotFound(err):
		klog.Errorf("error while getting %s %q: %v", resourceKind, obj.GetName(), err)
		return err
	case apierrors.IsNotFound(err):
		// The resource does not exist, do nothing.
		klog.V(6).Infof("%s %q does not exist. Nothing to do", resourceKind, obj.GetName())
	default:
		if err := client.IgnoreNotFound(cl.Delete(ctx, obj)); err != nil {
			klog.Errorf("error while deleting %s %q: %v", resourceKind, obj.GetName(), err)
			return err
		}
		klog.Infof("%s %q correctly deleted", resourceKind, obj.GetName())
	}
	return nil
}
