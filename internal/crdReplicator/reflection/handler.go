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

package reflection

import (
	"context"
	"fmt"
	"reflect"
	"strconv"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
	"k8s.io/utils/trace"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/liqotech/liqo/pkg/consts"
	traceutils "github.com/liqotech/liqo/pkg/utils/trace"
)

const (
	specKey   = "spec"
	statusKey = "status"

	finalizer = "crdreplicator.liqo.io/resource"
)

// item represents an item to be processed.
type item struct {
	gvr  schema.GroupVersionResource
	name string
}

// handle is the reconciliation function which is executed to reflect an object.
func (r *Reflector) handle(ctx context.Context, key item) error {
	tracer := trace.New("Handle", trace.Field{Key: "RemoteClusterID", Value: r.remoteClusterID},
		trace.Field{Key: "Resource", Value: key.gvr}, trace.Field{Key: "Name", Value: key.name})
	defer tracer.LogIfLong(traceutils.LongThreshold())

	klog.Infof("[%v] Processing %v with name %v", r.remoteClusterID, key.gvr, key.name)
	resource, ok := r.get(key.gvr)
	if !ok {
		klog.Warningf("[%v] Failed to retrieve resource information for %v", r.remoteClusterID, key.gvr)
		return nil
	}

	// Retrieve the resource from the local cluster
	local, err := resource.local.Get(key.name)
	if err != nil {
		if kerrors.IsNotFound(err) {
			klog.Infof("[%v] Deleting remote %v with name %v, since the local one does no longer exist",
				r.remoteClusterID, key.gvr, key.name)
			defer tracer.Step("Ensured the absence of the remote object")
			_, err = r.deleteRemoteObject(ctx, resource, key)
			return err
		}
		klog.Errorf("[%v] Failed to retrieve local %v with name %v: %v", r.remoteClusterID, key.gvr, key.name, err)
		return err
	}

	// Convert the resource to unstructured
	tmp, err := runtime.DefaultUnstructuredConverter.ToUnstructured(local)
	if err != nil {
		klog.Errorf("[%v] Failed to convert local %v with name %v to unstructured: %v", r.remoteClusterID, key.gvr, key.name, err)
		return err
	}
	localUnstr := &unstructured.Unstructured{Object: tmp}

	// Check if the resource has the expected destination cluster
	if remoteClusterID, ok := localUnstr.GetLabels()[consts.ReplicationDestinationLabel]; !ok || remoteClusterID != string(r.remoteClusterID) {
		klog.Warningf("[%v] Resource %v with name %q has a mismatching destination cluster ID: %v",
			r.remoteClusterID, key.gvr, key.name, remoteClusterID)
		// Do not return an error, since retrying would be pointless
		return nil
	}
	tracer.Step("Retrieved the local object")

	// Check if the local resource has been marked for deletion
	if !localUnstr.GetDeletionTimestamp().IsZero() {
		klog.Infof("[%v] Deleting remote %v with name %v, since the local one is being deleted", r.remoteClusterID, key.gvr, key.name)
		vanished, err := r.deleteRemoteObject(ctx, resource, key)
		if err != nil {
			return err
		}
		tracer.Step("Ensured the absence of the remote object")

		// Remove the finalizer from the local resource, if the remote one does no longer exist.
		if vanished {
			_, err = r.ensureLocalFinalizer(ctx, key.gvr, localUnstr, controllerutil.RemoveFinalizer)
			tracer.Step("Ensured the local finalizer absence")
			return err
		}

		return nil
	}

	// Ensure the local resource has the finalizer
	if localUnstr, err = r.ensureLocalFinalizer(ctx, key.gvr, localUnstr, controllerutil.AddFinalizer); err != nil {
		return err
	}
	tracer.Step("Ensured the local finalizer presence")

	// Retrieve the resource from the remote cluster
	remote, err := resource.remote.Get(key.name)
	switch {
	case kerrors.IsForbidden(err):
		klog.Infof("[%v] Cannot retrieve remote %v with name %v (permission removed by provider)", r.remoteClusterID, key.gvr, key.name)
		return nil
	case kerrors.IsNotFound(err):
		klog.Infof("[%v] Creating remote %v with name %v", r.remoteClusterID, key.gvr, key.name)
		defer tracer.Step("Ensured the presence of the remote object")
		errCreate := r.createRemoteObject(ctx, resource, localUnstr)
		if kerrors.IsForbidden(errCreate) {
			klog.Infof("[%v] Cannot create remote %v with name %v (permission removed by provider)", r.remoteClusterID, key.gvr, key.name)
			return nil
		}
		return errCreate
	case err != nil:
		klog.Errorf("[%v] Failed to retrieve remote %v with name %v: %v", r.remoteClusterID, key.gvr, key.name, err)
		return err
	}

	// Convert the resource to unstructured
	tmp, err = runtime.DefaultUnstructuredConverter.ToUnstructured(remote)
	if err != nil {
		klog.Errorf("[%v] Failed to convert remote %v with name %v to unstructured: %v", r.remoteClusterID, key.gvr, key.name, err)
		return err
	}
	remoteUnstr := &unstructured.Unstructured{Object: tmp}
	tracer.Step("Retrieved the remote object")

	// Replicate the spec towards the remote cluster
	if remoteUnstr, err = r.updateRemoteObjectSpec(ctx, key.gvr, localUnstr, remoteUnstr); err != nil {
		return err
	}
	tracer.Step("Ensured the spec is synchronized")

	// Replicate the status towards the local or remote cluster, depending on the reflection policy
	defer tracer.Step("Ensured the status is synchronized")
	return r.updateObjectStatus(ctx, resource, localUnstr, remoteUnstr)
}

