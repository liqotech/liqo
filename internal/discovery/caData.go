package discovery

import (
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"os"
)

func (discovery *DiscoveryCtrl) SetupCaData() {
	_, err := discovery.client.CoreV1().Secrets(discovery.Namespace).Get("ca-data", metav1.GetOptions{})
	if err == nil {
		// already exists
		return
	}

	// get CaData from Secrets
	secrets, err := discovery.client.CoreV1().Secrets(discovery.Namespace).List(metav1.ListOptions{
		Limit:         1,
		FieldSelector: "type=kubernetes.io/service-account-token",
	})
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}
	if len(secrets.Items) == 0 {
		klog.Error(nil, "No service account found, I can't get CaData")
		os.Exit(1)
	}
	if secrets.Items[0].Data["ca.crt"] == nil {
		klog.Error(nil, "Cannot get CaData from secret")
		os.Exit(1)
	}

	secret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ca-data",
		},
		Data: map[string][]byte{
			"ca.crt": secrets.Items[0].Data["ca.crt"],
		},
	}
	_, err = discovery.client.CoreV1().Secrets(discovery.Namespace).Create(secret)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}
}
