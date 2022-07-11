// Copyright 2019-2022 The Liqo Authors
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

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	traceutils "github.com/liqotech/liqo/pkg/utils/trace"
)

const (
	specKey   = "spec"
	statusKey = "status"

	finalizer = "crdReplicator.liqo.io"
)

// item represents an item to be processed.
type item struct {
	gvr             schema.GroupVersionResource
	sourceClusterID string
	targetClusterID string
	name            string
}

// handle is the reconciliation function which is executed to reflect an object.
func (r *Reflector) handle(ctx context.Context, key item) error {
	tracer := trace.New("Handle", trace.Field{Key: "Resource", Value: key.gvr}, trace.Field{Key: "Name", Value: key.name})
	defer tracer.LogIfLong(traceutils.LongThreshold())

	klog.Infof("Processing %v with name %v, source %v, target %v (local-to-local: %v)", key.gvr, key.name, key.sourceClusterID, key.targetClusterID, r.isLocalToLocal)
	resource, ok := r.get(key.gvr, key.sourceClusterID, key.targetClusterID)
	if !ok {
		klog.Warningf("Failed to retrieve resource information for %v", key.gvr)
		return nil
	}
	klog.Infof("Retrieved resource information for %v with name %v, source %v, target %v (local-to-local: %v)", key.gvr, key.name, key.sourceClusterID, key.targetClusterID, r.isLocalToLocal)

	sourceClusterID := resource.sourceClusterID
	targetClusterID := resource.targetClusterID

	sourceClusterName := resource.sourceClusterName
	targetClusterName := resource.targetClusterName

	// Retrieve the resource from the source cluster
	localRes, err := resource.listerForSource.Get(key.name)
	if err != nil {
		if kerrors.IsNotFound(err) {
			klog.Infof("[%v] Deleting remote %v with name %v, since the local one does no longer exist",
				targetClusterID, key.gvr, key.name)
			defer tracer.Step("Ensured the absence of the remote object")
			_, err = r.deleteRemoteObject(ctx, resource, key)
			return err
		}
		klog.Errorf("[%v] Failed to retrieve local %v with name %v: %v", targetClusterID, key.gvr, key.name, err)
		return err
	}

	// Convert the resource to unstructured
	tmp, err := runtime.DefaultUnstructuredConverter.ToUnstructured(localRes)
	if err != nil {
		klog.Errorf("[%v] Failed to convert local %v with name %v to unstructured: %v", targetClusterID, key.gvr, key.name, err)
		return err
	}
	localUnstr := &unstructured.Unstructured{Object: tmp}

	// Check if the resource has the expected destination cluster
	if key.gvr.Resource == netv1alpha1.NetworkConfigGroupResource.Resource {
		if passthrough, ok := localUnstr.GetLabels()["passthrough"]; !ok || passthrough != strconv.FormatBool(true) {
			// if it is not passthrough, ensure the destination clusterID equals the target clusterID
			if remoteClusterID, ok := localUnstr.GetLabels()["destination"]; !ok || remoteClusterID != targetClusterID {
				klog.Warningf("[%v] Resource %v with name %q has a mismatching destination cluster ID: %v",
					targetClusterID, key.gvr, key.name, remoteClusterID)
				// Do not return an error, since retrying would be pointless
				return nil
			}
		}
	} else if remoteClusterID, ok := localUnstr.GetLabels()[consts.ReplicationDestinationLabel]; !ok || remoteClusterID != targetClusterID {
		klog.Warningf("[%v] Resource %v with name %q has a mismatching destination cluster ID: %v",
			targetClusterID, key.gvr, key.name, remoteClusterID)
		// Do not return an error, since retrying would be pointless
		return nil
	}
	tracer.Step("Retrieved the local object")

	// Check if the local resource has been marked for deletion
	if !localUnstr.GetDeletionTimestamp().IsZero() {
		klog.Infof("[%v] Deleting remote %v with name %v, since the local one is being deleted", targetClusterID, key.gvr, key.name)
		vanished, err := r.deleteRemoteObject(ctx, resource, key)
		if err != nil {
			return err
		}
		tracer.Step("Ensured the absence of the remote object")

		// Remove the finalizer from the local resource, if the remote one does no longer exist.
		if vanished {
			_, err = r.ensureLocalFinalizer(ctx, key.gvr, targetClusterID, localUnstr, false, controllerutil.RemoveFinalizer)
			tracer.Step("Ensured the local finalizer absence")
			return err
		}
	}

	// Ensure the local resource has the finalizer
	if localUnstr, err = r.ensureLocalFinalizer(ctx, key.gvr, targetClusterID, localUnstr, true, controllerutil.AddFinalizer); err != nil {
		return err
	}
	tracer.Step("Ensured the local finalizer presence")

	// Retrieve the resource from the target cluster
	remoteRes, err := resource.listerForTarget.Get(key.name)
	if err != nil {
		if kerrors.IsNotFound(err) {
			klog.Infof("[%v] Creating remote %v with name %v", targetClusterID, key.gvr, key.name)
			defer tracer.Step("Ensured the presence of the remote object")
			return r.createRemoteObject(ctx, resource, localUnstr, sourceClusterID, sourceClusterName, targetClusterID, targetClusterName)
		}
		klog.Errorf("[%v] Failed to retrieve remote %v with name %v: %v", targetClusterID, key.gvr, key.name, err)
		return err
	}

	// Convert the resource to unstructured
	tmp, err = runtime.DefaultUnstructuredConverter.ToUnstructured(remoteRes)
	if err != nil {
		klog.Errorf("[%v] Failed to convert remote %v with name %v to unstructured: %v", targetClusterID, key.gvr, key.name, err)
		return err
	}
	remoteUnstr := &unstructured.Unstructured{Object: tmp}
	tracer.Step("Retrieved the remote object")

	// Replicate the spec towards the remote cluster
	if remoteUnstr, err = r.updateRemoteObjectSpec(ctx, key.gvr, targetClusterID, resource.targetNamespace, localUnstr, remoteUnstr); err != nil {
		return err
	}
	tracer.Step("Ensured the spec is synchronized")

	// Replicate the status towards the local or remote cluster, depending on the reflection policy
	defer tracer.Step("Ensured the status is synchronized")
	return r.updateObjectStatus(ctx, resource, localUnstr, remoteUnstr)
}

