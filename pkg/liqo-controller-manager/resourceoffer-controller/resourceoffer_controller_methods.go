package resourceoffercontroller

import (
	"context"
	"reflect"
	"sync"

	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	crdclient "github.com/liqotech/liqo/pkg/crdClient"
	"github.com/liqotech/liqo/pkg/utils"
	foreigncluster "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

// WatchConfiguration watches a ClusterConfig for reconciling updates on ClusterConfig.
func (r *ResourceOfferReconciler) WatchConfiguration(kubeconfigPath string, localCrdClient *crdclient.CRDClient, wg *sync.WaitGroup) {
	defer wg.Done()
	utils.WatchConfiguration(func(configuration *configv1alpha1.ClusterConfig) {
		r.setConfig(configuration)
	}, localCrdClient, kubeconfigPath)
}

func (r *ResourceOfferReconciler) getConfig() *configv1alpha1.ClusterConfig {
	r.configurationMutex.RLock()
	defer r.configurationMutex.RUnlock()

	return r.configuration.DeepCopy()
}

func (r *ResourceOfferReconciler) setConfig(config *configv1alpha1.ClusterConfig) {
	r.configurationMutex.Lock()
	defer r.configurationMutex.Unlock()

	if r.configuration == nil {
		r.configuration = config
		return
	}
	if !reflect.DeepEqual(r.configuration, config) {
		r.configuration = config
	}
}

// setOwnerReference sets owner reference to the related ForeignCluster.
func (r *ResourceOfferReconciler) setOwnerReference(
	ctx context.Context, resourceOffer *sharingv1alpha1.ResourceOffer) (controllerutil.OperationResult, error) {
	// get the foreign cluster by clusterID label
	foreignCluster, err := foreigncluster.GetForeignClusterByID(ctx, r.Client, resourceOffer.Spec.ClusterId)
	if err != nil {
		klog.Error(err)
		return controllerutil.OperationResultNone, err
	}
	return controllerutil.CreateOrUpdate(ctx, r.Client, resourceOffer, func() error {
		// add owner reference, if it is not already set
		if err := controllerutil.SetControllerReference(foreignCluster, resourceOffer, r.Scheme); err != nil {
			klog.Error(err)
			return err
		}
		return nil
	})
}

// setResourceOfferPhase checks if the resource request can be accepted and set its phase accordingly.
func (r *ResourceOfferReconciler) setResourceOfferPhase(
	ctx context.Context, resourceOffer *sharingv1alpha1.ResourceOffer) (controllerutil.OperationResult, error) {
	// we want only to care about resource offers with a pending status
	if resourceOffer.Status.Phase != "" && resourceOffer.Status.Phase != sharingv1alpha1.ResourceOfferPending {
		return controllerutil.OperationResultNone, nil
	}

	return controllerutil.CreateOrPatch(ctx, r.Client, resourceOffer, func() error {
		switch r.getConfig().Spec.AdvertisementConfig.IngoingConfig.AcceptPolicy {
		case configv1alpha1.AutoAcceptMax:
			resourceOffer.Status.Phase = sharingv1alpha1.ResourceOfferAccepted
		case configv1alpha1.ManualAccept:
			// require a manual accept/refuse
			resourceOffer.Status.Phase = sharingv1alpha1.ResourceOfferManualActionRequired
		}
		return nil
	})
}
