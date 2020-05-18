package kubernetes

import (
	"errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

func (p *KubernetesProvider) manageSvcEvent(event watch.Event) error {
	var err error

	svc, ok := event.Object.(*corev1.Service)
	if !ok {
		return errors.New("cannot cast object to service")
	}

	nattedNS := p.NatNamespace(svc.Namespace, false)
	if nattedNS == "" {
		return errors.New("namespace not nattable")
	}

	switch event.Type {
	case watch.Added:
		_, err := p.foreignClient.CoreV1().Services(nattedNS).Get(svc.Name, metav1.GetOptions{})
		if err != nil {
			p.log.Info("remote svc " + svc.Name + " doesn't exist: creating it")

			if err = CreateService(p.foreignClient, svc, nattedNS); err != nil {
				p.log.Error(err, "unable to create service " + svc.Name + " on cluster " + p.clusterId)
			} else {
				p.log.Info("correctly created service " + svc.Name + " on cluster " + p.clusterId)
			}
		}

	case watch.Modified:
		if err = UpdateService(p.foreignClient, svc, nattedNS); err != nil {
			p.log.Error(err, "unable to update service " + svc.Name + " on cluster " + p.clusterId)
		} else {
			p.log.Info("correctly updated service " + svc.Name + " on cluster " + p.clusterId)
		}

	case watch.Deleted:
		if err = DeleteService(p.foreignClient, svc, nattedNS); err != nil {
			p.log.Error(err, "unable to delete service " + svc.Name + " on cluster " + p.clusterId)
		} else {
			p.log.Info("correctly deleted service " + svc.Name + " on cluster " + p.clusterId)
		}
	}

	return nil
}

func CreateService(c *kubernetes.Clientset, svc *corev1.Service, namespace string) error {
	svcRemote := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        svc.Name,
			Namespace:   namespace,
			Labels:      svc.Labels,
			Annotations: nil,
		},
		Spec: corev1.ServiceSpec{
			Ports:    svc.Spec.Ports,
			Selector: svc.Spec.Selector,
			Type:     svc.Spec.Type,
		},
	}
	svcRemote.Labels["liqo/reflection"] = "reflected"

	_, err := c.CoreV1().Services(namespace).Create(&svcRemote)

	return err
}

func UpdateService(c *kubernetes.Clientset, svc *corev1.Service, namespace string) error {
	serviceOld, err := c.CoreV1().Services(namespace).Get(svc.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	svc.SetResourceVersion(serviceOld.ResourceVersion)
	svc.SetUID(serviceOld.UID)
	_, err = c.CoreV1().Services(namespace).Update(svc)

	return err
}

func DeleteService(c *kubernetes.Clientset, svc *corev1.Service, namespace string) error {
	svc.Namespace = namespace
	err := c.CoreV1().Services(namespace).Delete(svc.Name, &metav1.DeleteOptions{})

	return err
}
