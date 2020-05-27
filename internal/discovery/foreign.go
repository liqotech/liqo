package discovery

import (
	v1 "github.com/liqoTech/liqo/api/discovery/v1"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
)

func (discovery *DiscoveryCtrl) UpdateForeign(data []*TxtData) {
	for _, txtData := range data {
		_, err := discovery.clientDiscovery.ForeignClusters().Get(txtData.ID, metav1.GetOptions{})
		if err == nil {
			// already exists
			continue
		}
		resp, err := http.Get(txtData.Config.Url)
		if err != nil {
			discovery.Log.Error(err, err.Error())
			continue
		}
		config, err := ioutil.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			discovery.Log.Error(err, err.Error())
			continue
		}
		_, err = discovery.createForeign(config, txtData.ID, txtData.Config.Namespace)
		if err != nil {
			discovery.Log.Error(err, err.Error())
			continue
		}
		discovery.Log.Info("ForeignCluster " + txtData.ID + " created")
	}
}

func (discovery *DiscoveryCtrl) createForeign(config []byte, clusterID string, foreignNamespace string) (*v1.ForeignCluster, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterID,
		},
		Data: map[string][]byte{
			"kubeconfig": config,
		},
	}
	secret, err := discovery.client.CoreV1().Secrets(discovery.Namespace).Create(secret)
	if err != nil {
		return nil, err
	}
	fc := &v1.ForeignCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterID,
		},
		Spec: v1.ForeignClusterSpec{
			ClusterID: clusterID,
			Namespace: foreignNamespace,
			KubeConfigRef: corev1.ObjectReference{
				Kind:       secret.Kind,
				Namespace:  secret.Namespace,
				Name:       secret.Name,
				UID:        secret.UID,
				APIVersion: secret.APIVersion,
			},
			Join: discovery.config.AutoJoin,
		},
	}
	fc, err = discovery.clientDiscovery.ForeignClusters().Create(fc)
	if err != nil {
		return nil, err
	}
	secret.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: fc.APIVersion,
			Kind:       fc.APIVersion,
			Name:       fc.Name,
			UID:        fc.UID,
		},
	}
	_, err = discovery.client.CoreV1().Secrets(discovery.Namespace).Update(secret)
	return fc, err
}
