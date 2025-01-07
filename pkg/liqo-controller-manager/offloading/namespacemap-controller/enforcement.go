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

package namespacemapctrl

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils"
	liqoerrors "github.com/liqotech/liqo/pkg/utils/errors"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

// createNamespace creates a new namespace associated with a NamespaceMap. It returns whether a possible error
// could be ignored if previously successful or it refers to an hard failure.
func (r *NamespaceMapReconciler) createNamespace(ctx context.Context, name, originName string,
	nm *offloadingv1beta1.NamespaceMap) (ignorable bool, err error) {
	// The label is guaranteed to exist, since it is part of the filter predicate.
	origin := nm.Labels[consts.ReplicationOriginLabel]
	nmID, err := cache.MetaNamespaceKeyFunc(nm)
	utilruntime.Must(err)

	var namespace corev1.Namespace
	err = r.Get(ctx, types.NamespacedName{Name: name}, &namespace)
	if client.IgnoreNotFound(err) != nil {
		return true, fmt.Errorf("failed to retrieve namespace %q: %w", name, err)
	}

	// The namespace does not yet exist, and needs to be created
	if err != nil {
		namespace = corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Annotations: map[string]string{
					consts.RemoteNamespaceManagedByAnnotationKey:    nmID,
					consts.RemoteNamespaceOriginalNameAnnotationKey: originName,
				},
				Labels: map[string]string{
					consts.RemoteClusterID: origin,
				},
			},
		}

		if err := liqoerrors.IgnoreAlreadyExists(r.Create(ctx, &namespace)); err != nil {
			return false, fmt.Errorf("failed to create namespace %q: %w", name, err)
		}

		klog.Infof("Namespace %q successfully created", klog.KObj(&namespace))
	}

	// Check whether this namespace is controlled by the NamespaceMap controller.
	if value, ok := namespace.Annotations[consts.RemoteNamespaceManagedByAnnotationKey]; !ok || value != nmID {
		err := fmt.Errorf("namespace %q already exists and it is not managed by NamespaceMap %q", name, nmID)
		return false, err
	}

	// Make sure the appropriate role binding is present in the namespace for virtual kubelet operations.
	// The rolebinding is named after the tenant namespace name, since that is guaranteed to be unique.
	// This will simplify the support for remote namespaces associated with multiple origins.
	binding := rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Namespace: name, Name: nm.GetNamespace()}}
	result, err := resource.CreateOrUpdate(ctx, r.Client, &binding, func() error {
		binding.Annotations = labels.Merge(binding.GetAnnotations(), map[string]string{
			consts.RemoteNamespaceManagedByAnnotationKey: nmID,
		})
		binding.Labels = labels.Merge(binding.GetLabels(), map[string]string{
			consts.K8sAppManagedByKey: consts.LiqoAppLabelValue,
			consts.RemoteClusterID:    origin,
		})

		if binding.CreationTimestamp.IsZero() {
			binding.Subjects = []rbacv1.Subject{{APIGroup: rbacv1.GroupName, Kind: rbacv1.GroupKind, Name: origin}}
			binding.RoleRef = rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: consts.RemoteNamespaceClusterRoleName}
		}

		return nil
	})
	if err != nil {
		return true, fmt.Errorf("failed to enforce role binding %q: %w", klog.KObj(&binding), err)
	}

	klog.V(utils.FromResult(result)).Infof("RoleBinding %q successfully enforced (with %v operation)", klog.KObj(&binding), result)
	return true, nil
}

// For every entry of DesiredMapping create remote Namespace if it has not already being created.
// ensureNamespacesExistence tries to create all the remote namespaces requested in DesiredMapping (NamespaceMap->Spec->DesiredMapping).
func (r *NamespaceMapReconciler) ensureNamespacesExistence(ctx context.Context, nm *offloadingv1beta1.NamespaceMap) error {
	var err error

	for originName, destinationName := range nm.Spec.DesiredMapping {
		phase := offloadingv1beta1.MappingAccepted
		if ignorable, creationError := r.createNamespace(ctx, destinationName, originName, nm); creationError != nil {
			// Do not overwrite the phase in case the mapping was already present, and this is marked as a temporary error.
			previous, found := nm.Status.CurrentMapping[originName]
			if !ignorable || !found || previous.Phase != offloadingv1beta1.MappingAccepted {
				phase = offloadingv1beta1.MappingCreationLoopBackOff
			}

			klog.Errorf("Namespace enforcement failure: %v", creationError)
			err = creationError
		}

		nm.Status.CurrentMapping[originName] = offloadingv1beta1.RemoteNamespaceStatus{RemoteNamespace: destinationName, Phase: phase}
	}

	return err
}

