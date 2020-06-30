package advertisement_operator

import (
	"context"
	protocolv1 "github.com/liqoTech/liqo/api/advertisement-operator/v1"
	pkg "github.com/liqoTech/liqo/pkg/advertisement-operator"
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

type AdvertisementWatcher struct {
	RemoteClient     client.Client
	LocalClient      client.Client
	Scheme           *runtime.Scheme
	HomeClusterId    string
	ForeignClusterId string
}

func WatchAdvertisement(localCRDClient client.Client, scheme *runtime.Scheme, remoteKubeconfig string, sec *v1.Secret, homeClusterId string, foreignClusterId string) {

	config, err := pkg.GetConfig(remoteKubeconfig, nil, sec)
	if err != nil {
		klog.Error(err)
		return
	}

	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: "0",
		Port:               9443,
	})
	if err != nil {
		klog.Errorln(err, "Unable to start remote watcher")
		return
	}

	if err = (&AdvertisementWatcher{
		RemoteClient:     mgr.GetClient(),
		LocalClient:      localCRDClient,
		Scheme:           mgr.GetScheme(),
		HomeClusterId:    homeClusterId,
		ForeignClusterId: foreignClusterId,
	}).SetupWithManager(mgr); err != nil {
		klog.Error(err)
		return
	}
	klog.Info("starting remote advertisement watcher")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		klog.Error(err)
		return
	}
}

// TODO: copied code (from vk); can we create a generic function?
func checkAdvFiltering(object metav1.Object, watchedClusterId string) bool {

	clusterId := strings.Replace(object.GetName(), "advertisement-", "", 1)
	return clusterId == watchedClusterId
}

func (r *AdvertisementWatcher) SetupWithManager(mgr ctrl.Manager) error {

	generationChangedPredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return checkAdvFiltering(e.Meta, r.HomeClusterId)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return checkAdvFiltering(e.Meta, r.HomeClusterId)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return checkAdvFiltering(e.MetaNew, r.HomeClusterId)
		},
		GenericFunc: nil,
	}
	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(generationChangedPredicate).
		For(&protocolv1.Advertisement{}).
		Complete(r)
}

// triggered by events on the advertisement created by the broadcaster on the remote cluster
func (r *AdvertisementWatcher) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()

	// get remote advertisement
	var adv protocolv1.Advertisement
	if err := r.RemoteClient.Get(ctx, req.NamespacedName, &adv); err != nil {
		if kerror.IsNotFound(err) {
			// reconcile was triggered by a delete request
			klog.Info("Adv deleted")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		} else {
			return ctrl.Result{}, err
		}
	}

	// TODO: may be omitted
	if adv.Spec.ClusterId != r.HomeClusterId {
		// this is not the Advertisement created by the broadcaster
		return ctrl.Result{}, nil
	}

	// Reconcile hasn't been triggered by a modification of the tunnelEndpoint creator
	if adv.Status.RemoteRemappedPodCIDR == "" {
		return ctrl.Result{}, nil
	}

	// get the advertisement of the foreign cluster (stored in the local cluster)
	var foreignClusterAdv protocolv1.Advertisement
	namespacedName := types.NamespacedName{
		Namespace: "default",
		Name:      "advertisement-" + r.ForeignClusterId,
	}
	if err := r.LocalClient.Get(ctx, namespacedName, &foreignClusterAdv); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	}

	foreignClusterAdv.Status.LocalRemappedPodCIDR = adv.Status.RemoteRemappedPodCIDR
	if err := r.LocalClient.Status().Update(ctx, &foreignClusterAdv); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	}
	klog.Info("correctly set status of foreign cluster " + r.ForeignClusterId + " advertisement")
	return ctrl.Result{}, nil
}