// createRemoteObject creates a given object in the remote cluster.
func (r *Reflector) createRemoteObject(ctx context.Context, resource *resourceToReflect, local *unstructured.Unstructured, sourceClusterID, sourceClusterName, targetClusterID, targetClusterName string) error {
	remote := &unstructured.Unstructured{}
	remote.SetGroupVersionKind(local.GetObjectKind().GroupVersionKind())
	remote.SetNamespace(resource.targetNamespace)
	remote.SetName(local.GetName())
	if r.isLocalToLocal {
		remote.SetLabels(r.mutateLabelsForLocalToLocal(resource.localClusterID, targetClusterID, targetClusterName, local.GetLabels()))
	} else {
		remote.SetLabels(r.mutateLabelsForRemote(sourceClusterID, sourceClusterName, local.GetLabels()))
	}
	remote.SetAnnotations(local.GetAnnotations())

	// Retrieve the spec of the local object
	spec, err := r.getNestedMap(targetClusterID, local, specKey, resource.gvr)
	utilruntime.Must(err)

	err = unstructured.SetNestedMap(remote.Object, spec, specKey)
	utilruntime.Must(err)

	// Create the resource in the remote cluster
	if remote, err = r.clientForTarget.Resource(resource.gvr).Namespace(resource.targetNamespace).Create(ctx, remote, metav1.CreateOptions{}); err != nil {
		klog.Errorf("[%v] Failed to create remote %v with name %v: %v", targetClusterID, resource.gvr, local.GetName(), err)
		return err
	}
	klog.Infof("[%v] Remote %v with name %v successfully created", targetClusterID, resource.gvr, local.GetName())

	// Replicate the status towards the local or remote cluster, depending on the reflection policy
	return r.updateObjectStatus(ctx, resource, local, remote)
}

