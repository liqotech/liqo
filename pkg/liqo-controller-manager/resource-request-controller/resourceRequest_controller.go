package resourcerequestoperator

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	crdreplicator "github.com/liqotech/liqo/internal/crdReplicator"
)

// ResourceRequestReconciler reconciles a ResourceRequest object.
type ResourceRequestReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	ClusterID   string
	Broadcaster *Broadcaster
}

const (
	offerPrefix = "resourceoffer-"
)

// +kubebuilder:rbac:groups=sharing.liqo.io,resources=resourceoffers,verbs=get;list;watch;create;update;patch;
// +kubebuilder:rbac:groups=sharing.liqo.io,resources=resourceoffers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=resourcerequests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=resourcerequests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters/status,verbs=get;update;patch

// +kubebuilder:rbac:groups=capsule.clastix.io,resources=tenants,verbs=get;list;watch;create;update;patch;delete;

// Reconcile is the main function of the controller which reconciles ResourceRequest resources.
func (r *ResourceRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	var resourceRequest discoveryv1alpha1.ResourceRequest
	err = r.Get(ctx, req.NamespacedName, &resourceRequest)
	if err != nil {
		klog.Errorf("unable to get resourceRequest %s: %s", req.NamespacedName, err)
		return ctrl.Result{}, nil
	}

	remoteClusterID := resourceRequest.Spec.ClusterIdentity.ClusterID

	var requireSpecUpdate bool
	// ensure the ForeignCluster existence, if not exists we have to add a new one
	// with IncomingPeering discovery method.
	requireSpecUpdate, err = r.ensureForeignCluster(ctx, &resourceRequest)
	if err != nil {
		klog.Errorf("%s -> Error generating resourceOffer: %s", remoteClusterID, err)
		return ctrl.Result{}, err
	}

	if requireTenantDeletion(&resourceRequest) {
		if err = r.ensureTenantDeletion(ctx, &resourceRequest); err != nil {
			klog.Errorf("%s -> Error deleting Tenant: %s", remoteClusterID, err)
			return ctrl.Result{}, err
		}
		requireSpecUpdate = true
	} else {
		newRequireSpecUpdate := false
		if newRequireSpecUpdate, err = r.ensureTenant(ctx, &resourceRequest); err != nil {
			klog.Errorf("%s -> Error creating Tenant: %s", remoteClusterID, err)
			return ctrl.Result{}, err
		}
		requireSpecUpdate = requireSpecUpdate || newRequireSpecUpdate
	}

	if requireSpecUpdate {
		if err = r.Client.Update(ctx, &resourceRequest); err != nil {
			klog.Error(err)
			return ctrl.Result{}, err
		}
		// always requeue after a spec update
		return ctrl.Result{}, err
	}

	defer func() {
		newErr := r.Client.Status().Update(ctx, &resourceRequest)
		if newErr != nil {
			klog.Error(newErr)
			err = newErr
		}
	}()

	if resourceRequest.Spec.WithdrawalTimestamp.IsZero() {
		r.Broadcaster.enqueueForCreationOrUpdate(remoteClusterID)
		if err != nil {
			klog.Errorf("%s -> Error generating resourceOffer: %s", remoteClusterID, err)
			return ctrl.Result{}, err
		}
	} else {
		err = r.invalidateResourceOffer(ctx, &resourceRequest)
		if err != nil {
			klog.Errorf("%s -> Error invalidating resourceOffer: %s", remoteClusterID, err)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager is the setup function of the controller.
func (r *ResourceRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// generate the predicate to filter just the ResourceRequest created by the remote cluster checking crdReplicator labels
	p, err := predicate.LabelSelectorPredicate(crdreplicator.ReplicatedResourcesLabelSelector)
	if err != nil {
		klog.Error(err)
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&discoveryv1alpha1.ResourceRequest{}, builder.WithPredicates(p)).
		Owns(&sharingv1alpha1.ResourceOffer{}).
		Complete(r)
}
