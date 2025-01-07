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

package manager

import (
	"context"
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"k8s.io/utils/trace"

	liqoclient "github.com/liqotech/liqo/pkg/client/clientset/versioned"
	liqoinformers "github.com/liqotech/liqo/pkg/client/informers/externalversions"
	traceutils "github.com/liqotech/liqo/pkg/utils/trace"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

var _ Manager = (*manager)(nil)

// manager is an object managing the reflection of objects between the local and the remote cluster.
type manager struct {
	sync.Mutex

	local            kubernetes.Interface
	remote           kubernetes.Interface
	localLiqo        liqoclient.Interface
	remoteLiqo       liqoclient.Interface
	resync           time.Duration
	eventBroadcaster record.EventBroadcaster

	reflectors              []Reflector
	localPodInformerFactory informers.SharedInformerFactory

	namespaceHandler NamespaceHandler

	started bool
	stop    map[string]context.CancelFunc

	forgingOpts forge.ForgingOpts
}

// New returns a new manager to start the reflection towards a remote cluster.
func New(local, remote kubernetes.Interface, localLiqo, remoteLiqo liqoclient.Interface, resync time.Duration,
	eb record.EventBroadcaster, forgingOpts *forge.ForgingOpts) Manager {
	// Configure the field selector to retrieve only the pods scheduled on the current virtual node.
	localPodTweakListOptions := func(opts *metav1.ListOptions) {
		opts.FieldSelector = fields.OneTermEqualSelector("spec.nodeName", forge.LiqoNodeName).String()
	}

	return &manager{
		local:            local,
		remote:           remote,
		localLiqo:        localLiqo,
		remoteLiqo:       remoteLiqo,
		resync:           resync,
		eventBroadcaster: eb,

		reflectors: make([]Reflector, 0),
		localPodInformerFactory: informers.NewSharedInformerFactoryWithOptions(local, resync,
			informers.WithTweakListOptions(localPodTweakListOptions)),

		started: false,
		stop:    make(map[string]context.CancelFunc),

		forgingOpts: ptr.Deref(forgingOpts, forge.NewEmptyForgingOpts()),
	}
}

// With registers the given reflector to the manager.
func (m *manager) With(reflector Reflector) Manager {
	if m.started {
		panic("Attempted to register a new reflector while already running")
	}

	m.reflectors = append(m.reflectors, reflector)
	return m
}

func (m *manager) WithNamespaceHandler(handler NamespaceHandler) Manager {
	if m.started {
		panic("Attempted to register a namespace event handler while already running")
	}

	m.namespaceHandler = handler
	return m
}

// Start starts the reflection manager. It panics if executed twice.
func (m *manager) Start(ctx context.Context) {
	if m.started {
		panic("Attempted to start the reflection manager while already running")
	}

	klog.Info("Starting the reflection manager...")
	ready := false
	for _, reflector := range m.reflectors {
		opts := options.New(m.local, m.localPodInformerFactory.Core().V1().Pods()).
			WithReadinessFunc(func() bool { return ready }).WithEventBroadcaster(m.eventBroadcaster)
		reflector.Start(ctx, opts)
	}

	// This is a no-op in case no informers/listers have been retrieved.
	m.localPodInformerFactory.Start(ctx.Done())
	m.localPodInformerFactory.WaitForCacheSync(ctx.Done())

	m.started = true

	if m.namespaceHandler != nil {
		m.namespaceHandler.Start(ctx, m)
	} else {
		klog.Warningf("Starting reflection manager without namespace handler")
	}

	// Set the reflector readiness flag after all namespaced reflectors are started so that the fallback reflectors
	// do not process resources that are intended to be handled by namespaced reflectors.
	ready = true

	go func() {
		<-ctx.Done()
		for _, stop := range m.stop {
			stop()
		}
	}()
}

// StartNamespace starts the reflection for a given namespace.
func (m *manager) StartNamespace(local, remote string) {
	m.Lock()
	defer m.Unlock()

	if !m.started {
		panic(fmt.Errorf(
			"attempted to start the reflection between local namespace %q and remote namespace %q but the manager is not running", local, remote))
	}

	klog.Infof("Starting reflection between local namespace %q and remote namespace %q", local, remote)
	if _, found := m.stop[local]; found {
		klog.Warningf("Reflection between local namespace %q and remote namespace %q already started", local, remote)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.stop[local] = cancel

	// The local informer factories, which select all resources in the given namespace.
	localFactory := informers.NewSharedInformerFactoryWithOptions(m.local, m.resync, informers.WithNamespace(local))
	localLiqoFactory := liqoinformers.NewSharedInformerFactoryWithOptions(m.localLiqo, m.resync, liqoinformers.WithNamespace(local))

	// The remote informer factories, which select all resources in the given namespace.
	// We do not filter the resources by label selector, to be able to abort reflection in case the remote object already exists.
	remoteFactory := informers.NewSharedInformerFactoryWithOptions(m.remote, m.resync, informers.WithNamespace(remote))
	remoteLiqoFactory := liqoinformers.NewSharedInformerFactoryWithOptions(m.remoteLiqo, m.resync, liqoinformers.WithNamespace(remote))

	ready := false
	for _, reflector := range m.reflectors {
		opts := options.NewNamespaced().
			WithLocal(local, m.local, localFactory).WithLiqoLocal(m.localLiqo, localLiqoFactory).
			WithRemote(remote, m.remote, remoteFactory).WithLiqoRemote(m.remoteLiqo, remoteLiqoFactory).
			WithReadinessFunc(func() bool { return ready }).WithEventBroadcaster(m.eventBroadcaster).
			WithForgingOpts(&m.forgingOpts)
		reflector.StartNamespace(opts)
	}

	// The initialization is executed in a separate go routine, as cache synchronization might require some time to complete.
	go func() {
		tracer := trace.New("Initialization", trace.Field{Key: "LocalNamespace", Value: local}, trace.Field{Key: "RemoteNamespace", Value: remote})
		defer tracer.LogIfLong(traceutils.LongThreshold())

		// Start the factories, and wait for their caches to sync
		localFactory.Start(ctx.Done())
		localLiqoFactory.Start(ctx.Done())
		remoteFactory.Start(ctx.Done())
		remoteLiqoFactory.Start(ctx.Done())

		localFactory.WaitForCacheSync(ctx.Done())
		localLiqoFactory.WaitForCacheSync(ctx.Done())
		remoteFactory.WaitForCacheSync(ctx.Done())
		remoteLiqoFactory.WaitForCacheSync(ctx.Done())

		// If the context was closed before the cache was ready, let abort the setup
		select {
		case <-ctx.Done():
			return
		default:
			break
		}

		// The factories have synced, and we are now ready to start te replication
		klog.Infof("Reflection between local namespace %q and remote namespace %q correctly started", local, remote)
		ready = true
	}()
}

// StopNamespace stops the reflection for a given namespace.
func (m *manager) StopNamespace(local, remote string) {
	m.Lock()
	defer m.Unlock()

	klog.Infof("Stopping reflection between local namespace %q and remote namespace %q", local, remote)
	stop, found := m.stop[local]
	if !found {
		klog.Warningf("Reflection between local namespace %q and remote namespace %q already stopped", local, remote)
		return
	}

	stop()
	delete(m.stop, local)

	for _, reflector := range m.reflectors {
		reflector.StopNamespace(local, remote)
	}
	klog.Infof("Reflection between local namespace %q and remote namespace %q correctly stopped", local, remote)
}

// Resync forces the resync of all the informers.
func (m *manager) Resync() error {
	for i := range m.reflectors {
		if err := m.reflectors[i].Resync(); err != nil {
			klog.Errorf("Error while resyncing the %s reflector: %s", m.reflectors[i], err)
		}
	}
	return nil
}