// updateRemoteObjectSpec updates the spec of a remote object.
func (r *Reflector) updateRemoteObjectSpec(ctx context.Context, gvr schema.GroupVersionResource, targetClusterID, targetNamespace string, local, remote *unstructured.Unstructured) (
	*unstructured.Unstructured, error) {
	// Retrieve the spec of the local and remote objects
	specLocal, err := r.getNestedMap(targetClusterID, local, specKey, gvr)
	utilruntime.Must(err)

	specRemote, err := r.getNestedMap(targetClusterID, remote, specKey, gvr)
	utilruntime.Must(err)

	// The specs are already the same, nothing to do
	if reflect.DeepEqual(specLocal, specRemote) {
		return remote, nil
	}

	// Update the remote spec field
	err = unstructured.SetNestedMap(remote.Object, specLocal, specKey)
	utilruntime.Must(err)

	// Update the resource in the remote cluster
	if remote, err = r.clientForTarget.Resource(gvr).Namespace(targetNamespace).Update(ctx, remote, metav1.UpdateOptions{}); err != nil {
		klog.Errorf("[%v] Failed to update remote %v with name %v: %v", targetClusterID, gvr, local.GetName(), err)
		return remote, err
	}

	klog.Infof("[%v] Remote %v with name %v successfully updated", targetClusterID, gvr, local.GetName())
	return remote, nil
}

// updateObjectStatus updates the status of a local or remote object, depending on the resource ownership.
func (r *Reflector) updateObjectStatus(ctx context.Context, resource *resourceToReflect, local, remote *unstructured.Unstructured) error {
	switch resource.ownership {
	case consts.OwnershipLocal:
		return r.updateObjectStatusInner(ctx, r.clientForTarget, resource.targetClusterID, resource.targetNamespace, resource.gvr, local, remote)
	case consts.OwnershipShared:
		return r.updateObjectStatusInner(ctx, r.getLocalClient(), resource.sourceClusterID, resource.sourceNamespace, resource.gvr, remote, local)
	default:
		klog.Fatalf("Unknown ownership %v", resource.ownership)
	}
	return nil
}

// updateObjectStatusInner performs the actual status update. Arguments source and target could either refer to local or remote resources.
func (r *Reflector) updateObjectStatusInner(ctx context.Context, client dynamic.Interface, clusterID, namespace string,
	gvr schema.GroupVersionResource, source, target *unstructured.Unstructured) error {
	// Retrieve the status of the source and target objects
	statusSource, err := r.getNestedMap(clusterID, source, statusKey, gvr)
	utilruntime.Must(err)

	statusTarget, err := r.getNestedMap(clusterID, target, statusKey, gvr)
	utilruntime.Must(err)

	// The statuses are already the same, nothing to do
	if reflect.DeepEqual(statusSource, statusTarget) {
		return nil
	}

	// Update the target status field
	err = unstructured.SetNestedMap(target.Object, statusSource, statusKey)
	utilruntime.Must(err)

	// Update the target resource
	if _, err = client.Resource(gvr).Namespace(namespace).UpdateStatus(ctx, target, metav1.UpdateOptions{}); err != nil {
		klog.Errorf("[%v] Failed to update the status of %v with name %v: %v", clusterID, gvr, source.GetName(), err)
		return err
	}

	klog.Infof("[%v] Status of %v with name %v successfully updated", clusterID, gvr, source.GetName())
	return nil
}

// deleteRemoteObject deletes a given object from the remote cluster.
func (r *Reflector) deleteRemoteObject(ctx context.Context, resource *resourceToReflect, key item) (vanished bool, err error) {
	if _, err := resource.listerForTarget.Get(key.name); err != nil {
		if kerrors.IsNotFound(err) {
			klog.Infof("[%v] Remote %v with name %v already vanished", resource.targetClusterID, key.gvr, key.name)
			return true, nil
		}
		klog.Errorf("[%v] Failed to retrieve remote object %v: %v", resource.targetClusterID, key.gvr, key.name, err)
		return false, err
	}

	err = r.clientForTarget.Resource(key.gvr).Namespace(resource.targetNamespace).Delete(ctx, key.name, metav1.DeleteOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		klog.Errorf("[%v] Failed to delete remote %v with name %v: %v", resource.targetClusterID, key.gvr, key.name, err)
		return false, err
	}

	klog.Infof("[%v] Remote %v with name %v successfully deleted", resource.targetClusterID, key.gvr, key.name)
	return kerrors.IsNotFound(err), nil
}

