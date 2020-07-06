package search_domain_operator

import (
	discoveryv1 "github.com/liqoTech/liqo/api/discovery/v1"
	"github.com/liqoTech/liqo/internal/discovery"
	"github.com/liqoTech/liqo/pkg/crdClient/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog"
	"time"
)

type SearchDomainReconciler struct {
	requeueAfter  time.Duration
	crdClient     *v1alpha1.CRDClient
	DiscoveryCtrl *discovery.DiscoveryCtrl
}

func (r *SearchDomainReconciler) Reconcile(event watch.Event, sd *discoveryv1.SearchDomain) (time.Duration, error) {
	klog.Info("Reconciling SearchDomain " + sd.Name)

	if event.Type == watch.Added || event.Type == watch.Modified {
		txts, err := Wan(r.DiscoveryCtrl.Config.DnsServer, sd.Spec.Domain)
		if err != nil {
			klog.Error(err, err.Error())
			return 0, err
		}
		fcs := r.DiscoveryCtrl.UpdateForeign(txts, sd)
		if len(fcs) > 0 {
			sd.Status.ForeignClusters = append(sd.Status.ForeignClusters, ForeignClustersToObjectReferences(fcs)...)
			_, err = r.crdClient.Resource("searchdomains").Update(sd.Name, sd, metav1.UpdateOptions{})
			if err != nil {
				klog.Error(err, err.Error())
				return 0, err
			}
		}
	}

	klog.Info("SearchDomain " + sd.Name + " successfully reconciled")
	return r.requeueAfter, nil
}

func ForeignClustersToObjectReferences(fcs []*discoveryv1.ForeignCluster) []v1.ObjectReference {
	refs := []v1.ObjectReference{}
	for _, fc := range fcs {
		refs = append(refs, v1.ObjectReference{
			Kind:       "ForeignCluster",
			APIVersion: "discovery.liqo.io/v1",
			Name:       fc.Name,
			UID:        fc.UID,
		})
	}
	return refs
}
