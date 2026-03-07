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

package exposition

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"sync"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	corev1listers "k8s.io/client-go/listers/core/v1"
	discoveryv1listers "k8s.io/client-go/listers/discovery/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/trace"
	"sigs.k8s.io/controller-runtime/pkg/client"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	offloadingv1beta1clients "github.com/liqotech/liqo/pkg/client/clientset/versioned/typed/offloading/v1beta1"
	ipamv1alpha1listers "github.com/liqotech/liqo/pkg/client/listers/ipam/v1alpha1"
	offloadingv1beta1listers "github.com/liqotech/liqo/pkg/client/listers/offloading/v1beta1"
	directconnectioninfo "github.com/liqotech/liqo/pkg/utils/directconnection"
	getters "github.com/liqotech/liqo/pkg/utils/getters"
	ipamutils "github.com/liqotech/liqo/pkg/utils/ipam"
	"github.com/liqotech/liqo/pkg/utils/virtualkubelet"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/generic"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

var _ manager.NamespacedReflector = (*NamespacedEndpointSliceReflector)(nil)

const (
	// EndpointSliceReflectorName -> The name associated with the EndpointSlice reflector.
	EndpointSliceReflectorName      = "EndpointSlice"
	maxAnnotationSize               = 256 * 1024 // 256 KiB
	directConnectionAnnotationLabel = "direct-connections-data"
	directConnectionServiceLabel    = "use-direct-connections"
)

// NamespacedEndpointSliceReflector manages the EndpointSlice reflection for a given pair of local and remote namespaces.
type NamespacedEndpointSliceReflector struct {
	generic.NamespacedReflector

	localNodeClient                  corev1listers.NodeLister
	localServices                    corev1listers.ServiceNamespaceLister
	localEndpointSlices              discoveryv1listers.EndpointSliceNamespaceLister
	remoteShadowEndpointSlices       offloadingv1beta1listers.ShadowEndpointSliceNamespaceLister
	remoteShadowEndpointSlicesClient offloadingv1beta1clients.ShadowEndpointSliceInterface
	localIPs                         ipamv1alpha1listers.IPNamespaceLister

	localPodCIDR *net.IPNet

	translations sync.Map
}

// NewEndpointSliceReflector returns a new EndpointSliceReflector instance.
func NewEndpointSliceReflector(localPodCIDR string, reflectorConfig *offloadingv1beta1.ReflectorConfig) manager.Reflector {
	return generic.NewReflector(EndpointSliceReflectorName, NewNamespacedEndpointSliceReflector(localPodCIDR),
		generic.WithoutFallback(), reflectorConfig.NumWorkers, reflectorConfig.Type, generic.ConcurrencyModeLeader)
}

// NewNamespacedEndpointSliceReflector returns a function generating NamespacedEndpointSliceReflector instances.
func NewNamespacedEndpointSliceReflector(localPodCIDR string) func(*options.NamespacedOpts) manager.NamespacedReflector {
	return func(opts *options.NamespacedOpts) manager.NamespacedReflector {
		localNode := opts.LocalFactory.Core().V1().Nodes()
		localServices := opts.LocalFactory.Core().V1().Services()
		localEndpointSlices := opts.LocalFactory.Discovery().V1().EndpointSlices()
		localIPs := opts.LocalLiqoFactory.Ipam().V1alpha1().IPs()
		remoteShadow := opts.RemoteLiqoFactory.Offloading().V1beta1().ShadowEndpointSlices()

		_, err := localEndpointSlices.Informer().AddEventHandler(opts.HandlerFactory(generic.NamespacedKeyer(opts.LocalNamespace)))
		utilruntime.Must(err)
		_, err = remoteShadow.Informer().AddEventHandler(opts.HandlerFactory(generic.NamespacedKeyer(opts.LocalNamespace)))
		utilruntime.Must(err)

		_, podCIDR, err := net.ParseCIDR(localPodCIDR)
		utilruntime.Must(err)

		ner := &NamespacedEndpointSliceReflector{
			NamespacedReflector:              generic.NewNamespacedReflector(opts, EndpointSliceReflectorName),
			localNodeClient:                  localNode.Lister(),
			localServices:                    localServices.Lister().Services(opts.LocalNamespace),
			localEndpointSlices:              localEndpointSlices.Lister().EndpointSlices(opts.LocalNamespace),
			remoteShadowEndpointSlices:       remoteShadow.Lister().ShadowEndpointSlices(opts.RemoteNamespace),
			remoteShadowEndpointSlicesClient: opts.RemoteLiqoClient.OffloadingV1beta1().ShadowEndpointSlices(opts.RemoteNamespace),
			localIPs:                         localIPs.Lister().IPs(opts.LocalNamespace),
			localPodCIDR:                     podCIDR,
		}

		// Enqueue all existing remote EndpointSlices in case the local Service has the "skip-reflection" annotation, to ensure they are also deleted.
		_, err = localServices.Informer().AddEventHandler(opts.HandlerFactory(ner.ServiceToEndpointSlicesKeyer))
		utilruntime.Must(err)

		return ner
	}
}

