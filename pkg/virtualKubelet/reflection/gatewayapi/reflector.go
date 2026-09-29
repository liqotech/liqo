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

package gatewayapi

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"k8s.io/utils/trace"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"
	gwclient "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
	gwinformers "sigs.k8s.io/gateway-api/pkg/client/informers/externalversions"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/utils/virtualkubelet"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/generic"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

// Scheme is the scheme used to record the events concerning the Gateway API objects, which are not part of the client-go one.
var Scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(Scheme))
	utilruntime.Must(gwv1.Install(Scheme))
}

// parentGatewaysIndex is the name of the index of the local routes, keyed by the Gateways they are attached to.
const parentGatewaysIndex = "liqo.io/parent-gateways"

// Config groups the parameters of the Gateway API reflectors.
type Config struct {
	// Support is the type of reflection supported, depending on the availability of the resource in both clusters.
	Support Support
	// GatewaysAvailable is whether the Gateway resource is available in the local cluster,
	// which is required to attach the reflected routes to the reflected Gateways.
	GatewaysAvailable bool

	// SharedGateway is the Gateway offered by the remote cluster the reflected routes are attached to, if any.
	SharedGateway *types.NamespacedName
	// VirtualGatewayClass is the name of the local GatewayClass whose Gateways are reflected.
	VirtualGatewayClass string
	// RemoteGatewayClass is the name of the GatewayClass offered by the remote cluster, if any.
	RemoteGatewayClass string
}

// object is the constraint satisfied by the Gateway API types handled by the reflection.
type object interface {
	metav1.Object
	runtime.Object
}

// namespaceLister retrieves the objects of a given namespace from the informer cache.
type namespaceLister[O object] interface {
	Get(name string) (O, error)
	List(selector labels.Selector) ([]O, error)
}

