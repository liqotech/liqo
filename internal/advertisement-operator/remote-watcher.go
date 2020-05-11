package advertisement_operator

import (
	"context"
	"github.com/go-logr/logr"
	protocolv1 "github.com/netgroup-polito/dronev2/api/advertisement-operator/v1"
	pkg "github.com/netgroup-polito/dronev2/pkg/advertisement-operator"
	v1 "k8s.io/api/core/v1"
	kerror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AdvertisementWatcher struct {
	client.Client
	LocalClient      client.Client
	Log              logr.Logger
	Scheme           *runtime.Scheme
	ForeignClusterID string
}

func WatchAdvertisement(localCRDClient client.Client, scheme *runtime.Scheme, remoteKubeconfig string, cm *v1.ConfigMap, foreignClusterId string) {
	config, err := pkg.GetConfig(remoteKubeconfig, cm)
	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: "0",
		Port:               9443,
	})
	if err != nil {
		log.Error(err, "unable to start remote watcher")
		os.Exit(1)
	}

	if err = (&AdvertisementWatcher{
		Client:           mgr.GetClient(),
		LocalClient:      localCRDClient,
		Log:              ctrl.Log.WithName("remote-advertisement-watcher"),
		Scheme:           mgr.GetScheme(),
		ForeignClusterID: foreignClusterId,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create advertisement watcher")
		os.Exit(1)
	}
	log.Info("starting remote advertisement watcher")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func (r *AdvertisementWatcher) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&protocolv1.Advertisement{}).
		Complete(r)
}

// triggered by events on the advertisement created by the broadcaster on the remote cluster
func (r *AdvertisementWatcher) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("remote-advertisement-watcher", req.NamespacedName)

	// get remote advertisement
	var adv protocolv1.Advertisement
	if err := r.Get(ctx, req.NamespacedName, &adv); err != nil {
		if kerror.IsNotFound(err) {
			// reconcile was triggered by a delete request
			log.Info("Adv deleted")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		} else {
			return ctrl.Result{}, err
		}
	}

	// Reconcile hasn't been triggered by a modification of the tunnelEndpoint creator
	if adv.Status.RemoteRemappedPodCIDR == "" {
		return ctrl.Result{}, nil
	}

	// get the advertisement of the foreign cluster (stored in the local cluster)
	var foreignClusterAdv protocolv1.Advertisement
	namespacedName := types.NamespacedName{
		Namespace: "default",
		Name:      "advertisement-" + r.ForeignClusterID,
	}
	if err := r.LocalClient.Get(ctx, namespacedName, &foreignClusterAdv); err != nil {
		log.Info("unable to get foreign cluster advertisement")
	}

	foreignClusterAdv.Status.LocalRemappedPodCIDR = adv.Status.RemoteRemappedPodCIDR
	if err := r.LocalClient.Status().Update(ctx, &foreignClusterAdv); err != nil{
		log.Error(err, "unable to update Advertisement status")
	}

	return ctrl.Result{}, nil
}
