package natmappingoperator

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	liqonet "github.com/liqotech/liqo/pkg/liqonet"
)

// NatMappingController reconciles a NatMapping object.
type NatMappingController struct {
	client.Client
	Scheme *runtime.Scheme
	liqonet.IPTablesHandler
	readyClusters map[string]struct{}
}

var result = ctrl.Result{
	Requeue:      false,
	RequeueAfter: 5 * time.Second,
}

//+kubebuilder:rbac:groups=net.liqo.io,resources=natmappings,verbs=get;list;watch;create;update;patch;delete

// Reconcile NatMapping resource.
func (npc *NatMappingController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var nm netv1alpha1.NatMapping
	if err := npc.Get(ctx, req.NamespacedName, &nm); apierrors.IsNotFound(err) {
		// Reconcile was triggered by a delete request
		return ctrl.Result{}, client.IgnoreNotFound(err)
	} else if err != nil {
		// Unknown error
		klog.Errorf("an error occurred while getting resource %s: %s", req.NamespacedName, err.Error())
		return result, err
	}
	// There's no need of a pre-delete logic since IPTables rules for cluster are removed by the
	// tunnel-operator after the un-peer.

	// Is the remote cluster tunnel ready? If not, do nothing
	if _, ready := npc.readyClusters[nm.Spec.ClusterID]; !ready {
		return ctrl.Result{}, fmt.Errorf("tunnel for cluster %s is not ready", nm.Spec.ClusterID)
	}
	// If the tunnel is ready, then insert rules
	if err := npc.EnsureChainRulespecsPerNm(&nm); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	}
	if err := npc.EnsurePreroutingRulesPerNm(&nm); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (npc *NatMappingController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&netv1alpha1.NatMapping{}).
		Complete(npc)
}

// NewNatMappingController returns a NAT mapping controller istance.
func NewNatMappingController(mgr ctrl.Manager, readyClusters map[string]struct{}) (*NatMappingController, error) {
	iptablesHandler, err := liqonet.NewIPTablesHandler()
	if err != nil {
		return nil, err
	}
	return &NatMappingController{
		Client:          mgr.GetClient(),
		IPTablesHandler: iptablesHandler,
		readyClusters:   readyClusters,
	}, nil
}
