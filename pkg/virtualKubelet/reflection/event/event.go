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

package event

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	netv1 "k8s.io/api/networking/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	discoveryv1listers "k8s.io/client-go/listers/discovery/v1"
	netv1listers "k8s.io/client-go/listers/networking/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/trace"
	"sigs.k8s.io/controller-runtime/pkg/client"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/utils/virtualkubelet"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/generic"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

var _ manager.NamespacedReflector = (*NamespacedEventReflector)(nil)

const (
	// EventReflectorName -> The name associated with the Event reflector.
	EventReflectorName = "Event"
)

// NamespacedEventReflector manages the Event reflection for a given pair of local and remote namespaces.
type NamespacedEventReflector struct {
	generic.NamespacedReflector

	localEvents       corev1listers.EventNamespaceLister
	remoteEvents      corev1listers.EventNamespaceLister
	localEventsClient v1.EventInterface

	localConfigMaps     corev1listers.ConfigMapNamespaceLister
	localSecrets        corev1listers.SecretNamespaceLister
	localEndpointSlices discoveryv1listers.EndpointSliceNamespaceLister
	localIngresses      netv1listers.IngressNamespaceLister
	localServices       corev1listers.ServiceNamespaceLister
	localPvcs           corev1listers.PersistentVolumeClaimNamespaceLister
	localPods           corev1listers.PodNamespaceLister
}

// NewEventReflector returns a new EventReflector instance.
func NewEventReflector(reflectorConfig *offloadingv1beta1.ReflectorConfig) manager.Reflector {
	return generic.NewReflector(EventReflectorName, NewNamespacedEventReflector, generic.WithoutFallback(),
		reflectorConfig.NumWorkers, reflectorConfig.Type, generic.ConcurrencyModeLeader)
}

// NewNamespacedEventReflector returns a new NamespacedEventReflector instance.
func NewNamespacedEventReflector(opts *options.NamespacedOpts) manager.NamespacedReflector {
	local := opts.LocalFactory.Core().V1().Events()
	remote := opts.RemoteFactory.Core().V1().Events()

	localConfigMaps := opts.LocalFactory.Core().V1().ConfigMaps()
	localSecrets := opts.LocalFactory.Core().V1().Secrets()
	localEndpointSlices := opts.LocalFactory.Discovery().V1().EndpointSlices()
	localIngresses := opts.LocalFactory.Networking().V1().Ingresses()
	localServices := opts.LocalFactory.Core().V1().Services()
	localPvcs := opts.LocalFactory.Core().V1().PersistentVolumeClaims()
	localPods := opts.LocalFactory.Core().V1().Pods()

	_, err := local.Informer().AddEventHandler(opts.HandlerFactory(generic.NamespacedKeyer(opts.LocalNamespace)))
	utilruntime.Must(err)
	_, err = remote.Informer().AddEventHandler(opts.HandlerFactory(generic.NamespacedKeyer(opts.LocalNamespace)))
	utilruntime.Must(err)

	return &NamespacedEventReflector{
		NamespacedReflector: generic.NewNamespacedReflector(opts, EventReflectorName),
		localEvents:         local.Lister().Events(opts.LocalNamespace),
		remoteEvents:        remote.Lister().Events(opts.RemoteNamespace),
		localEventsClient:   opts.LocalClient.CoreV1().Events(opts.LocalNamespace),

		localConfigMaps:     localConfigMaps.Lister().ConfigMaps(opts.LocalNamespace),
		localSecrets:        localSecrets.Lister().Secrets(opts.LocalNamespace),
		localEndpointSlices: localEndpointSlices.Lister().EndpointSlices(opts.LocalNamespace),
		localIngresses:      localIngresses.Lister().Ingresses(opts.LocalNamespace),
		localServices:       localServices.Lister().Services(opts.LocalNamespace),
		localPvcs:           localPvcs.Lister().PersistentVolumeClaims(opts.LocalNamespace),
		localPods:           localPods.Lister().Pods(opts.LocalNamespace),
	}
}