// namespaceClient mutates the objects of a given namespace.
type namespaceClient[A any, O object] interface {
	Apply(ctx context.Context, obj A, opts metav1.ApplyOptions) (O, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
}

// kind groups the kind-specific operations required to reflect a given type of Gateway API resource.
type kind[O object, A any] struct {
	// Name is the name of the reflector, which is also used as kind in the log messages.
	Name string
	// GroupResource identifies the resource in the log and event messages.
	GroupResource schema.GroupResource

	Informer func(factory gwinformers.SharedInformerFactory) cache.SharedIndexInformer
	Lister   func(factory gwinformers.SharedInformerFactory, namespace string) namespaceLister[O]
	Client   func(client gwclient.Interface, namespace string) namespaceClient[A, O]
	Forge    func(local O, targetNamespace string, opts *forge.GatewayAPIForgingOpts, forgingOpts *forge.ForgingOpts) (A, []string, error)

	// ParentGateways returns the Gateways the given object is attached to. It is nil for resources other than routes.
	ParentGateways func(obj O) []types.NamespacedName
	// StatusReflector returns the reflector of the status of the remote objects. It is nil for resources without status.
	StatusReflector func(opts *options.NamespacedOpts, forgingOpts *forge.GatewayAPIForgingOpts) statusReflector[O]
}

// NamespacedReflector manages the reflection of a given type of Gateway API resource for a given pair of local and remote namespaces.
// Local objects are retrieved from cluster-wide informers, shared by all namespaces, while remote objects from namespaced informers.
type NamespacedReflector[O object, A any] struct {
	generic.NamespacedReflector

	kind *kind[O, A]

	localObjects  namespaceLister[O]
	remoteObjects namespaceLister[O]
	// remoteClient is nil if the reflection is degraded, i.e., the resource cannot be reflected to the remote cluster.
	remoteClient namespaceClient[A, O]
	// degradedMessage is the message of the events generated when the reflection is degraded.
	degradedMessage string
	// status reflects the status of the remote objects back to the local cluster. It is nil for resources without status.
	status statusReflector[O]

	forgingOpts forge.GatewayAPIForgingOpts
}

func newReflector[O object, A any](k *kind[O, A], reflectorConfig *offloadingv1beta1.ReflectorConfig, cfg *Config) manager.Reflector {
	return generic.NewReflector(k.Name, newNamespacedReflector(k, cfg), newClusterFallback(k, cfg),
		reflectorConfig.NumWorkers, reflectorConfig.Type, generic.ConcurrencyModeLeader)
}

func newNamespacedReflector[O object, A any](k *kind[O, A], cfg *Config) func(*options.NamespacedOpts) manager.NamespacedReflector {
	return func(opts *options.NamespacedOpts) manager.NamespacedReflector {
		reflector := &NamespacedReflector[O, A]{
			NamespacedReflector: generic.NewNamespacedReflector(opts, k.Name),
			kind:                k,
			localObjects:        k.Lister(opts.LocalGatewayFactory, opts.LocalNamespace),
			degradedMessage:     forge.EventGatewayAPIUnavailableMsg(k.GroupResource.String(), "remote"),
			forgingOpts: forge.GatewayAPIForgingOpts{
				Mapper:              forge.NamespaceMapper(opts.NamespaceMapper),
				SharedGateway:       cfg.SharedGateway,
				IsReflectedGateway:  isReflectedGateway(opts.LocalGatewayFactory, cfg),
				VirtualGatewayClass: cfg.VirtualGatewayClass,
				RemoteGatewayClass:  cfg.RemoteGatewayClass,
			},
		}

		if k.StatusReflector != nil {
			reflector.status = k.StatusReflector(opts, &reflector.forgingOpts)
		}

		// The events shall refer to the Gateway API objects, and identify the virtual node generating them,
		// to simplify troubleshooting in case the namespace is offloaded to multiple clusters.
		reflector.EventRecorder = opts.EventBroadcaster.NewRecorder(Scheme, corev1.EventSource{
			Component: "liqo-" + strings.ToLower(k.Name) + "-reflection", Host: forge.LiqoNodeName})

		if cfg.Support != SupportFull {
			return reflector
		}

		if !canListAndWatch(opts.RemoteClient, k.GroupResource, opts.RemoteNamespace) {
			// Starting the remote informer would block the reflection of the whole namespace, as the cache would never sync.
			klog.Warningf("Not allowed to list and watch %s in remote namespace %q: reflection towards cluster %q degraded",
				k.GroupResource, opts.RemoteNamespace, forge.RemoteCluster)
			reflector.degradedMessage = forge.EventGatewayAPIForbiddenMsg(k.GroupResource.String())
			return reflector
		}

		remote := k.Informer(opts.RemoteGatewayFactory)
		_, err := remote.AddEventHandler(opts.HandlerFactory(generic.NamespacedKeyer(opts.LocalNamespace)))
		utilruntime.Must(err)
		reflector.remoteObjects = k.Lister(opts.RemoteGatewayFactory, opts.RemoteNamespace)
		reflector.remoteClient = k.Client(opts.RemoteGatewayClient, opts.RemoteNamespace)

		return reflector
	}
}

// isReflectedGateway returns a function checking whether the given local Gateway belongs to the virtual GatewayClass.
func isReflectedGateway(factory gwinformers.SharedInformerFactory, cfg *Config) func(namespace, name string) bool {
	if !cfg.GatewaysAvailable {
		return func(_, _ string) bool { return false }
	}

	gateways := factory.Gateway().V1().Gateways().Lister()
	return func(namespace, name string) bool {
		gateway, err := gateways.Gateways(namespace).Get(name)
		return err == nil && string(gateway.Spec.GatewayClassName) == cfg.VirtualGatewayClass
	}
}

// Handle reconciles the objects of the given kind.
func (nr *NamespacedReflector[O, A]) Handle(ctx context.Context, name string) error {
	if nr.remoteClient == nil {
		return nr.handleDegraded(ctx, name)
	}

	tracer := trace.FromContext(ctx)

	// Retrieve the local and remote objects (only not found errors can occur).
	klog.V(4).Infof("Handling reflection of local %s %q (remote: %q)", nr.kind.Name, nr.LocalRef(name), nr.RemoteRef(name))
	local, lerr := nr.localObjects.Get(name)
	utilruntime.Must(client.IgnoreNotFound(lerr))
	remote, rerr := nr.remoteObjects.Get(name)
	utilruntime.Must(client.IgnoreNotFound(rerr))
	tracer.Step("Retrieved the local and remote objects")

	// Forge the mutation to be applied to the remote cluster, as it also determines whether the object is managed by the reflection.
	var mutation A
	var warnings []string
	var ferr error
	if lerr == nil {
		mutation, warnings, ferr = nr.kind.Forge(local, nr.RemoteNamespace(), &nr.forgingOpts, nr.ForgingOpts)
		if errors.Is(ferr, forge.ErrNotManaged) {
			// Let pretend the local object does not exist, so that the remote one (if previously reflected) gets deleted.
			klog.V(4).Infof("Local %s %q is not managed by the reflection", nr.kind.Name, nr.LocalRef(name))
			lerr = kerrors.NewNotFound(nr.kind.GroupResource, name)
		}
	}

	// Abort the reflection if the remote object is not managed by us, as we do not want to mutate others' objects.
	if rerr == nil && !forge.IsReflected(remote) {
		if lerr == nil { // Do not output the warning event in case the event was triggered by the remote object (i.e., the local one does not exists).
			klog.Infof("Skipping reflection of local %s %q as remote already exists and is not managed by us", nr.kind.Name, nr.LocalRef(name))
			nr.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedReflectionAlreadyExistsMsg())
			return nr.reportFailure(ctx, local, "an object with the same name, not managed by Liqo, already exists")
		}
		return nil
	}

	// Abort the reflection if the local object has the "skip-reflection" annotation.
	if !kerrors.IsNotFound(lerr) {
		skipReflection, err := nr.ShouldSkipReflection(local)
		if err != nil {
			klog.Errorf("Failed to check whether local %s %q should be reflected: %v", nr.kind.Name, nr.LocalRef(name), err)
			return err
		}
		if skipReflection {
			klog.Infof("Skipping reflection of local %s %q as not allowed by the %q reflection policy",
				nr.kind.Name, nr.LocalRef(name), nr.GetReflectionType())
			nr.Event(local, corev1.EventTypeNormal, forge.EventReflectionDisabled, forge.EventObjectReflectionDisabledMsg(nr.GetReflectionType()))
			if kerrors.IsNotFound(rerr) { // The remote object does not already exist, hence no further action is required.
				return nil
			}

			// Otherwise, let pretend the local object does not exist, so that the remote one gets deleted.
			lerr = kerrors.NewNotFound(nr.kind.GroupResource, name)
		}
	}

	tracer.Step("Performed the sanity checks")

	// The local object does no longer exist (or it shall not be reflected). Ensure it is also absent from the remote cluster.
	if kerrors.IsNotFound(lerr) {
		defer tracer.Step("Ensured the absence of the remote object")
		return nr.ensureRemoteAbsence(ctx, name, remote, rerr)
	}

	var notReflectable *forge.ErrNotReflectable
	switch {
	case errors.As(ferr, &notReflectable):
		// The object cannot be reflected as is: ensure the remote copy (if any) is removed, as it would be outdated.
		reason := strings.Join(append([]string{ferr.Error()}, warnings...), "; ")
		klog.Warningf("Local %s %q cannot be reflected: %v", nr.kind.Name, nr.LocalRef(name), reason)
		nr.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventReflectionNotPossibleMsg(reason))
		if err := nr.deleteRemote(ctx, name, remote, rerr); err != nil {
			return err
		}
		return nr.reportFailure(ctx, local, "not reflected: "+reason)
	case ferr != nil:
		klog.Errorf("Failed to forge remote %s %q (local: %q): %v", nr.kind.Name, nr.RemoteRef(name), nr.LocalRef(name), ferr)
		nr.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedReflectionMsg(ferr))
		return errors.Join(ferr, nr.reportFailure(ctx, local, "reflection failed: "+ferr.Error()))
	}
	tracer.Step("Remote mutation created")

	defer tracer.Step("Enforced the correctness of the remote object")
	if _, err := nr.remoteClient.Apply(ctx, mutation, forge.ApplyOptions()); err != nil {
		klog.Errorf("Failed to enforce remote %s %q (local: %q): %v", nr.kind.Name, nr.RemoteRef(name), nr.LocalRef(name), err)
		nr.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedReflectionMsg(err))
		return errors.Join(err, nr.reportFailure(ctx, local, "reflection failed: "+err.Error()))
	}

	klog.Infof("Remote %s %q successfully enforced (local: %q)", nr.kind.Name, nr.RemoteRef(name), nr.LocalRef(name))

	// Reflect the status of the remote object back to the local cluster, once it has been created.
	if nr.status != nil && rerr == nil {
		if err := nr.status.Enforce(ctx, local, remote); err != nil {
			klog.Errorf("Failed to reflect the status of remote %s %q (local: %q): %v", nr.kind.Name, nr.RemoteRef(name), nr.LocalRef(name), err)
			nr.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedStatusReflectionMsg(err))
			return err
		}
	}

	if len(warnings) > 0 {
		nr.Event(local, corev1.EventTypeWarning, forge.EventPartialReflection, forge.EventPartialReflectionMsg(warnings))
		return nil
	}
	nr.Event(local, corev1.EventTypeNormal, forge.EventSuccessfulReflection, forge.EventSuccessfulReflectionMsg())
	return nil
}