// createRemoteObject creates a given object in the remote cluster.
func (r *Reflector) createRemoteObject(ctx context.Context, resource *reflectedResource, local *unstructured.Unstructured) error {
	remote := &unstructured.Unstructured{}
	remote.SetGroupVersionKind(local.GetObjectKind().GroupVersionKind())
	remote.SetNamespace(r.remoteNamespace)
	remote.SetName(local.GetName())
	remote.SetLabels(r.mutateLabelsForRemote(local.GetLabels()))
	remote.SetAnnotations(local.GetAnnotations())

	// Retrieve the spec of the local object
	spec, err := r.getNestedMap(local, specKey, resource.gvr)
	utilruntime.Must(err)

	err = unstructured.SetNestedMap(remote.Object, spec, specKey)
	utilruntime.Must(err)

	// Create the resource in the remote cluster
	if remote, err = r.remoteClient.Resource(resource.gvr).Namespace(r.remoteNamespace).Create(ctx, remote, metav1.CreateOptions{}); err != nil {
		klog.Errorf("[%v] Failed to create remote %v with name %v: %v", r.remoteClusterID, resource.gvr, local.GetName(), err)
		return err
	}
	klog.Infof("[%v] Remote %v with name %v successfully created", r.remoteClusterID, resource.gvr, local.GetName())

	// Replicate the status towards the local or remote cluster, depending on the reflection policy
	return r.updateObjectStatus(ctx, resource, local, remote)
}

// updateRemoteObjectSpec updates the spec of a remote object.
func (r *Reflector) updateRemoteObjectSpec(ctx context.Context, gvr schema.GroupVersionResource, local, remote *unstructured.Unstructured) (
	*unstructured.Unstructured, error) {
	// Retrieve the spec of the local and remote objects
	specLocal, err := r.getNestedMap(local, specKey, gvr)
	utilruntime.Must(err)

	specRemote, err := r.getNestedMap(remote, specKey, gvr)
	utilruntime.Must(err)

	// The specs are already the same, nothing to do
	if reflect.DeepEqual(specLocal, specRemote) {
		return remote, nil
	}

	// Update the remote spec field
	err = unstructured.SetNestedMap(remote.Object, specLocal, specKey)
	utilruntime.Must(err)

	// Update the resource in the remote cluster
	if remote, err = r.remoteClient.Resource(gvr).Namespace(r.remoteNamespace).Update(ctx, remote, metav1.UpdateOptions{}); err != nil {
		klog.Errorf("[%v] Failed to update remote %v with name %v: %v", r.remoteClusterID, gvr, local.GetName(), err)
		return remote, err
	}

	klog.Infof("[%v] Remote %v with name %v successfully updated", r.remoteClusterID, gvr, local.GetName())
	return remote, nil
}

// updateObjectStatus updates the status of a local or remote object, depending on the resource ownership.
func (r *Reflector) updateObjectStatus(ctx context.Context, resource *reflectedResource, local, remote *unstructured.Unstructured) error {
	switch resource.ownership {
	case consts.OwnershipLocal:
		return r.updateObjectStatusInner(ctx, r.remoteClient, r.remoteNamespace, resource.gvr, local, remote)
	case consts.OwnershipShared:
		return r.updateObjectStatusInner(ctx, r.manager.client, r.localNamespace, resource.gvr, remote, local)
	default:
		klog.Fatalf("Unknown ownership %v", resource.ownership)
	}
	return nil
}

// updateObjectStatusInner performs the actual status update.
func (r *Reflector) updateObjectStatusInner(ctx context.Context, cl dynamic.Interface, namespace string,
	gvr schema.GroupVersionResource, source, destination *unstructured.Unstructured) error {
	// Retrieve the status of the source and destination objects
	statusSource, err := r.getNestedMap(source, statusKey, gvr)
	utilruntime.Must(err)

	statusDestination, err := r.getNestedMap(destination, statusKey, gvr)
	utilruntime.Must(err)

	// The statuses are already the same, nothing to do
	if reflect.DeepEqual(statusSource, statusDestination) {
		return nil
	}

	// Update the local status field
	err = unstructured.SetNestedMap(destination.Object, statusSource, statusKey)
	utilruntime.Must(err)

	// Update the resource in the destination cluster
	if _, err = cl.Resource(gvr).Namespace(namespace).UpdateStatus(ctx, destination, metav1.UpdateOptions{}); err != nil {
		klog.Errorf("[%v] Failed to update the status of %v with name %v: %v", r.remoteClusterID, gvr, source.GetName(), err)
		return err
	}

	klog.Infof("[%v] Status of %v with name %v successfully updated", r.remoteClusterID, gvr, source.GetName())
	return nil
}