// Handle reconciles Event objects.
func (ner *NamespacedEventReflector) Handle(ctx context.Context, name string) error {
	tracer := trace.FromContext(ctx)

	// Retrieve the local and remote objects (only not found errors can occur).
	klog.V(4).Infof("Handling reflection of local Event %q (remote: %q)", ner.LocalRef(name), ner.RemoteRef(name))
	local, lerr := ner.localEvents.Get(name)
	utilruntime.Must(client.IgnoreNotFound(lerr))
	remote, rerr := ner.remoteEvents.Get(name)
	utilruntime.Must(client.IgnoreNotFound(rerr))
	tracer.Step("Retrieved the local and remote objects")

	// Abort the reflection if the local object is not managed by us, as we do not want to mutate others' objects.
	if lerr == nil && !forge.IsReflected(local) {
		if rerr == nil { // Do not output the warning event in case the event was triggered by the local object (i.e., the remote one does not exists).
			klog.Infof("Skipping reflection of remote Event %q as local already exists and is not managed by us", ner.RemoteRef(name))
		}
		return nil
	}

	// Abort the reflection if the remote object has the "skip-reflection" annotation.
	if !kerrors.IsNotFound(rerr) {
		skipReflection, err := ner.ShouldSkipReflection(remote)
		if err != nil {
			klog.Errorf("Failed to check whether remote event %q should be reflected: %v", ner.RemoteRef(name), err)
			return err
		}
		if skipReflection {
			if ner.GetReflectionType() == offloadingv1beta1.DenyList {
				klog.Infof("Skipping reflection of remote Event %q as marked with the skip annotation", ner.RemoteRef(name))
			} else { // AllowList
				klog.Infof("Skipping reflection of remote Event %q as not marked with the allow annotation", ner.RemoteRef(name))
			}
			if kerrors.IsNotFound(lerr) { // The remote object does not already exist, hence no further action is required.
				return nil
			}

			// Otherwise, let pretend the remote object does not exist, so that the local one gets deleted.
			rerr = kerrors.NewNotFound(corev1.Resource("Event"), local.GetName())
		}
	}

	tracer.Step("Performed the sanity checks")

	// The remote Event does no longer exist. Ensure it is also absent from the local cluster.
	if kerrors.IsNotFound(rerr) {
		defer tracer.Step("Ensured the absence of the local object")
		if !kerrors.IsNotFound(lerr) {
			klog.V(4).Infof("Deleting local Event %q, since remote %q does no longer exist", ner.LocalRef(name), ner.RemoteRef(name))
			return ner.DeleteLocal(ctx, ner.localEventsClient, EventReflectorName, name, local.GetUID())
		}

		klog.V(4).Infof("Local Event %q and remote Event %q both vanished", ner.LocalRef(name), ner.RemoteRef(name))
		return nil
	}

	// The local Event does not exist yet. Forge it and create it on the local cluster.
	if kerrors.IsNotFound(lerr) {
		defer tracer.Step("Ensured the presence of the local object")
		klog.V(4).Infof("Creating local Event %q, since remote %q does not exist yet", ner.LocalRef(name), ner.RemoteRef(name))

		localInvolvedObject, err := ner.getLocalObject(remote.InvolvedObject.Kind, remote.InvolvedObject.APIVersion, remote.InvolvedObject.Name)
		if err != nil {
			// Skip the reflection of the event if the involved object does not exist in the local cluster
			klog.V(4).Infof("Unable to get local object %q: %v", remote.InvolvedObject.Name, err)
			return nil
		}

		source := corev1.EventSource{
			Component: remote.Source.Component + " (remote)",
			Host:      remote.Source.Host,
		}

		e := corev1.Event{
			ObjectMeta: metav1.ObjectMeta{
				Name:        remote.Name,
				Namespace:   ner.LocalNamespace(),
				Labels:      labels.Merge(forge.FilterNotReflected(remote.GetLabels(), ner.ForgingOpts.LabelsNotReflected), forge.ReflectionLabels()),
				Annotations: forge.FilterNotReflected(remote.GetAnnotations(), ner.ForgingOpts.AnnotationsNotReflected),
			},
			InvolvedObject: corev1.ObjectReference{
				APIVersion: remote.InvolvedObject.APIVersion,
				Kind:       remote.InvolvedObject.Kind,
				Name:       remote.InvolvedObject.Name,
				Namespace:  ner.LocalNamespace(),
				FieldPath:  remote.InvolvedObject.FieldPath,
				UID:        localInvolvedObject.GetUID(),
			},
			Reason:              remote.Reason,
			Message:             remote.Message,
			Type:                remote.Type,
			Source:              source,
			FirstTimestamp:      remote.FirstTimestamp,
			LastTimestamp:       remote.LastTimestamp,
			Count:               remote.Count,
			EventTime:           remote.EventTime,
			Series:              remote.Series,
			Action:              remote.Action,
			Related:             remote.Related,
			ReportingController: remote.ReportingController,
			ReportingInstance:   remote.ReportingInstance,
		}

		_, err = ner.localEventsClient.Create(ctx, &e, metav1.CreateOptions{})
		if err != nil {
			klog.Errorf("Unable to create local Event %q: %v", ner.LocalRef(name), err)
			return err
		}
	}

	klog.Infof("Local Event %q successfully enforced (remote: %q)", ner.LocalRef(name), ner.RemoteRef(name))

	return nil
}

func (ner *NamespacedEventReflector) getLocalObject(kind, apiVersion, name string) (client.Object, error) {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return nil, err
	}

	switch {
	case gv.Group == corev1.GroupName && gv.Version == corev1.SchemeGroupVersion.Version && kind == "ConfigMap":
		return ner.localConfigMaps.Get(name)
	case gv.Group == corev1.GroupName && gv.Version == corev1.SchemeGroupVersion.Version && kind == "Secret":
		return ner.localSecrets.Get(name)
	case gv.Group == discoveryv1.GroupName && gv.Version == discoveryv1.SchemeGroupVersion.Version && kind == "EndpointSlice":
		return ner.localEndpointSlices.Get(name)
	case gv.Group == netv1.GroupName && gv.Version == netv1.SchemeGroupVersion.Version && kind == "Ingress":
		return ner.localIngresses.Get(name)
	case gv.Group == corev1.GroupName && gv.Version == corev1.SchemeGroupVersion.Version && kind == "Service":
		return ner.localServices.Get(name)
	case gv.Group == corev1.GroupName && gv.Version == corev1.SchemeGroupVersion.Version && kind == "PersistentVolumeClaim":
		return ner.localPvcs.Get(name)
	case gv.Group == corev1.GroupName && gv.Version == corev1.SchemeGroupVersion.Version && kind == "Pod":
		return ner.localPods.Get(name)
	default:
		return nil, fmt.Errorf("unable to get local object %q: kind %q and apiVersion %q not supported", name, kind, apiVersion)
	}
}

// List returns the list of objects.
func (ner *NamespacedEventReflector) List() ([]interface{}, error) {
	return virtualkubelet.List[virtualkubelet.Lister[*corev1.Event], *corev1.Event](
		ner.localEvents,
		ner.remoteEvents,
	)
}
