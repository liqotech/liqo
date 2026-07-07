// Copyright 2019-2026 The Liqo Authors
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

package ipmapping

import (
	"context"
	"fmt"
	"strings"

	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	clientutils "github.com/liqotech/liqo/pkg/utils/clients"
	utilpredicates "github.com/liqotech/liqo/pkg/utils/predicates"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

const (
	kubernetesServiceName      = "kubernetes"
	kubernetesServiceNamespace = "default"
)

// EndpointSliceIPReconciler reconciles an EndpointSlice and create the corresponding IP resource.
type EndpointSliceIPReconciler struct {
	Client client.Client
	Scheme *runtime.Scheme
	Cache  cache.Cache

	labelsSetsFilter []labels.Set
	namespacesFilter []string
	ipNamespace      string
}

// NewEndpointSliceIPReconciler returns a new EndpointSliceIPReconciler.
// It initializes a dedicated cache scoped to the given namespaces and label sets for EndpointSlices.
func NewEndpointSliceIPReconciler(ctx context.Context, scheme *runtime.Scheme, mapper apimeta.RESTMapper,
	conf *rest.Config, ipNamespace string) (*EndpointSliceIPReconciler, error) {
	labelsSets := []labels.Set{{discoveryv1.LabelServiceName: kubernetesServiceName}}
	namespaces := []string{kubernetesServiceNamespace}

	// Create a dedicated cache for EndpointSlices in the specified namespaces.
	namespacesConfig := make(map[string]cache.Config, len(namespaces))
	for _, ns := range namespaces {
		namespacesConfig[ns] = cache.Config{}
	}
	cacheOptions := &cache.Options{
		Scheme: scheme,
		Mapper: mapper,
		ByObject: map[client.Object]cache.ByObject{
			&discoveryv1.EndpointSlice{}: {
				Namespaces: namespacesConfig,
			},
		},
	}

	epsClient, epsCache, err := clientutils.GetCachedClientAndCacheWithConfig(ctx, scheme, mapper, conf, cacheOptions)
	if err != nil {
		return nil, fmt.Errorf("creating endpointslice scoped client: %w", err)
	}

	return &EndpointSliceIPReconciler{
		Client: epsClient,
		Cache:  epsCache,
		Scheme: scheme,

		labelsSetsFilter: labelsSets,
		namespacesFilter: namespaces,
		ipNamespace:      ipNamespace,
	}, nil
}

// +kubebuilder:rbac:groups=discovery.k8s.io,resources=endpointslices,verbs=get;list;watch
// +kubebuilder:rbac:groups=ipam.liqo.io,resources=ips,verbs=get;list;watch;create;update;patch;delete

// Reconcile reconciles an EndpointSlice and creates the corresponding IP resource.
func (r *EndpointSliceIPReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var eps discoveryv1.EndpointSlice
	if err := r.Client.Get(ctx, req.NamespacedName, &eps); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("getting endpointslice %q: %w", req.NamespacedName, err)
	}

	klog.V(4).Infof("Reconciling EndpointSlice %q", req.String())

	// Common labels for all the IPs created from this EndpointSlice.
	commonLabels := map[string]string{
		consts.IPEndpointSliceNameLabelKey: eps.Name,
	}

	if !eps.DeletionTimestamp.IsZero() {
		if err := r.Client.DeleteAllOf(ctx, &ipamv1alpha1.IP{}, client.InNamespace(r.ipNamespace), client.MatchingLabels(commonLabels)); err != nil {
			return ctrl.Result{}, fmt.Errorf("deleting IPs for EndpointSlice %q: %w", req.NamespacedName, err)
		}
		return ctrl.Result{}, nil
	}

	ipLabels := commonLabels
	if eps.Labels[discoveryv1.LabelServiceName] == kubernetesServiceName {
		ipLabels[consts.IPTypeLabelKey] = consts.IPTypeAPIServer
	}

	annots := map[string]string{
		consts.PreinstalledAnnotKey: consts.PreinstalledAnnotValue,
	}

	expectedIPs := make(map[string]struct{})
	for _, endpoint := range eps.Endpoints {
		for _, addr := range endpoint.Addresses {
			name := forgeIPname(&eps, addr)
			expectedIPs[name] = struct{}{}
			op, err := r.ensureIP(ctx, name, r.ipNamespace, addr, ipLabels, annots)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("ensuring IP for EndpointSlice %q: %w", req.NamespacedName, err)
			}
			if op != controllerutil.OperationResultNone {
				klog.Infof("IP %q %s", name, op)
			}
		}
	}

	var ipList ipamv1alpha1.IPList
	if err := r.Client.List(ctx, &ipList, client.InNamespace(r.ipNamespace), client.MatchingLabels(commonLabels)); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing IPs for EndpointSlice %q: %w", req.NamespacedName, err)
	}
	for i := range ipList.Items {
		ip := &ipList.Items[i]
		if _, ok := expectedIPs[ip.Name]; ok {
			continue
		}
		err := r.Client.Delete(ctx, ip)
		if client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, fmt.Errorf("deleting stale IP %q for EndpointSlice %q: %w", ip.Name, req.NamespacedName, err)
		}
		if err == nil {
			klog.Infof("Deleted stale IP %q for EndpointSlice %q", ip.Name, req.NamespacedName)
		}
	}

	klog.V(4).Infof("Ensured IPs for EndpointSlice %q", req.String())
	return ctrl.Result{}, nil
}

// SetupWithManager registers the EndpointSliceAPIServerReconciler to the manager.
// The watch is restricted to EndpointSlices in the specified namespaces targeting the endpointslices
// with labels matching the labelsSets provided to the reconciler.
func (r *EndpointSliceIPReconciler) SetupWithManager(mgr ctrl.Manager) error {
	predicates := predicate.And(
		utilpredicates.NewTypedAnyNamespacePredicate[*discoveryv1.EndpointSlice](r.namespacesFilter),
		utilpredicates.NewTypedAnyLabelsSetPredicate[*discoveryv1.EndpointSlice](r.labelsSetsFilter),
		predicate.TypedGenerationChangedPredicate[*discoveryv1.EndpointSlice]{})

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlEndpointSlice).
		WatchesRawSource(source.Kind(r.Cache, &discoveryv1.EndpointSlice{},
			&handler.TypedEnqueueRequestForObject[*discoveryv1.EndpointSlice]{}, predicates)).
		Complete(r)
}

func forgeIPname(eps *discoveryv1.EndpointSlice, address string) string {
	// Replace dots with dashes to make the name DNS-1123 compliant.
	return eps.Name + "-" + strings.ReplaceAll(address, ".", "-")
}

func (r *EndpointSliceIPReconciler) ensureIP(ctx context.Context,
	name, namespace, endpoint string, lbls, annots map[string]string) (controllerutil.OperationResult, error) {
	ip := ipamv1alpha1.IP{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	return resource.CreateOrUpdate(ctx, r.Client, &ip, func() error {
		if ip.Labels == nil {
			ip.Labels = map[string]string{}
		}
		ip.Labels = labels.Merge(ip.Labels, lbls)

		if ip.Annotations == nil {
			ip.Annotations = map[string]string{}
		}
		ip.Annotations = labels.Merge(ip.Annotations, annots)

		ip.Spec.IP = networkingv1beta1.IP(endpoint)
		ip.Spec.Masquerade = ptr.To(true)

		return nil
	})
}