// ensureRemoteAbsence deletes the remote object, if it exists, as well as the shadow resource reporting its status.
func (nr *NamespacedReflector[O, A]) ensureRemoteAbsence(ctx context.Context, name string, remote O, rerr error) error {
	if err := nr.deleteStatus(ctx, name); err != nil {
		return err
	}
	return nr.deleteRemote(ctx, name, remote, rerr)
}

// deleteRemote deletes the remote object, if it exists.
func (nr *NamespacedReflector[O, A]) deleteRemote(ctx context.Context, name string, remote O, rerr error) error {
	if kerrors.IsNotFound(rerr) {
		klog.V(4).Infof("Remote %s %q already absent", nr.kind.Name, nr.RemoteRef(name))
		return nil
	}

	klog.V(4).Infof("Deleting remote %s %q, since local %q shall not be reflected", nr.kind.Name, nr.RemoteRef(name), nr.LocalRef(name))
	return nr.DeleteRemote(ctx, nr.remoteClient, nr.kind.Name, name, remote.GetUID())
}

// deleteStatus deletes the shadow resource reporting the status of the remote object, if any.
func (nr *NamespacedReflector[O, A]) deleteStatus(ctx context.Context, name string) error {
	if nr.status == nil {
		return nil
	}
	if err := nr.status.Delete(ctx, name); err != nil {
		klog.Errorf("Failed to delete the status of remote %s %q: %v", nr.kind.Name, nr.RemoteRef(name), err)
		return err
	}
	return nil
}

