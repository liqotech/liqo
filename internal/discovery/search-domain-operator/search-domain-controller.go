package search_domain_operator

import (
	"context"
	"errors"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/internal/discovery"
	"github.com/liqotech/liqo/pkg/crdClient"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"time"
)

type SearchDomainReconciler struct {
	Scheme *runtime.Scheme

	requeueAfter  time.Duration
	crdClient     *crdClient.CRDClient
	DiscoveryCtrl *discovery.DiscoveryCtrl

	DnsAddress string
}

func (r *SearchDomainReconciler) Reconcile(_ context.Context, req ctrl.Request) (ctrl.Result, error) {
	klog.Info("Reconciling SearchDomain " + req.Name)

	tmp, err := r.crdClient.Resource("searchdomains").Get(req.Name, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// has been deleted
			return ctrl.Result{}, nil
		}
		klog.Error(err, err.Error())
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: r.requeueAfter,
		}, err
	}
	sd, ok := tmp.(*discoveryv1alpha1.SearchDomain)
	if !ok {
		err := errors.New("retrieved resource is not a SearchDomain")
		klog.Error(err, err.Error())
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: r.requeueAfter,
		}, err
	}

	authData, err := LoadAuthDataFromDNS(r.DnsAddress, sd.Spec.Domain)
	if err != nil {
		klog.Error(err, err.Error())
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: r.requeueAfter,
		}, err
	}
	r.DiscoveryCtrl.UpdateForeignWAN(authData, sd)

	klog.Info("SearchDomain " + req.Name + " successfully reconciled")
	return ctrl.Result{
		Requeue:      true,
		RequeueAfter: r.requeueAfter,
	}, nil
}

func (r *SearchDomainReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&discoveryv1alpha1.SearchDomain{}).
		Complete(r)
}
