package kubernetes

import (
	"context"
	"errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog"
)

func (p *KubernetesProvider) manageSvcEvent(event watch.Event) error {
	var err error

	svc, ok := event.Object.(*corev1.Service)
	if !ok {
		return errors.New("cannot cast object to service")
	}
	klog.V(3).Infof("received %v on service %v", event.Type, svc.Name)

	nattedNS, err := p.NatNamespace(svc.Namespace, false)
	if err != nil {
		return err
	}

	switch event.Type {
	case watch.Added:
		_, err := p.foreignClient.Client().CoreV1().Services(nattedNS).Get(context.TODO(), svc.Name, metav1.GetOptions{})
		if err != nil {
			klog.Info("remote svc " + svc.Name + " doesn't exist: creating it")

			if err = p.createService(svc, nattedNS); err != nil {
				klog.Error(err, "unable to create service "+svc.Name+" on cluster "+p.foreignClusterId)
			} else {
				klog.Info("correctly created service " + svc.Name + " on cluster " + p.foreignClusterId)
			}
		}

	case watch.Modified:
		if err = p.updateService(svc, nattedNS); err != nil {
			klog.Error(err, "unable to update service "+svc.Name+" on cluster "+p.foreignClusterId)
		} else {
			klog.Info("correctly updated service " + svc.Name + " on cluster " + p.foreignClusterId)
		}

	case watch.Deleted:
		if err = p.deleteService(svc, nattedNS); err != nil {
			klog.Error(err, "unable to delete service "+svc.Name+" on cluster "+p.foreignClusterId)
		} else {
			klog.Info("correctly deleted service " + svc.Name + " on cluster " + p.foreignClusterId)
		}
	}

	return nil
}

func (p *KubernetesProvider) createService(svc *corev1.Service, namespace string) error {
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

	_, err := p.foreignClient.Client().CoreV1().Services(namespace).Create(context.TODO(), &svcRemote, metav1.CreateOptions{})

	return err
}

func (p *KubernetesProvider) updateService(svc *corev1.Service, namespace string) error {
	serviceOld, err := p.foreignClient.Client().CoreV1().Services(namespace).Get(context.TODO(), svc.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	svc2 := svc.DeepCopy()
	svc2.SetNamespace(namespace)
	svc2.SetResourceVersion(serviceOld.ResourceVersion)
	svc2.SetUID(serviceOld.UID)
	_, err = p.foreignClient.Client().CoreV1().Services(namespace).Update(context.TODO(), svc2, metav1.UpdateOptions{})

	return err
}

func (p *KubernetesProvider) deleteService(svc *corev1.Service, namespace string) error {
	svc.Namespace = namespace
	err := p.foreignClient.Client().CoreV1().Services(namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})

	return err
}