// deleteRemoteObject deletes a given object from the remote cluster.
func (r *Reflector) deleteRemoteObject(ctx context.Context, resource *reflectedResource, key item) (vanished bool, err error) {
	if _, err := resource.remote.Get(key.name); err != nil {
		if kerrors.IsForbidden(err) {
			klog.Infof("[%v] Cannot retrieve remote %v with name %v (permission removed by provider)", r.remoteClusterID, key.gvr, key.name)
			return true, nil
		}
		if kerrors.IsNotFound(err) {
			klog.Infof("[%v] Remote %v with name %v already vanished", r.remoteClusterID, key.gvr, key.name)
			return true, nil
		}
		klog.Errorf("[%v] Failed to retrieve remote object %v %s: %v", r.remoteClusterID, key.gvr, key.name, err)
		return false, err
	}

	err = r.remoteClient.Resource(key.gvr).Namespace(r.remoteNamespace).Delete(ctx, key.name, metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		klog.Errorf("[%v] Failed to delete remote %v with name %v: %v", r.remoteClusterID, key.gvr, key.name, err)
		return false, err
	}

	klog.Infof("[%v] Remote %v with name %v successfully deleted", r.remoteClusterID, key.gvr, key.name)
	return kerrors.IsNotFound(err), nil
}

// getNestedMap is a wrapper to retrieve a nested map from an unstructured object.
func (r *Reflector) getNestedMap(unstr *unstructured.Unstructured, key string, gvr schema.GroupVersionResource) (map[string]interface{}, error) {
	// Retrieve the spec of the original object
	nested, found, err := unstructured.NestedMap(unstr.Object, key)
	if err != nil {
		klog.Errorf("[%v] An error occurred while processing the %v of remote %v with name %v: %v",
			r.remoteClusterID, key, gvr, unstr.GetName(), err)
		return nil, fmt.Errorf("failed to retrieve %v key: %w", key, err)
	}

	// Do not fail in case the key is not found (should only happen during tests)
	if !found {
		nested = map[string]interface{}{}
	}

	return nested, nil
}

// ensureLocalFinalizer updates the local resource ensuring the presence/absence of the finalizer.
func (r *Reflector) ensureLocalFinalizer(ctx context.Context, gvr schema.GroupVersionResource, local *unstructured.Unstructured,
	updater func(client.Object, string) bool) (
	*unstructured.Unstructured, error) {
	// Do not perform any action if the finalizer is already as expected
	if !updater(local, finalizer) {
		return local, nil
	}

	updated, err := r.manager.client.Resource(gvr).Namespace(local.GetNamespace()).Update(ctx, local, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("[%v] Failed to update finalizer of local %v with name %v: %v", r.remoteClusterID, gvr, local.GetName(), err)
		return nil, err
	}

	klog.Infof("[%v] Successfully updated finalizer of local %v with name %v", r.remoteClusterID, gvr, local.GetName())
	return updated, nil
}

// mutateLabelsForRemote mutates the labels map adding the ones for the remote cluster.
// the ownership of the resource is removed as it would not make sense in a remote cluster.
func (r *Reflector) mutateLabelsForRemote(labels map[string]string) map[string]string {
	// We don't check if the map is nil, since it has to be initialized because we use the labels to filter the resources
	// which need to be replicated.

	// setting the replication label to false
	labels[consts.ReplicationRequestedLabel] = strconv.FormatBool(false)
	// setting replication status to true
	labels[consts.ReplicationStatusLabel] = strconv.FormatBool(true)
	// setting originID i.e clusterID of home cluster
	labels[consts.ReplicationOriginLabel] = string(r.localClusterID)
	// setting the right remote cluster ID
	labels[consts.RemoteClusterID] = string(r.localClusterID)

	// delete the ownership label if any.
	delete(labels, consts.LocalResourceOwnership)

	return labels
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (r *Reflector) runWorker() {
	for r.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the handler.
func (r *Reflector) processNextWorkItem() bool {
	// Get he element to be processed.
	key, shutdown := r.workqueue.Get()

	if shutdown {
		return false
	}

	// We call Done here so the workqueue knows we have finished
	// processing this item. We also must remember to call Forget if we
	// do not want this work item being re-queued. For example, we do
	// not call Forget if a transient error occurs, instead the item is
	// put back on the workqueue and attempted again after a back-off
	// period.
	defer r.workqueue.Done(key)

	// Run the handler, passing it the item to be processed as parameter.
	if err := r.handle(context.Background(), key.(item)); err != nil {
		// Put the item back on the workqueue to handle any transient errors.
		r.workqueue.AddRateLimited(key)
		return true
	}

	// Finally, if no error occurs we Forget this item so it does not
	// get queued again until another change happens.
	r.workqueue.Forget(key)
	return true
}
