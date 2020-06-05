package kubernetes

import (
	"errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

func (p *KubernetesProvider) manageCmEvent(event watch.Event) error {
	var err error

	cm, ok := event.Object.(*corev1.ConfigMap)
	if !ok {
		return errors.New("cannot cast object to configMap")
	}

	nattedNS, err := p.NatNamespace(cm.Namespace, false)
	if err != nil {
		return err
	}

	switch event.Type {
	case watch.Added:
		_, err := p.foreignClient.Client().CoreV1().ConfigMaps(nattedNS).Get(cm.Name, metav1.GetOptions{})
		if err != nil {
			p.log.Info("remote cm " + cm.Name + " doesn't exist: creating it")

			if err = CreateConfigMap(p.foreignClient.Client(), cm, nattedNS); err != nil {
				p.log.Error(err, "unable to create configMap "+cm.Name+" on cluster "+p.foreignClusterId)
			} else {
				p.log.Info("correctly created configMap " + cm.Name + " on cluster " + p.foreignClusterId)
			}
		}

	case watch.Modified:
		if err = UpdateConfigMap(p.foreignClient.Client(), cm, nattedNS); err != nil {
			p.log.Error(err, "unable to update configMap "+cm.Name+" on cluster "+p.foreignClusterId)
		} else {
			p.log.Info("correctly updated configMap " + cm.Name + " on cluster " + p.foreignClusterId)
		}

	case watch.Deleted:
		if err = DeleteConfigMap(p.foreignClient.Client(), cm, nattedNS); err != nil {
			p.log.Error(err, "unable to delete configMap "+cm.Name+" on cluster "+p.foreignClusterId)
		} else {
			p.log.Info("correctly deleted configMap " + cm.Name + " on cluster " + p.foreignClusterId)
		}
	}
	return nil
}

func CreateConfigMap(c *kubernetes.Clientset, cm *corev1.ConfigMap, namespace string) error {
	cmRemote := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cm.Name,
			Namespace:   namespace,
			Labels:      cm.Labels,
			Annotations: nil,
		},
		Data:       cm.Data,
		BinaryData: cm.BinaryData,
	}

	if cmRemote.Labels == nil {
		cmRemote.Labels = make(map[string]string)
	}
	cmRemote.Labels["liqo/reflection"] = "reflected"

	_, err := c.CoreV1().ConfigMaps(namespace).Create(&cmRemote)

	return err
}

func UpdateConfigMap(c *kubernetes.Clientset, cm *corev1.ConfigMap, namespace string) error {
	cmOld, err := c.CoreV1().ConfigMaps(namespace).Get(cm.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	cm.SetNamespace(namespace)
	cm.SetResourceVersion(cmOld.ResourceVersion)
	cm.SetUID(cmOld.UID)
	_, err = c.CoreV1().ConfigMaps(namespace).Update(cm)

	return err
}

func DeleteConfigMap(c *kubernetes.Clientset, cm *corev1.ConfigMap, namespace string) error {
	cm.Namespace = namespace
	err := c.CoreV1().ConfigMaps(namespace).Delete(cm.Name, &metav1.DeleteOptions{})

	return err
}