// reportFailure reports, through the shadow resource, that the given local object could not be reflected,
// so that the failure is visible in the status of the local object, in addition to the events.
func (nr *NamespacedReflector[O, A]) reportFailure(ctx context.Context, local O, message string) error {
	if nr.status == nil {
		return nil
	}
	if err := nr.status.Fail(ctx, local, message); err != nil {
		klog.Errorf("Failed to report the reflection failure of local %s %q: %v", nr.kind.Name, klog.KObj(local), err)
		return err
	}
	return nil
}

// handleDegraded notifies that the local object cannot be reflected, as the resource cannot be reflected to the remote cluster.
func (nr *NamespacedReflector[O, A]) handleDegraded(ctx context.Context, name string) error {
	local, err := nr.localObjects.Get(name)
	if kerrors.IsNotFound(err) {
		// Ensure no stale status is reported (e.g., in case the resource was previously reflected).
		return nr.deleteStatus(ctx, name)
	}
	utilruntime.Must(err)

	// Objects not managed by the reflection (e.g., Gateways of other classes) are silently ignored.
	if _, _, err := nr.kind.Forge(local, nr.RemoteNamespace(), &nr.forgingOpts, nr.ForgingOpts); errors.Is(err, forge.ErrNotManaged) {
		return nr.deleteStatus(ctx, name)
	}

	skipReflection, err := nr.ShouldSkipReflection(local)
	if err != nil {
		klog.Errorf("Failed to check whether local %s %q should be reflected: %v", nr.kind.Name, nr.LocalRef(name), err)
		return err
	}
	if skipReflection {
		return nr.deleteStatus(ctx, name)
	}

	klog.Warningf("Local %s %q cannot be reflected: %s", nr.kind.Name, nr.LocalRef(name), nr.degradedMessage)
	nr.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, nr.degradedMessage)
	return nr.reportFailure(ctx, local, nr.degradedMessage)
}

