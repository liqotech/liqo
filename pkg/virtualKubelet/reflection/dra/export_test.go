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

package dra

import (
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	resourcev1listers "k8s.io/client-go/listers/resource/v1"
	"k8s.io/client-go/util/workqueue"

	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

// Test-only exports for private members of the dra package.

// Handle exposes ResourceSliceReflector.handle for tests.
var Handle = (*ResourceSliceReflector).handle

// EnqueueRemoteSlicesForNode exposes ResourceSliceReflector.enqueueRemoteSlicesForNode for tests.
var EnqueueRemoteSlicesForNode = (*ResourceSliceReflector).enqueueRemoteSlicesForNode

// EnsureRemoteDeviceClass exposes the package-private function for tests.
var EnsureRemoteDeviceClass = ensureRemoteDeviceClass

// IsDRAAPISupported exposes the package-private discovery probe for tests.
var IsDRAAPISupported = isDRAAPISupported

// NewResourceSliceReflectorForTest builds a ResourceSliceReflector with prebuilt
// listers and clients, bypassing the informer factory wiring done by Start.
func NewResourceSliceReflectorForTest(
	localClient, remoteClient kubernetes.Interface,
	localSlices, remoteSlices resourcev1listers.ResourceSliceLister,
	localNodes corev1listers.NodeLister,
	forgingOpts *forge.ForgingOpts,
) *ResourceSliceReflector {
	return &ResourceSliceReflector{
		name:         ResourceSliceReflectorName,
		workers:      1,
		localClient:  localClient,
		remoteClient: remoteClient,
		localSlices:  localSlices,
		remoteSlices: remoteSlices,
		localNodes:   localNodes,
		forgingOpts:  forgingOpts,
		workqueue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[types.NamespacedName](),
			workqueue.TypedRateLimitingQueueConfig[types.NamespacedName]{Name: ResourceSliceReflectorName}),
	}
}

// SetForgingOpts mutates the forging options on a reflector after construction.
// Used by tests to swap the opts mid-suite.
func (rsr *ResourceSliceReflector) SetForgingOpts(forgingOpts *forge.ForgingOpts) {
	rsr.forgingOpts = forgingOpts
}

// WorkqueueLen returns the number of items currently queued. Test-only helper
// so suites can assert on enqueue/resync behavior.
func (rsr *ResourceSliceReflector) WorkqueueLen() int {
	return rsr.workqueue.Len()
}