// deleteNamespace removes an existing namespace associated with a NamespaceMap, and returns whether it still exists or not.
func (r *NamespaceMapReconciler) deleteNamespace(ctx context.Context, namespaceName, nmID string) (existing bool, err error) {
	var namespace corev1.Namespace
	if err = r.Get(ctx, types.NamespacedName{Name: namespaceName}, &namespace); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("Namespace %q correctly terminated", namespaceName)
			return false, nil
		}
		return true, fmt.Errorf("failed to retrieve namespace %q: %w", namespaceName, err)
	}

	// Check if it is a namespace has been created by the NamespaceMap controller.
	if value, ok := namespace.Annotations[consts.RemoteNamespaceManagedByAnnotationKey]; !ok || value != nmID {
		klog.Warningf("Namespace %q is not associated with NamespaceMap %q", namespaceName, nmID)
		return false, nil
	}

	if !namespace.DeletionTimestamp.IsZero() {
		klog.V(2).Infof("Namespace %q is undergoing graceful termination", namespaceName)
		return true, nil
	}

	if err = r.Delete(ctx, &namespace); err != nil {
		return true, fmt.Errorf("failed to delete namespace %q: %w", namespaceName, err)
	}

	klog.Infof("Namespace %q correctly marked for termination", namespaceName)
	return true, nil
}

// If DesiredMapping field has less entries than CurrentMapping, is necessary to remove some remote namespaces.
// This function checks if remote namespaces requested to be deleted are really deleted.
func (r *NamespaceMapReconciler) ensureNamespacesDeletion(ctx context.Context, nm *offloadingv1beta1.NamespaceMap) error {
	nmID, err := cache.MetaNamespaceKeyFunc(nm)
	utilruntime.Must(err)

	for originName, destinationStatus := range nm.Status.CurrentMapping {
		if _, ok := nm.Spec.DesiredMapping[originName]; ok && nm.DeletionTimestamp.IsZero() {
			continue
		}

		existing, deletionError := r.deleteNamespace(ctx, destinationStatus.RemoteNamespace, nmID)
		if err != nil {
			klog.Errorf("Namespace enforcement failure: %v", err)
			err = deletionError
			continue
		}

		if existing {
			nm.Status.CurrentMapping[originName] = offloadingv1beta1.RemoteNamespaceStatus{
				RemoteNamespace: destinationStatus.RemoteNamespace,
				Phase:           offloadingv1beta1.MappingTerminating,
			}
		} else {
			delete(nm.Status.CurrentMapping, originName)
		}
	}

	return err
}

// EnsureNamespaces checks if there are Namespaces that have to be created or deleted, in accordance with DesiredMapping
// field. It updates also NamespaceOffloading status in according to the remote Namespace Status.
func (r *NamespaceMapReconciler) EnsureNamespaces(ctx context.Context, nm *offloadingv1beta1.NamespaceMap) error {
	if nm.Status.CurrentMapping == nil {
		nm.Status.CurrentMapping = map[string]offloadingv1beta1.RemoteNamespaceStatus{}
	}

	errorCreationPhase := r.ensureNamespacesExistence(ctx, nm)
	errorDeletionPhase := r.ensureNamespacesDeletion(ctx, nm)

	if err := r.Status().Update(ctx, nm); err != nil {
		klog.Errorf("Failed to update the status of NamespaceMap %q: %v", klog.KObj(nm), err)
		return err
	}
	klog.V(4).Infof("Successfully enforced the status of NamespaceMap %q", klog.KObj(nm))

	if errorCreationPhase != nil {
		return fmt.Errorf("failed creating remote namespaces: %w", errorCreationPhase)
	}
	if errorDeletionPhase != nil {
		return fmt.Errorf("failed deleting remote namespaces: %w", errorDeletionPhase)
	}

	return nil
}

// NamespaceMapDeletionProcess handles the graceful termination of a NamespaceMap, deleting all namespaces and
// eventually removing the finalizer.
func (r *NamespaceMapReconciler) NamespaceMapDeletionProcess(ctx context.Context, nm *offloadingv1beta1.NamespaceMap) error {
	errorDeletionPhase := r.ensureNamespacesDeletion(ctx, nm)

	// Regardless of the outcome, update the status of the NamespaceMap, as part of the namespaces might have changed.
	if err := r.Status().Update(ctx, nm); err != nil {
		klog.Errorf("Failed to update the status of NamespaceMap %q: %v", klog.KObj(nm), err)
		return err
	}

	// If the NamespaceMap status is empty, then it is possible to remove the finalizer.
	if errorDeletionPhase == nil && len(nm.Status.CurrentMapping) == 0 {
		return r.RemoveNamespaceMapControllerFinalizer(ctx, nm)
	}

	// Return the deletion phase error (if any), to ensure the process is retried.
	return errorDeletionPhase
}