// Handle reconciles endpointslice objects.
func (ner *NamespacedEndpointSliceReflector) Handle(ctx context.Context, name string) error {
	tracer := trace.FromContext(ctx)

	// Skip the kubernetes service, as meaningless to reflect.
	if ner.LocalNamespace() == corev1.NamespaceDefault && name == kubernetesServiceName {
		klog.V(4).Infof("Skipping reflection of local EndpointSlice %q as blacklisted", ner.LocalRef(name))
		return nil
	}

	// Retrieve the local and remote objects (only not found errors can occur).
	klog.V(4).Infof("Handling reflection of local EndpointSlice %q (remote: %q)", ner.LocalRef(name), ner.RemoteRef(name))

	local, lerr := ner.localEndpointSlices.Get(name)
	utilruntime.Must(client.IgnoreNotFound(lerr))
	localExists := !kerrors.IsNotFound(lerr)

	remote, rerr := ner.remoteShadowEndpointSlices.Get(name)
	utilruntime.Must(client.IgnoreNotFound(rerr))
	remoteExists := !kerrors.IsNotFound(rerr)

	tracer.Step("Retrieved the local and remote objects")

	// Abort the reflection if the remote object is not managed by us, as we do not want to mutate others' objects.
	if remoteExists && (!forge.IsReflected(remote) || !forge.IsEndpointSliceManagedByReflection(remote)) {
		// Prevent misleading warnings triggered by remote non-reflected endpointslices, since they inherit
		// the labels from the service. Hence, vanilla remote endpointslices do also trigger the Handle function.
		if localExists {
			klog.Infof("Skipping reflection of local EndpointSlice %q as remote already exists and is not managed by us", ner.LocalRef(name))
			ner.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedReflectionAlreadyExistsMsg())
		}
		return nil
	}

	// Abort the reflection if the local object has the "skip-reflection" annotation.
	if localExists {
		skipReflection, err := ner.ShouldSkipReflection(local)
		if err != nil {
			klog.Errorf("Failed to check whether local Endpointslice %q should be reflected: %v", ner.LocalRef(name), err)
			return err
		}
		if skipReflection {
			if ner.GetReflectionType() == offloadingv1beta1.DenyList {
				klog.Infof("Skipping reflection of local EndpointSlice %q as marked with the skip annotation", ner.LocalRef(name))
			} else { // AllowList
				klog.Infof("Skipping reflection of local EndpointSlice %q as not marked with the allow annotation", ner.LocalRef(name))
			}
			ner.Event(local, corev1.EventTypeNormal, forge.EventReflectionDisabled, forge.EventObjectReflectionDisabledMsg(ner.GetReflectionType()))
			if !remoteExists { // The shadow object does not already exist, hence no further action is required.
				return nil
			}

			// Otherwise, let pretend the local object does not exist, so that the remote one gets deleted.
			localExists = false
		}
	}

	tracer.Step("Performed the sanity checks")

	// The local endpointslice does no longer exist. Ensure it is also absent from the remote cluster.
	if !localExists {
		defer tracer.Step("Ensured the absence of the remote object")
		if remoteExists {
			klog.V(4).Infof("Deleting remote shadowendpointslice %q, since local %q does no longer exist", ner.RemoteRef(name), ner.LocalRef(name))
			return ner.DeleteRemote(ctx, ner.remoteShadowEndpointSlicesClient, "ShadowEndpointSlice", name, remote.GetUID())
		}

		klog.V(4).Infof("Local EndpointSlice %q and remote EndpointSlice %q both vanished", ner.LocalRef(name), ner.RemoteRef(name))
		return nil
	}

	directConnectionsEnabled := ner.ShouldProvideDirectConnectionData(local)

	// Wrap the address translation logic, so that we do not have to handle errors in the forge logic.
	var terr error
	translator := func(originals []string) []string {
		// Avoid processing further addresses if one already failed.
		if terr != nil {
			return nil
		}

		var translations []string
		translations, terr = ner.MapEndpointIPs(name, originals, directConnectionsEnabled)
		return translations
	}

	var marshaledData []byte
	if directConnectionsEnabled {
		// If the service associated to the local Endpointslice is flagged with the "use-direct-connection": true annotation
		// then gather the data needed to make the providers use the direct connections among them.
		// 1) The address that needs to be remapped.
		// 2) ClusterID of the cluster on which that endpoint is running.

		var remoteConnectionsData directconnectioninfo.DirectConnectionData

		for _, endpoint := range local.Endpoints {
			if endpoint.NodeName == nil {
				continue
			}
			if endpoint.TargetRef == nil {
				klog.Errorf("Endpoint with IP %q has no targetRef, skipping direct connection data collection for it", endpoint.Addresses)
				continue
			}
			node, err := ner.localNodeClient.Get(*endpoint.NodeName)
			if err != nil {
				klog.Errorf("Failed getting the node %q: %v", *endpoint.NodeName, err)
				continue
			}

			// No data collection for nodes in the remote cluster or in the central cluster
			if !directconnectioninfo.ShouldIncludeDataFromNode(node, string(forge.RemoteCluster)) {
				continue
			}

			clusterID, err := getters.RetrieveRemoteClusterIDFromNode(node)
			if err != nil {
				klog.Errorf("Failed to retrieve remote cluster ID from node %q: %v", *endpoint.NodeName, err)
				continue
			}

			IPs := endpoint.Addresses
			remoteConnectionsData.Add(clusterID, IPs...)
		}
		if len(remoteConnectionsData.ByCluster) == 0 {
			klog.Errorf("Service is set for direct connections but no data found for this endpointslice: %s", local.Name)
		} else {
			var err error

			marshaledData, err = remoteConnectionsData.ToJSON()
			if err != nil {
				klog.Errorf("Failed to marshal direct connection data: %v", err)
			} else {
				klog.V(4).Infof("marshaled direct connection data: %s", string(marshaledData))
			}

			if len(marshaledData)+totalAnnotationsSize(local) >= maxAnnotationSize {
				// Max size of data in annotations is 256KB
				klog.Errorf("marshaled direct connection data exceeds maximum size of %d bytes: %d bytes", maxAnnotationSize, len(marshaledData))
				marshaledData = nil
			} else {
				klog.V(4).Infof("Direct connection data for endpointslice %q marshaled successfully", local.Name)
			}
		}
	}

	target := forge.RemoteShadowEndpointSlice(local, remote, ner.localNodeClient, ner.RemoteNamespace(), translator, ner.ForgingOpts)
	if terr != nil {
		klog.Errorf("Reflection of local EndpointSlice %q to %q failed: %v", ner.LocalRef(name), ner.RemoteRef(name), terr)
		ner.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedReflectionMsg(terr))
		return terr
	}
	
	if marshaledData != nil {
		target.Annotations[directConnectionAnnotationLabel] = string(marshaledData)
	}
	tracer.Step("Forged the remote shadowendpointslice")

	// If the remote shadowendpointslice does not exist, then create it.
	if !remoteExists {
		defer tracer.Step("Ensured the presence of the remote object")
		_, err := ner.remoteShadowEndpointSlicesClient.Create(ctx, target, metav1.CreateOptions{FieldManager: forge.ReflectionFieldManager})
		if err != nil {
			if kerrors.IsAlreadyExists(err) {
				klog.Infof("Remote shadowendpointslice %q already exists (local endpointslice: %q)", ner.RemoteRef(name), ner.LocalRef(name))
				return nil
			}
			klog.Errorf("Failed to create remote shadowendpointslice %q (local endpointslice: %q): %v", ner.RemoteRef(name), ner.LocalRef(name), err)
			if !kerrors.IsConflict(err) {
				ner.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedReflectionMsg(err))
			}
			return err
		}

		klog.Infof("Remote shadowendpointslice %q successfully created (local: %q)", ner.RemoteRef(name), ner.LocalRef(name))
		ner.Event(local, corev1.EventTypeNormal, forge.EventSuccessfulReflection, forge.EventSuccessfulReflectionMsg())
		tracer.Step("Created the remote shadowendpointslice")
		return nil
	}

	// If so, perform the actual update operation if needed.
	if ner.ShouldUpdateShadowEndpointSlice(ctx, remote, target) {
		_, err := ner.remoteShadowEndpointSlicesClient.Update(ctx, target, metav1.UpdateOptions{FieldManager: forge.ReflectionFieldManager})
		if err != nil {
			klog.Errorf("Failed to update remote shadowendpointslice %q (local endpointslice: %q): %v", ner.RemoteRef(name), ner.LocalRef(name), err)
			if !kerrors.IsConflict(err) {
				ner.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedReflectionMsg(err))
			}
			return err
		}

		klog.Infof("Remote shadowendpointslice %q successfully updated (local endpointslice: %q)", ner.RemoteRef(name), ner.LocalRef(name))
		ner.Event(local, corev1.EventTypeNormal, forge.EventSuccessfulReflection, forge.EventSuccessfulReflectionMsg())
		tracer.Step("Updated the remote shadowendpointslice")
	} else {
		klog.V(4).Infof("Skipping remote shadowendpointslice %q update, as already synced", ner.RemoteRef(name))
	}

	return nil
}

