package resourcerequestoperator

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
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
	timeToLive  = 30 * time.Minute
)

// +kubebuilder:rbac:groups=discovery.liqo.io,resources=resourceRequests,verbs=get;list;watch;create;update;patch;
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=resourceRequests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sharing.liqo.io,resources=resourceOffers,verbs=get;list;watch;create;update;patch;
// +kubebuilder:rbac:groups=sharing.liqo.io,resources=resourceOffers/status,verbs=get;update;patch

// Reconcile is the main function of the controller which reconciles ResourceRequest resources.
func (r *ResourceRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var resourceRequest discoveryv1alpha1.ResourceRequest
	err := r.Get(ctx, req.NamespacedName, &resourceRequest)
	if err != nil {
		klog.Errorf("%s -> unable to get resourceRequest %s: %s", r.ClusterID, req.NamespacedName, err)
		return ctrl.Result{}, nil
	}

	offerErr := r.generateResourceOffer(&resourceRequest)
	if offerErr != nil {
		klog.Errorf("%s -> Error generating resourceOffer: %s", r.ClusterID, offerErr)
		return ctrl.Result{}, offerErr
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
		WithEventFilter(p).
		For(&discoveryv1alpha1.ResourceRequest{}).
		Complete(r)
}
