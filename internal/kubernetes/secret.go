package kubernetes

import (
	"context"
	"errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog"
)

func (p *KubernetesProvider) manageSecEvent(event watch.Event) error {
	var err error

	sec, ok := event.Object.(*corev1.Secret)
	if !ok {
		return errors.New("cannot cast object to secret")
	}
	klog.V(3).Infof("received %v on secret %v", event.Type, sec.Name)

	nattedNS, err := p.NatNamespace(sec.Namespace, false)
	if err != nil {
		return err
	}

	switch event.Type {
	case watch.Added:
		_, err := p.foreignClient.Client().CoreV1().Secrets(nattedNS).Get(context.TODO(), sec.Name, metav1.GetOptions{})
		if err != nil {
			klog.V(5).Info("remote secret " + sec.Name + " doesn't exist: creating it")

			if err = p.createSecret(sec, nattedNS); err != nil {
				klog.Error(err, "unable to create secret "+sec.Name+" on cluster "+p.foreignClusterId)
			} else {
				klog.V(5).Info("correctly created secret " + sec.Name + " on cluster " + p.foreignClusterId)
			}
		}

	case watch.Modified:
		if err = p.updateSecret(sec, nattedNS); err != nil {
			klog.Error(err, "unable to update secret "+sec.Name+" on cluster "+p.foreignClusterId)
		} else {
			klog.V(5).Info("correctly updated secret " + sec.Name + " on cluster " + p.foreignClusterId)
		}

	case watch.Deleted:
		if err = p.deleteSecret(sec, nattedNS); err != nil {
			klog.Error(err, "unable to delete secret "+sec.Name+" on cluster "+p.foreignClusterId)
		} else {
			klog.V(5).Info("correctly deleted secret " + sec.Name + " on cluster " + p.foreignClusterId)
		}
	}
	return nil
}

func (p *KubernetesProvider) createSecret(sec *corev1.Secret, namespace string) error {
	secRemote := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        sec.Name,
			Namespace:   namespace,
			Labels:      sec.Labels,
			Annotations: nil,
		},
		Data:       sec.Data,
		StringData: sec.StringData,
		Type:       sec.Type,
	}

	if secRemote.Labels == nil {
		secRemote.Labels = make(map[string]string)
	}
	secRemote.Labels["liqo/reflection"] = "reflected"

	_, err := p.foreignClient.Client().CoreV1().Secrets(namespace).Create(context.TODO(), &secRemote, metav1.CreateOptions{})

	return err
}

func (p *KubernetesProvider) updateSecret(sec *corev1.Secret, namespace string) error {
	secOld, err := p.foreignClient.Client().CoreV1().Secrets(namespace).Get(context.TODO(), sec.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	sec2 := sec.DeepCopy()
	sec2.SetNamespace(namespace)
	sec2.SetResourceVersion(secOld.ResourceVersion)
	sec2.SetUID(secOld.UID)
	_, err = p.foreignClient.Client().CoreV1().Secrets(namespace).Update(context.TODO(), sec2, metav1.UpdateOptions{})

	return err
}

func (p *KubernetesProvider) deleteSecret(sec *corev1.Secret, namespace string) error {
	sec.Namespace = namespace
	err := p.foreignClient.Client().CoreV1().Secrets(namespace).Delete(context.TODO(), sec.Name, metav1.DeleteOptions{})

	return err
}
