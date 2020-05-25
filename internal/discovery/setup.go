package discovery

import (
	"github.com/netgroup-polito/dronev2/internal/discovery/clients"
	"github.com/netgroup-polito/dronev2/internal/discovery/kubeconfig"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
)

// configure ConfigMap served from credentials-provider to foreign "client" clusters
func (discovery *DiscoveryCtrl) SetupConfigmap() {
	if discovery.config.EnableAdvertisement {
		clientset, err := clients.NewK8sClient()
		if err != nil {
			discovery.Log.Error(err, err.Error())
			os.Exit(1)
		}

		cm, err := clientset.CoreV1().ConfigMaps(discovery.Namespace).Get("credentials-provider-static-content", v1.GetOptions{})
		if err != nil {
			discovery.Log.Error(err, err.Error())
			os.Exit(1)
		}
		cm.Data["config.yaml"], err = kubeconfig.CreateKubeConfig("unauth-user", discovery.Namespace)
		if err != nil {
			discovery.Log.Error(err, err.Error())
			os.Exit(1)
		}
		_, err = clientset.CoreV1().ConfigMaps(discovery.Namespace).Update(cm)
		if err != nil {
			discovery.Log.Error(err, err.Error())
			os.Exit(1)
		}
	}
}
