package search_domain_operator

import (
	"errors"
	discoveryv1alpha1 "github.com/liqotech/liqo/api/discovery/v1alpha1"
	"github.com/liqotech/liqo/internal/discovery"
	"github.com/liqotech/liqo/pkg/crdClient"
	v1 "k8s.io/api/core/v1"
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
}

func (r *SearchDomainReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
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

	update := false

	txts, err := Wan(r.DiscoveryCtrl.Config.DnsServer, sd.Spec.Domain, false)
	if err != nil {
		klog.Error(err, err.Error())
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: r.requeueAfter,
		}, err
	}
	fcs := r.DiscoveryCtrl.UpdateForeign(txts, sd)
	if len(fcs) > 0 {
		// new FCs added, so update the list
		AddToList(sd, ForeignClustersToObjectReferences(fcs))
		update = true
	}

	// check deletion, ForeignClusters that are no more in DNS list will be deleted
	toDelete, err := r.CheckForDeletion(sd, txts)
	if err != nil {
		klog.Error(err, err.Error())
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: r.requeueAfter,
		}, err
	}
	if len(toDelete) > 0 {
		for _, fcName := range toDelete {
			err := r.crdClient.Resource("foreignclusters").Delete(fcName, metav1.DeleteOptions{})
			if err != nil {
				klog.Error(err, err.Error())
				continue
			}
			// find index of element to delete
			index := -1
			for i, fc := range sd.Status.ForeignClusters {
				if fc.Name == fcName {
					index = i
					break
				}
			}
			if index < 0 || index > len(sd.Status.ForeignClusters) {
				klog.Error("index out of range on FC deletion")
				continue
			}
			// delete element from list
			sd.Status.ForeignClusters[index] = sd.Status.ForeignClusters[len(sd.Status.ForeignClusters)-1]
			sd.Status.ForeignClusters = sd.Status.ForeignClusters[:len(sd.Status.ForeignClusters)-1]
		}
		update = true
	}

	if update {
		_, err = r.crdClient.Resource("searchdomains").Update(sd.Name, sd, metav1.UpdateOptions{})
		if err != nil {
			klog.Error(err, err.Error())
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: r.requeueAfter,
			}, err
		}
	}

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

func ForeignClustersToObjectReferences(fcs []*discoveryv1alpha1.ForeignCluster) []v1.ObjectReference {
	refs := []v1.ObjectReference{}
	for _, fc := range fcs {
		refs = append(refs, v1.ObjectReference{
			Kind:       "ForeignCluster",
			APIVersion: "discovery.liqo.io/v1alpha1",
			Name:       fc.Name,
			UID:        fc.UID,
		})
	}
	return refs
}

func AddToList(sd *discoveryv1alpha1.SearchDomain, refs []v1.ObjectReference) {
	for _, ref := range refs {
		contains := false
		for _, fc := range sd.Status.ForeignClusters {
			if fc.Name == ref.Name {
				contains = true
				break
			}
		}
		if !contains {
			sd.Status.ForeignClusters = append(sd.Status.ForeignClusters, ref)
		}
	}
}

func (r *SearchDomainReconciler) CheckForDeletion(sd *discoveryv1alpha1.SearchDomain, txts []*discovery.TxtData) ([]string, error) {
	toDelete := []string{}
	for _, fc := range sd.Status.ForeignClusters {
		contains := false
		for _, txt := range txts {
			if txt.ID == fc.Name {
				contains = true
				break
			}
		}
		if !contains {
			toDelete = append(toDelete, fc.Name)
		}
	}
	return toDelete, nil
}