// getNestedMap is a wrapper to retrieve a nested map from an unstructured object.
func (r *Reflector) getNestedMap(clusterID string, unstr *unstructured.Unstructured, key string, gvr schema.GroupVersionResource) (map[string]interface{}, error) {
	// Retrieve the spec of the original object
	nested, found, err := unstructured.NestedMap(unstr.Object, key)
	if err != nil {
		klog.Errorf("[%v] An error occurred while processing the %v of remote %v with name %v: %v",
			clusterID, key, gvr, unstr.GetName(), err)
		return nil, fmt.Errorf("failed to retrieve %v key: %w", key, err)
	}

	// Do not fail in case the key is not found (should only happen during tests)
	if !found {
		nested = map[string]interface{}{}
	}

	return nested, nil
}

// ensureLocalFinalizer updates the local resource ensuring the presence/absence of the finalizer.
func (r *Reflector) ensureLocalFinalizer(ctx context.Context, gvr schema.GroupVersionResource, targetClusterID string, local *unstructured.Unstructured,
	expected bool, updater func(client.Object, string)) (
	*unstructured.Unstructured, error) {
	// Do not perform any action if the finalizer is already present (expected is true) or absent (expected is false)
	if controllerutil.ContainsFinalizer(local, finalizer) == expected {
		return local, nil
	}

	updater(local, finalizer)
	updated, err := r.getLocalClient().Resource(gvr).Namespace(local.GetNamespace()).Update(ctx, local, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("[%v] Failed to update finalizer of local %v with name %v: %v", targetClusterID, gvr, local.GetName(), err)
		return nil, err
	}

	klog.Infof("[%v] Successfully updated finalizer of local %v with name %v", targetClusterID, gvr, local.GetName())
	return updated, nil
}

func (r *Reflector) mutateLabelsForLocalToLocal(localClusterID, targetClusterID, targetClusterName string, labels map[string]string) map[string]string {
	// setting remoteID to the target clusterID (overwriting it if needed)
	labels[consts.ReplicationDestinationLabel] = targetClusterID
	// setting remoteName to the target clusterName (overwriting it if needed)
	labels[consts.ReplicationDestinationNameLabel] = targetClusterName
	// setting the replication label to true (overwriting it if needed)
	labels[consts.ReplicationRequestedLabel] = strconv.FormatBool(true)
	// setting the replication status to false (overwriting it if needed)
	labels[consts.ReplicationStatusLabel] = strconv.FormatBool(false)

	return labels
}

// mutateLabelsForRemote mutates the labels map adding the ones for the remote cluster.
// the ownership of the resource is removed as it would not make sense in a remote cluster.
func (r *Reflector) mutateLabelsForRemote(sourceClusterID, sourceClusterName string, labels map[string]string) map[string]string {
	// We don't check if the map is nil, since it has to be initialized because we use the labels to filter the resources
	// which need to be replicated.

	if _, ok := labels[consts.ReplicationOriginLabel]; !ok {
		// setting originID and originName, i.e. the source clusterID and clusterName, if they are not already present
		// (otherwise they would get overwritten when reflecting to the remote peer after passing through the central cluster)
		labels[consts.ReplicationOriginLabel] = sourceClusterID
		labels[consts.ReplicationOriginNameLabel] = sourceClusterName
	}

	// setting the replication label to false
	labels[consts.ReplicationRequestedLabel] = strconv.FormatBool(false)
	// setting replication status to true
	labels[consts.ReplicationStatusLabel] = strconv.FormatBool(true)

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
	// Get the element to be processed.
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
