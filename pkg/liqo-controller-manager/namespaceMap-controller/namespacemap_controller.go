/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package namespacemapctrl

import (
	"context"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mapsv1alpha1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
)

// NamespaceMapReconciler creates remote namespaces and updates NamespaceMaps Status.
type NamespaceMapReconciler struct {
	client.Client
	RemoteClients         map[string]kubernetes.Interface
	IdentityManagerClient kubernetes.Interface
	LocalClusterID        string
	RequeueTime           time.Duration
}

// cluster-role
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=virtualkubelet.liqo.io,resources=namespacemaps,verbs=get;watch;list;update;patch;create;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch;delete

// needed by resource requests operator
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=resourcerequests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=resourcerequests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters/status,verbs=get;update;patch

// needed to approve the certificates
// +kubebuilder:rbac:groups=certificates.k8s.io,resources=certificatesigningrequests,verbs=get;list;watch
// +kubebuilder:rbac:groups=certificates.k8s.io,resources=certificatesigningrequests/status,verbs=update
// +kubebuilder:rbac:groups=certificates.k8s.io,resources=certificatesigningrequests/approval,verbs=update
// +kubebuilder:rbac:groups=certificates.k8s.io,resourceNames=kubernetes.io/kubelet-serving,resources=signers,verbs=approve

// role
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=configmaps,verbs=get;watch;list;update;patch

// Reconcile adds/removes NamespaceMap finalizer, and checks differences
// between DesiredMapping and CurrentMapping in order to create/delete remote Namespaces if it is necessary.
func (r *NamespaceMapReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	namespaceMap := &mapsv1alpha1.NamespaceMap{}
	if err := r.Get(ctx, req.NamespacedName, namespaceMap); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("The NamespaceMap '%s' doesn't exist anymore", req.Name)
			return ctrl.Result{}, nil
		}
		klog.Errorf("%s --> Unable to get NamespaceMap '%s'", err, req.Name)
		return ctrl.Result{}, err
	}

	// If the NamespaceMap is requested to be deleted
	if !namespaceMap.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.namespaceMapDeletionProcess(ctx, namespaceMap)
	}

	// If someone deletes the namespaceMap, then it is necessary to remove all remote namespaces
	// associated with this resource before deleting it, so a finalizer is necessary.
	if err := r.SetNamespaceMapControllerFinalizer(ctx, namespaceMap); err != nil {
		return ctrl.Result{}, err
	}

	// Create/Delete remote Namespaces if it is necessary, according to NamespaceMap status.
	if err := r.ensureRemoteNamespaces(ctx, namespaceMap); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: r.RequeueTime}, nil
}

// SetupWithManager monitors only updates on NamespaceMap.
func (r *NamespaceMapReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mapsv1alpha1.NamespaceMap{}).
		Complete(r)
}
