package discovery

import (
	"github.com/netgroup-polito/dronev2/internal/discovery/clients"
	"github.com/netgroup-polito/dronev2/internal/discovery/kubeconfig"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
)

// configure ConfigMap served from credentials-provider to foreign "client" clusters
func SetupConfigmap() {
	dc := GetDiscoveryConfig()
	if dc.EnableAdvertisement {
		clientset, err := clients.NewK8sClient()
		if err != nil {
			Log.Error(err, err.Error())
			os.Exit(1)
		}

		cm, err := clientset.CoreV1().ConfigMaps(Namespace).Get("credentials-provider-static-content", v1.GetOptions{})
		if err != nil {
			Log.Error(err, err.Error())
			os.Exit(1)
		}
		cm.Data["config.yaml"], err = kubeconfig.CreateKubeConfig("unauth-user", Namespace)
		if err != nil {
			Log.Error(err, err.Error())
			os.Exit(1)
		}
		_, err = clientset.CoreV1().ConfigMaps(Namespace).Update(cm)
		if err != nil {
			Log.Error(err, err.Error())
			os.Exit(1)
		}
	}
}