// Cleanup deletes the shadow resources reporting the status of the objects reflected to the remote cluster,
// when the reflection of the namespace is stopped.
func (nr *NamespacedReflector[O, A]) Cleanup(ctx context.Context, _, _ string) error {
	if nr.status == nil {
		return nil
	}
	return nr.status.Cleanup(ctx)
}

// List returns the list of objects to be reflected.
func (nr *NamespacedReflector[O, A]) List() ([]interface{}, error) {
	if nr.remoteClient == nil {
		return virtualkubelet.List[namespaceLister[O], O](nr.localObjects)
	}
	return virtualkubelet.List[namespaceLister[O], O](nr.localObjects, nr.remoteObjects)
}

// canListAndWatch returns whether the given client is allowed to list and watch the given resource in the given namespace.
// Permissions in the remote namespaces are granted by the Liqo instance installed in the remote cluster, which might not
// support the reflection of the given resource. Any error is conservatively considered as a denial.
func canListAndWatch(kubeClient kubernetes.Interface, resource schema.GroupResource, namespace string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, verb := range []string{"list", "watch"} {
		review := &authorizationv1.SelfSubjectAccessReview{Spec: authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace: namespace, Verb: verb, Group: resource.Group, Resource: resource.Resource,
			},
		}}

		result, err := kubeClient.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, review, metav1.CreateOptions{})
		if err != nil {
			klog.Errorf("Failed to check whether allowed to %s %s in remote namespace %q: %v", verb, resource, namespace, err)
			return false
		}
		if !result.Status.Allowed {
			return false
		}
	}
	return true
}

// clusterFallback leverages the fallback mechanism of the generic reflector to register the event handlers on the
// cluster-wide local informers, which are shared by all namespaces. Objects in namespaces not reflected are ignored.
type clusterFallback[O object, A any] struct {
	kind    *kind[O, A]
	factory gwinformers.SharedInformerFactory
}

var _ manager.FallbackReflector = (*clusterFallback[*gwv1.HTTPRoute, any])(nil)

func newClusterFallback[O object, A any](k *kind[O, A], cfg *Config) generic.FallbackReflectorFactoryFunc {
	return func(opts *options.ReflectorOpts) manager.FallbackReflector {
		// Enqueue the local objects in the namespaces currently reflected.
		_, err := k.Informer(opts.LocalGatewayFactory).AddEventHandler(opts.HandlerFactory(mappedKeyer(opts.NamespaceMapper)))
		utilruntime.Must(err)

		// Enqueue the local routes attached to a Gateway whenever it changes (e.g., its class is modified),
		// since that determines whether the routes are attached to the reflected Gateway or to the shared one.
		if k.ParentGateways != nil && cfg.GatewaysAvailable {
			routes := k.Informer(opts.LocalGatewayFactory)
			utilruntime.Must(routes.AddIndexers(cache.Indexers{parentGatewaysIndex: parentGatewaysIndexFunc(k.ParentGateways)}))

			gateways := opts.LocalGatewayFactory.Gateway().V1().Gateways().Informer()
			_, err = gateways.AddEventHandler(opts.HandlerFactory(parentGatewaysKeyer(routes.GetIndexer(), opts.NamespaceMapper)))
			utilruntime.Must(err)
		}

		return &clusterFallback[O, A]{kind: k, factory: opts.LocalGatewayFactory}
	}
}

