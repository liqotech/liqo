package discovery

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog"
	"os"
)

var trustedWatchRunning = false

func (discovery *DiscoveryCtrl) WatchTrustedCAs() error {
	// start it only once
	if trustedWatchRunning {
		return nil
	}
	trustedWatchRunning = true

	// if trusted-ca ConfigMap changes, this component has to be reloaded
	wc, err := discovery.crdClient.Client().CoreV1().ConfigMaps(discovery.Namespace).Watch(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.Error(err, err.Error())
		return err
	}
	go func() {
		ch := wc.ResultChan()
		for event := range ch {
			if event.Type == watch.Modified {
				klog.Info("Trusted CA modified, reload discovery component")
				os.Exit(0)
			}
		}
	}()
	return nil
}
