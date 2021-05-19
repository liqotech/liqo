package resourceoffercontroller

import (
	"time"

	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// NewResourceOfferController creates and returns a new reconciler for the ResourceOffers.
func NewResourceOfferController(mgr manager.Manager, resyncPeriod time.Duration) *ResourceOfferReconciler {
	return &ResourceOfferReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),

		eventsRecorder: mgr.GetEventRecorderFor("ResourceOffer"),

		resyncPeriod: resyncPeriod,
	}
}
