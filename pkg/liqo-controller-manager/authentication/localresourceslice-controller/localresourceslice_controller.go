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

package localresourceslicecontroller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	klog "k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/internal/crdReplicator/reflection"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
)

// NewLocalResourceSliceReconciler returns a new LocalResourceSliceReconciler.
func NewLocalResourceSliceReconciler(cl client.Client, s *runtime.Scheme,
	recorder record.EventRecorder, liqoNamespace string,
	localClusterID liqov1beta1.ClusterID) *LocalResourceSliceReconciler {
	return &LocalResourceSliceReconciler{
		Client: cl,
		Scheme: s,

		eventRecorder: recorder,

		liqoNamespace:  liqoNamespace,
		localClusterID: localClusterID,
	}
}

// LocalResourceSliceReconciler reconciles a ResourceSliceReconciler object.
type LocalResourceSliceReconciler struct {
	client.Client
	*runtime.Scheme

	eventRecorder record.EventRecorder

	liqoNamespace  string
	localClusterID liqov1beta1.ClusterID
}

// cluster-role
// +kubebuilder:rbac:groups=authentication.liqo.io,resources=resourceslices;resourceslices/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=authentication.liqo.io,resources=identities,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch

// Reconcile local ResourceSlice resources.
func (r *LocalResourceSliceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var resourceSlice authv1beta1.ResourceSlice
	if err := r.Get(ctx, req.NamespacedName, &resourceSlice); err != nil {
		if errors.IsNotFound(err) {
			klog.V(4).Infof("resourceSlice %q not found", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		klog.Errorf("unable to get ResourceSlice %q: %v", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	if resourceSlice.Spec.ConsumerClusterID == nil {
		// Set the identity of the local cluster as the consumer identity.
		resourceSlice.Spec.ConsumerClusterID = ptr.To(r.localClusterID)
	}

	if resourceSlice.Spec.ProviderClusterID == nil {
		if resourceSlice.Labels != nil && resourceSlice.Labels[consts.RemoteClusterID] != "" {
			resourceSlice.Spec.ProviderClusterID = ptr.To(liqov1beta1.ClusterID(resourceSlice.Labels[consts.RemoteClusterID]))
		} else {
			// If there is no label, get the identity from the Tenant namespace.
			tenantNamespace := resourceSlice.Namespace
			var ns corev1.Namespace
			if err := r.Get(ctx, client.ObjectKey{Name: tenantNamespace}, &ns); err != nil {
				klog.Errorf("unable to get Namespace %q: %v", tenantNamespace, err)
				r.eventRecorder.Event(&resourceSlice, corev1.EventTypeWarning, "FailedGetNamespace", err.Error())
				return ctrl.Result{}, err
			}

			clusterID, err := tenantnamespace.GetClusterIDFromTenantNamespace(&ns)
			if err != nil {
				klog.Errorf("unable to get ClusterID from Namespace %q: %v", tenantNamespace, err)
				r.eventRecorder.Event(&resourceSlice, corev1.EventTypeWarning, "FailedGetClusterID", err.Error())
				return ctrl.Result{}, err
			}

			resourceSlice.Spec.ProviderClusterID = ptr.To(clusterID)
		}
	}

	// Get public and private keys of the local cluster.
	privateKey, _, err := authentication.GetClusterKeys(ctx, r.Client, r.liqoNamespace)
	if err != nil {
		klog.Errorf("unable to get local cluster keys: %v", err)
		r.eventRecorder.Event(&resourceSlice, corev1.EventTypeWarning, "FailedGetLocalClusterKeys", err.Error())
		return ctrl.Result{}, err
	}

	if len(resourceSlice.Spec.CSR) == 0 {
		// Generate a CSR for the remote cluster.
		CSR, err := authentication.GenerateCSRForResourceSlice(privateKey, &resourceSlice)
		if err != nil {
			klog.Errorf("unable to generate CSR for ResourceSlice %q: %v", req.NamespacedName, err)
			r.eventRecorder.Event(&resourceSlice, corev1.EventTypeWarning, "FailedGenerateCSR", err.Error())
			return ctrl.Result{}, err
		}

		resourceSlice.Spec.CSR = CSR
	}

	if err = r.Update(ctx, &resourceSlice); err != nil {
		klog.Errorf("unable to update ResourceSlice %q: %v", req.NamespacedName, err)
		r.eventRecorder.Event(&resourceSlice, corev1.EventTypeWarning, "FailedUpdateResourceSlice", err.Error())
		return ctrl.Result{}, err
	}

	klog.Infof("ResourceSlice %q reconciled", req.NamespacedName)
	r.eventRecorder.Event(&resourceSlice, corev1.EventTypeNormal, "Reconciled", "ResourceSlice reconciled")

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LocalResourceSliceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// generate the predicate to filter just the ResourceSlices created by the local cluster checking crdReplicator labels
	localResSliceFilter, err := predicate.LabelSelectorPredicate(reflection.LocalResourcesLabelSelector())
	if err != nil {
		klog.Error(err)
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlResourceSliceLocal).
		For(&authv1beta1.ResourceSlice{}, builder.WithPredicates(localResSliceFilter)).
		Complete(r)
}