// ShouldUpdateShadowEndpointSlice checks whether it is necessary to update the remote shadowendpointslice, based on the forged one.
func (ner *NamespacedEndpointSliceReflector) ShouldUpdateShadowEndpointSlice(ctx context.Context,
	remote, target *offloadingv1beta1.ShadowEndpointSlice) bool {
	defer trace.FromContext(ctx).Step("Checked whether a shadowendpointslice update was needed")
	return !labels.Equals(remote.GetLabels(), target.GetLabels()) ||
		!labels.Equals(remote.GetAnnotations(), target.GetAnnotations()) ||
		!reflect.DeepEqual(remote.Spec.Template.AddressType, target.Spec.Template.AddressType) ||
		!reflect.DeepEqual(remote.Spec.Template.Endpoints, target.Spec.Template.Endpoints) ||
		!reflect.DeepEqual(remote.Spec.Template.Ports, target.Spec.Template.Ports)
}

// MapEndpointIPFromIPResource maps an IP string using an IP resource.
func (ner *NamespacedEndpointSliceReflector) MapEndpointIPFromIPResource(original string) (string, error) {
	ips, err := ner.localIPs.List(labels.Everything())
	if err != nil {
		return "", fmt.Errorf("failed to list IPs: %w", err)
	}
	for i := range ips {
		if ips[i].Spec.IP.String() == original {
			remappedIP := ipamutils.GetRemappedIP(ips[i])
			if remappedIP == "" {
				return "", fmt.Errorf("resource IP %q (%q) has not been mapped yet", ips[i].Name, ips[i].Spec.IP)
			}
			return remappedIP.String(), nil
		}
	}
	return original, fmt.Errorf("resource IP %s not found", original)
}

