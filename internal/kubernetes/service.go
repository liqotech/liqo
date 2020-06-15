package kubernetes

import (
	"errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
)

func (p *KubernetesProvider) manageSvcEvent(event watch.Event) error {
	var err error

	svc, ok := event.Object.(*corev1.Service)
	if !ok {
		return errors.New("cannot cast object to service")
	}
	klog.V(3).Info("received %v on service %v", event.Type, svc.Name)

	nattedNS, err := p.NatNamespace(svc.Namespace, false)
	if err != nil {
		return err
	}

	switch event.Type {
	case watch.Added:
		_, err := p.foreignClient.Client().CoreV1().Services(nattedNS).Get(svc.Name, metav1.GetOptions{})
		if err != nil {
			klog.Info("remote svc " + svc.Name + " doesn't exist: creating it")

			if err = CreateService(p.foreignClient.Client(), svc, nattedNS); err != nil {
				klog.Error(err, "unable to create service "+svc.Name+" on cluster "+p.foreignClusterId)
			} else {
				klog.Info("correctly created service " + svc.Name + " on cluster " + p.foreignClusterId)
			}
		}

	case watch.Modified:
		if err = UpdateService(p.foreignClient.Client(), svc, nattedNS); err != nil {
			klog.Error(err, "unable to update service "+svc.Name+" on cluster "+p.foreignClusterId)
		} else {
			klog.Info("correctly updated service " + svc.Name + " on cluster " + p.foreignClusterId)
		}

	case watch.Deleted:
		if err = DeleteService(p.foreignClient.Client(), svc, nattedNS); err != nil {
			klog.Error(err, "unable to delete service "+svc.Name+" on cluster "+p.foreignClusterId)
		} else {
			klog.Info("correctly deleted service " + svc.Name + " on cluster " + p.foreignClusterId)
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

	if svcRemote.Labels == nil {
		svcRemote.Labels = make(map[string]string)
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

	svc.SetNamespace(namespace)
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
