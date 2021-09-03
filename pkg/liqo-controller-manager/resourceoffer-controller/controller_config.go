package resourceoffercontroller

import (
	"time"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/liqotech/liqo/pkg/vkMachinery/forge"
)

// NewResourceOfferController creates and returns a new reconciler for the ResourceOffers.
func NewResourceOfferController(
	mgr manager.Manager, clusterID string,
	resyncPeriod time.Duration, liqoNamespace string,
	virtualKubeletOpts *forge.VirtualKubeletOpts,
	disableAutoAccept bool) *ResourceOfferReconciler {
	return &ResourceOfferReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),

		eventsRecorder: mgr.GetEventRecorderFor("ResourceOffer"),
		clusterID:      clusterID,

		liqoNamespace: liqoNamespace,

		virtualKubeletOpts: virtualKubeletOpts,
		disableAutoAccept:  disableAutoAccept,

		resyncPeriod: resyncPeriod,
	}
}