// Handle ignores the objects in the namespaces not reflected.
func (cf *clusterFallback[O, A]) Handle(_ context.Context, _ types.NamespacedName) error { return nil }

// Keys returns the keys of the local objects in the given namespace, which are enqueued when the reflection starts,
// since the events generated by the cluster-wide informers before that moment have been filtered out.
func (cf *clusterFallback[O, A]) Keys(local, _ string) []types.NamespacedName {
	objects, err := cf.kind.Lister(cf.factory, local).List(labels.Everything())
	utilruntime.Must(err)

	keys := make([]types.NamespacedName, 0, len(objects))
	for _, obj := range objects {
		keys = append(keys, types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetName()})
	}
	return keys
}

// Ready returns whether the fallback is ready, which is always true, as the cluster-wide informers are synced before starting.
func (cf *clusterFallback[O, A]) Ready() bool { return true }

// List returns no objects, as the ones in the namespaces reflected are listed by the namespaced reflectors.
func (cf *clusterFallback[O, A]) List() ([]interface{}, error) { return nil, nil }

// mappedKeyer returns a keyer enqueuing the objects in the namespaces currently reflected.
func mappedKeyer(mapper options.NamespaceMapper) options.Keyer {
	return func(metadata metav1.Object) []types.NamespacedName {
		if _, found := mapper(metadata.GetNamespace()); !found {
			return nil
		}
		return []types.NamespacedName{{Namespace: metadata.GetNamespace(), Name: metadata.GetName()}}
	}
}

// parentGatewaysKeyer returns a keyer enqueuing the routes attached to a given Gateway, in the namespaces currently reflected.
func parentGatewaysKeyer(routes cache.Indexer, mapper options.NamespaceMapper) options.Keyer {
	return func(metadata metav1.Object) []types.NamespacedName {
		objects, err := routes.ByIndex(parentGatewaysIndex, types.NamespacedName{Namespace: metadata.GetNamespace(), Name: metadata.GetName()}.String())
		if err != nil {
			klog.Errorf("Failed to retrieve the routes attached to Gateway %q: %v", klog.KObj(metadata), err)
			return nil
		}

		var keys []types.NamespacedName
		for _, obj := range objects {
			route, ok := obj.(metav1.Object)
			if !ok {
				continue
			}
			if _, found := mapper(route.GetNamespace()); found {
				keys = append(keys, types.NamespacedName{Namespace: route.GetNamespace(), Name: route.GetName()})
			}
		}
		return keys
	}
}

// parentGatewaysIndexFunc returns the index function keying the routes by the Gateways they are attached to.
func parentGatewaysIndexFunc[O object](parents func(obj O) []types.NamespacedName) cache.IndexFunc {
	return func(obj interface{}) ([]string, error) {
		route, ok := obj.(O)
		if !ok {
			return nil, fmt.Errorf("unexpected object type %T", obj)
		}

		var keys []string
		for _, parent := range parents(route) {
			keys = append(keys, parent.String())
		}
		return keys, nil
	}
}

// routeParentGateways returns the Gateways the given route is attached to.
func routeParentGateways(namespace string, spec *gwv1.CommonRouteSpec) []types.NamespacedName {
	var gateways []types.NamespacedName
	for i := range spec.ParentRefs {
		parent := &spec.ParentRefs[i]
		if (parent.Group != nil && *parent.Group != gwv1.GroupName) || (parent.Kind != nil && *parent.Kind != "Gateway") {
			continue
		}

		gateway := types.NamespacedName{Namespace: namespace, Name: string(parent.Name)}
		if parent.Namespace != nil {
			gateway.Namespace = string(*parent.Namespace)
		}
		gateways = append(gateways, gateway)
	}
	return gateways
}