// MapEndpointIPs maps the local set of addresses to the corresponding remote ones.
func (ner *NamespacedEndpointSliceReflector) MapEndpointIPs(endpointslice string, originals []string, skipTranslation bool) ([]string, error) {
	
	if skipTranslation {
		klog.V(4).Infof("Skipping translation of endpoint IPs for EndpointSlice %q, beacuse direct connections are enabled.", ner.LocalRef(endpointslice))
		return originals, nil
	}
	
	var translations []string
	var err error

	// Retrieve the cache for the given endpointslice. The cache is not synchronized,
	// since we are guaranteed to be the only ones operating on this object.
	ucache, _ := ner.translations.LoadOrStore(endpointslice, map[string]string{})
	cache := ucache.(map[string]string)

	for _, original := range originals {
		// Check if we already know the translation.
		translation, found := cache[original]

		if !found {
			// Skip IPAM translation if direct connections are enabled
            if ner.localPodCIDR.Contains(net.ParseIP(original)) {
                translation = original
            } else {
                // Cache miss -> interact with IPAM for translation
                translation, err = ner.MapEndpointIPFromIPResource(original)
                if err != nil {
                    return nil, fmt.Errorf("failed to translate endpoint IP %v: %w", original, err)
                }
            }
            cache[original] = translation
		}

		translations = append(translations, translation)
		klog.V(6).Infof("Translated local endpoint IP %v to remote %v", original, translation)
	}

	return translations, nil
}

