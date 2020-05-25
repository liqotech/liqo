package kubernetes

import (
	"context"
	"github.com/go-logr/logr"
	advv1 "github.com/netgroup-polito/dronev2/api/advertisement-operator/v1"
	"github.com/netgroup-polito/dronev2/internal/node"
	v1 "k8s.io/api/core/v1"
	kerror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"strings"
)

type VirtualNodeReconciler struct {
	client.Client
	Log            logr.Logger
	Scheme         *runtime.Scheme
	provider       *KubernetesProvider
	nodeController *node.NodeController
	ready          chan bool
}

// NewVirtualNodeReconciler returns a new instance of VirtualNodeReconciler already set-up
func NewVirtualNodeReconciler(p *KubernetesProvider, n *node.NodeController) (chan bool, error) {
	ready := make(chan bool, 1)
	return ready, (&VirtualNodeReconciler{
		Client:         p.manager.GetClient(),
		Log:            ctrl.Log.WithName("controllers").WithName("Advertisement"),
		Scheme:         p.manager.GetScheme(),
		provider:       p,
		nodeController: n,
		ready:          ready,
	}).SetupWithManager(p.manager)
}

// The reconciliation function; every time this function is called,
// the node status is updated by means of r.updateFromAdv
func (r *VirtualNodeReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("vk", req.NamespacedName)

	// get advertisement
	var adv advv1.Advertisement
	if err := r.Get(ctx, req.NamespacedName, &adv); err != nil {
		if kerror.IsNotFound(err) {
			// reconcile was triggered by a delete request
			log.Info("Adv deleted")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		} else {
			return ctrl.Result{}, err
		}
	}

	if err := r.updateFromAdv(ctx, adv); err != nil {
		klog.Info(err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// checkAdvFiltering filters the triggering of the reconcile function
func (r *VirtualNodeReconciler) checkAdvFiltering(object metav1.Object) bool {

	clusterId := strings.Replace(object.GetName(), "advertisement-", "", 1)
	if clusterId == r.provider.foreignClusterId {
		return true
	}

	return false
}

func (r *VirtualNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {

	var generationChangedPredicate = predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return r.checkAdvFiltering(e.MetaNew)
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return r.checkAdvFiltering(e.Meta)
		},
	}

	if err := ctrl.NewControllerManagedBy(mgr).
		For(&advv1.Advertisement{}).
		WithEventFilter(generationChangedPredicate).
		Complete(r); err != nil {
		return err
	}

	go func() {
		if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
			klog.Fatal(err)
		}
	}()

	return nil
}

// Initialization of the Virtual kubelet, that implies:
// * clientProvider initialization
// * podWatcher launch
func (r *VirtualNodeReconciler) initVirtualKubelet(adv advv1.Advertisement) error {
	klog.Info("vk initializing")

	if adv.Status.RemoteRemappedPodCIDR != "None" {
		r.provider.RemappedPodCidr = adv.Status.RemoteRemappedPodCIDR
	} else {
		r.provider.RemappedPodCidr = adv.Spec.Network.PodCIDR
	}

	r.ready <- true

	return nil
}

// updateFromAdv gets and  advertisement and updates the node status accordingly
func (r *VirtualNodeReconciler) updateFromAdv(ctx context.Context, adv advv1.Advertisement) error {

	if r.provider.initialized == false {
		if err := r.initVirtualKubelet(adv); err != nil {
			return err
		}
	}

	var no v1.Node
	if err := r.Get(ctx, types.NamespacedName{Name:r.provider.nodeName}, &no); err != nil {
		return err
	}

	if r.provider.initialized == false {
		r.provider.initialized = true
		if err := r.setAnnotation(ctx, "cluster-id", r.provider.foreignClusterId, &no); err != nil {
			return err
		}
	}

	if no.Status.Capacity == nil {
		no.Status.Capacity = v1.ResourceList{}
	}
	if no.Status.Allocatable == nil {
		no.Status.Allocatable = v1.ResourceList{}
	}
	for k, v := range adv.Spec.ResourceQuota.Hard {
		no.Status.Capacity[k] = v
		no.Status.Allocatable[k] = v
	}

	no.Status.Images = []v1.ContainerImage{}
	for _, i := range adv.Spec.Images {
		no.Status.Images = append(no.Status.Images, i)
	}

	return r.nodeController.UpdateNodeFromOutside(ctx, false, &no)
}

func (r *VirtualNodeReconciler) setAnnotation(ctx context.Context, k, v string, no *v1.Node) error {
	metav1.SetMetaDataAnnotation(&no.ObjectMeta, k, v)

	if err := r.Update(ctx, no); err != nil {
		return err
	}

	return nil
}
