package advertisement_operator

import (
	"context"
	protocolv1 "github.com/liqoTech/liqo/api/advertisement-operator/v1"
	policyv1 "github.com/liqoTech/liqo/api/cluster-config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (b *AdvertisementBroadcaster) WatchConfiguration(kubeconfigPath string) error {
	configClient, err := policyv1.CreateClusterConfigClient(kubeconfigPath)
	if err != nil {
		b.Log.Info(err.Error())
		return err
	}

	watcher, err := configClient.Resource("clusterconfigs").Watch(metav1.ListOptions{})
	if err != nil {
		return err
	}

	go func() {
		for event := range watcher.ResultChan() {
			configuration, ok := event.Object.(*policyv1.ClusterConfig)
			if !ok {
				continue
			}

			switch event.Type {
			case watch.Added, watch.Modified:
				b.ClusterConfig.ResourceSharingPercentage = configuration.Spec.ResourceSharingPercentage
			case watch.Deleted:
				// TODO: set default config?
			}
		}
	}()
	return nil
}

func (r *AdvertisementReconciler) WatchConfiguration(kubeconfigPath string) error {
	configClient, err := policyv1.CreateClusterConfigClient(kubeconfigPath)
	if err != nil {
		r.Log.Info(err.Error())
		return err
	}

	watcher, err := configClient.Resource("clusterconfigs").Watch(metav1.ListOptions{})
	if err != nil {
		return err
	}

	go func() {
		for event := range watcher.ResultChan() {
			configuration, ok := event.Object.(*policyv1.ClusterConfig)
			if !ok {
				continue
			}

			// if first time, copy
			if r.ClusterConfig == defaultClusterConfig {
				r.ClusterConfig = configuration.Spec
			}

			switch event.Type {
			case watch.Added, watch.Modified:
				if configuration.Spec.MaxAcceptableAdvertisement > r.ClusterConfig.MaxAcceptableAdvertisement {
					// the maximum has increased: check if there are refused advertisements which now can be accepted
					r.ClusterConfig = configuration.Spec
					var advList protocolv1.AdvertisementList
					err = r.Client.List(context.Background(), &advList)
					if err != nil {
						r.Log.Error(err, "Unable to apply configuration: error listing Advertisements")
						continue
					}
					for _, adv := range advList.Items {
						if adv.Status.AdvertisementStatus == "REFUSED" {
							r.checkAdvertisement(context.Background(), r.Log, &adv)
						}
					}
				} else {
					// the maximum has decreased: if the accepted advertisements are too many, delete some of them
					r.ClusterConfig = configuration.Spec
					if r.ClusterConfig.MaxAcceptableAdvertisement < r.AcceptedAdvNum {
						var advList protocolv1.AdvertisementList
						err = r.Client.List(context.Background(), &advList, &client.ListOptions{})
						if err != nil {
							r.Log.Error(err, "Unable to apply configuration: error listing Advertisements")
							continue
						}
						for i := 0; i < int(r.AcceptedAdvNum-r.ClusterConfig.MaxAcceptableAdvertisement); i++ {
							adv := advList.Items[i]
							if adv.Status.AdvertisementStatus == "ACCEPTED" {
								err = r.Client.Delete(context.Background(), &adv)
								if err != nil {
									r.Log.Error(err, "Unable to apply configuration: error deleting Advertisement "+adv.Name)
								}
								r.AcceptedAdvNum--
							}
						}
					}
				}
			case watch.Deleted:
				// TODO: set default config?
			}
		}
	}()
	return nil
}