// ShouldSkipReflection returns whether the reflection of the given object should be skipped.
func (ner *NamespacedEndpointSliceReflector) ShouldSkipReflection(obj metav1.Object) (bool, error) {
	// Check if the endpointslice is explicitly marked to be skipped or allowed.
	// If so, we do not care about the reflection policy, as the annotation takes precedence.
	shouldSkip, err := ner.ForcedAllowOrSkip(obj)
	if err != nil {
		return true, err
	} else if shouldSkip != nil {
		return *shouldSkip, nil
	}

	// If no annotation are set, we consider the the standard reflection policy.
	// Note: it is inherited from the service reflector.

	// Check if a service is associated to the EndpointSlice, and whether it is marked to be skipped.
	svcname, ok := obj.GetLabels()[discoveryv1.LabelServiceName]
	if !ok {
		// If the endpointslice is not associated to a service, use standard logic to determine whetever or not
		// to reflect the endpointslice.
		return ner.NamespacedReflector.ShouldSkipReflection(obj)
	}

	svc, err := ner.localServices.Get(svcname)
	// Continue with the standard logic in case the service is not found, as this is likely due to a race conditions
	// (i.e., the service has not yet been cached). If necessary, the informer will trigger a re-enqueue,
	// thus performing once more this check.
	if err != nil {
		return ner.NamespacedReflector.ShouldSkipReflection(obj)
	}

	// The associated service exists, hence check if it is marked to be skipped.
	return ner.NamespacedReflector.ShouldSkipReflection(svc)
}

// ServiceToEndpointSlicesKeyer returns the NamespacedName of all local EndpointSlices associated with the given local Service.
func (ner *NamespacedEndpointSliceReflector) ServiceToEndpointSlicesKeyer(metadata metav1.Object) []types.NamespacedName {
	req, err := labels.NewRequirement(discoveryv1.LabelServiceName, selection.Equals, []string{metadata.GetName()})
	utilruntime.Must(err)
	eps, err := ner.localEndpointSlices.List(labels.NewSelector().Add(*req))
	utilruntime.Must(err)

	keys := make([]types.NamespacedName, 0, len(eps))
	keyer := generic.NamespacedKeyer(ner.LocalNamespace())
	for _, ep := range eps {
		keys = append(keys, keyer(ep)...)
	}

	return keys
}

// ShouldProvideDirectConnectionData returns whether the reflector should provide the data to make pods deployed on providers communicate directly
// (only in case a direct connection is established).
//
// It's done by reading an annotation in the Service associated to the given Endpointslice.
func (ner *NamespacedEndpointSliceReflector) ShouldProvideDirectConnectionData(obj metav1.Object) bool {
	// Check if a service is associated to the EndpointSlice
	svcname, ok := obj.GetLabels()[discoveryv1.LabelServiceName]
	if !ok {
		return false
	}
	// Retrieve the service from the local cache, done again after ShouldSkipReflection
	svc, err := ner.localServices.Get(svcname)
	if err != nil {
		return false
	}

	// Check if the service is marked to provide direct connection data
	return svc.GetAnnotations()[directConnectionServiceLabel] == "true"
}

// totalAnnotationsSize computes the total size in bytes of all annotation keys and values in the given object.
func totalAnnotationsSize(obj metav1.Object) int {
	annotations := obj.GetAnnotations()

	size := 0
	for k, v := range annotations {
		size += len(k) + len(v)
	}
	return size
}

// List returns the list of EndpointSlices managed by informers.
func (ner *NamespacedEndpointSliceReflector) List() ([]interface{}, error) {
	listEps, err := virtualkubelet.List[virtualkubelet.Lister[*discoveryv1.EndpointSlice], *discoveryv1.EndpointSlice](
		ner.localEndpointSlices,
	)
	if err != nil {
		return nil, err
	}
	listSeps, err := virtualkubelet.List[virtualkubelet.Lister[*offloadingv1beta1.ShadowEndpointSlice], *offloadingv1beta1.ShadowEndpointSlice](
		ner.remoteShadowEndpointSlices,
	)
	if err != nil {
		return nil, err
	}
	return append(listEps, listSeps...), nil
}
