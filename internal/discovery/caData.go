package discovery

import (
	"context"
	goerrors "errors"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog"
	"os"
)

func (discovery *DiscoveryCtrl) SetupCaData() error {
	err := discovery.WatchTrustedCAs()
	if err != nil {
		klog.Error(err, err.Error())
		return err
	}

	// provide local CA
	_, err = discovery.crdClient.Client().CoreV1().Secrets(discovery.Namespace).Get(context.TODO(), "ca-data", metav1.GetOptions{})
	if err == nil {
		// already exists
		return err
	}

	// get CaData from Secrets
	secrets, err := discovery.crdClient.Client().CoreV1().Secrets(discovery.Namespace).List(context.TODO(), metav1.ListOptions{
		Limit:         1,
		FieldSelector: "type=kubernetes.io/service-account-token",
	})
	if err != nil {
		klog.Error(err, err.Error())
		return err
	}
	if len(secrets.Items) == 0 {
		klog.Error(nil, "No service account found, I can't get CaData")
		return goerrors.New("No service account found, I can't get CaData")
	}
	if secrets.Items[0].Data["ca.crt"] == nil {
		klog.Error(nil, "Cannot get CaData from secret")
		return goerrors.New("Cannot get CaData from secret")
	}

	secret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ca-data",
		},
		Data: map[string][]byte{
			"ca.crt": secrets.Items[0].Data["ca.crt"],
		},
	}
	_, err = discovery.crdClient.Client().CoreV1().Secrets(discovery.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		klog.Error(err, err.Error())
		return err
	}
	return nil
}

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
