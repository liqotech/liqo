package advertisement_operator

import (
	"github.com/go-logr/logr"
	protocolv1 "github.com/netgroup-polito/dronev2/api/advertisement-operator/v1"
	pkg "github.com/netgroup-polito/dronev2/pkg/advertisement-operator"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"strings"
)

func StartReflector(namespace string, adv protocolv1.Advertisement) {

	log := ctrl.Log.WithName("advertisement-reflector")
	log.Info("starting reflector for cluster " + adv.Spec.ClusterId)

	// create a client to the local cluster
	localClient, err := pkg.NewK8sClient("/home/francesco/kind/kubeconfig-cluster1", nil)
	if err != nil {
		log.Error(err, "unable to create client to local cluster")
		return
	}

	// create a client to the remote cluster
	cm, err := localClient.CoreV1().ConfigMaps("default").Get("foreign-kubeconfig-"+adv.Spec.ClusterId, metav1.GetOptions{})
	if err != nil {
		log.Error(err, "unable to get ConfigMap foreign-kubeconfig-"+adv.Spec.ClusterId)
		return
	}
	remoteClient, err := pkg.NewK8sClient("", cm)

	// create local service and endpoints watchers in the given namespace
	//go watchServices(log, localClient, remoteClient, namespace, adv)
	go watchEndpoints(log, localClient, remoteClient, namespace, adv)
	return
}

func watchServices(log logr.Logger, localClient *kubernetes.Clientset, remoteClient *kubernetes.Clientset, namespace string, adv protocolv1.Advertisement) {
	svcWatch, err := localClient.CoreV1().Services(namespace).Watch(metav1.ListOptions{})
	if err != nil {
		log.Error(err, "cannot watch services in namespace "+namespace)
	}

	for event := range svcWatch.ResultChan() {
		svc, ok := event.Object.(*corev1.Service)
		if !ok {
			continue
		}
		switch event.Type {
		case watch.Added:
			_, err := remoteClient.CoreV1().Services(namespace).Get(svc.Name, metav1.GetOptions{})
			if err != nil {
				log.Info("remote svc " + svc.Name + " doesn't exist: creating it")

				if err = CreateService(remoteClient, svc); err != nil {
					log.Error(err, "unable to create service "+svc.Name+" on cluster "+adv.Spec.ClusterId)
				} else {
					log.Info("correctly created service " + svc.Name + " on cluster " + adv.Spec.ClusterId)
				}
				//addEndpoints(localClient, remoteClient, svc, adv.Spec.ClusterId)
			}
		case watch.Modified:
			if err = UpdateService(remoteClient, svc); err != nil {
				log.Error(err, "unable to update service "+svc.Name+" on cluster "+adv.Spec.ClusterId)
			} else {
				log.Info("correctly updated service " + svc.Name + " on cluster " + adv.Spec.ClusterId)
			}
		case watch.Deleted:
			if err = DeleteService(remoteClient, svc); err != nil {
				log.Error(err, "unable to delete service "+svc.Name+" on cluster "+adv.Spec.ClusterId)
			} else {
				log.Info("correctly deleted service " + svc.Name + " on cluster " + adv.Spec.ClusterId)
			}
		}

	}
}

func CreateService(c *kubernetes.Clientset, svc *corev1.Service) error {
	// translate svc
	svcRemote := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        svc.Name,
			Namespace:   svc.Namespace,
			Labels:      svc.Labels,
			Annotations: nil,
		},
		Spec: corev1.ServiceSpec{
			Ports:    svc.Spec.Ports,
			Selector: svc.Spec.Selector,
			Type:     svc.Spec.Type,
		},
	}

	_, err := c.CoreV1().Services(svc.Namespace).Create(&svcRemote)
	return err
}

func UpdateService(c *kubernetes.Clientset, svc *corev1.Service) error {
	serviceOld, err := c.CoreV1().Services(svc.Namespace).Get(svc.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	svc.SetResourceVersion(serviceOld.ResourceVersion)
	svc.SetUID(serviceOld.UID)
	_, err = c.CoreV1().Services(svc.Namespace).Update(svc)
	return err
}

func DeleteService(c *kubernetes.Clientset, svc *corev1.Service) error {
	err := c.CoreV1().Services(svc.Namespace).Delete(svc.Name, &metav1.DeleteOptions{})
	return err
}

func addEndpoints(localClient *kubernetes.Clientset, remoteClient *kubernetes.Clientset, svc *corev1.Service, clusterId string) {
	// get local and remote endpoints
	endpoints, err := localClient.CoreV1().Endpoints(svc.Namespace).Get(svc.Name, metav1.GetOptions{})
	if err != nil {
		log.Error(err, "unable to get local endpoints "+svc.Name)
	}
	endpointsRemote, err := remoteClient.CoreV1().Endpoints(svc.Namespace).Get(svc.Name, metav1.GetOptions{})
	if err != nil {
		log.Error(err, "unable to get endpoints "+svc.Name+" on cluster "+clusterId)
	}
	if endpointsRemote.Subsets == nil {
		endpointsRemote.Subsets = make([]corev1.EndpointSubset, len(endpoints.Subsets))
	}

	// add local endpoints to remote
	flag := false
	for i, ep := range endpoints.Subsets {
		for j, addr := range ep.Addresses {
			// filter remote ep
			if !strings.HasPrefix(*addr.NodeName, "vk") {
				endpointsRemote.Subsets[i] = ep
				endpointsRemote.Subsets[i].Addresses[j].NodeName = nil
				flag = true
			}
		}
	}

	if flag == true {
		_, err = remoteClient.CoreV1().Endpoints(svc.Namespace).Update(endpointsRemote)
		if err != nil {
			log.Error(err, "Unable to update endpoints "+endpointsRemote.Name+" on cluster "+clusterId)
		} else {
			log.Info("Correctly updated endpoints " + endpointsRemote.Name + " on cluster " + clusterId)
		}
	}
}

func watchEndpoints(log logr.Logger, localClient *kubernetes.Clientset, remoteClient *kubernetes.Clientset, namespace string, adv protocolv1.Advertisement) {
	epWatch, err := localClient.CoreV1().Endpoints(namespace).Watch(metav1.ListOptions{})
	if err != nil {
		log.Error(err, "Cannot watch endpoints in namespace "+namespace)
	}

	for event := range epWatch.ResultChan() {
		ep, ok := event.Object.(*corev1.Endpoints)
		if !ok {
			continue
		}
		switch event.Type {
		case watch.Added:
			_, err := remoteClient.CoreV1().Endpoints(namespace).Get(ep.Name, metav1.GetOptions{})
			if err != nil {
				log.Info("remote endpoints " + ep.Name + " doesn't exist: creating it")

				if err = CreateEndpoints(remoteClient, ep); err != nil {
					log.Error(err, "unable to create endpoints "+ep.Name+" on cluster "+adv.Spec.ClusterId)
				} else {
					log.Info("correctly created endpoints " + ep.Name + " on cluster " + adv.Spec.ClusterId)
				}
			} else {
				log.Info("remote endpoints " + ep.Name + " already exist: updating it")

				if err = UpdateEndpoints(remoteClient, ep); err != nil {
					log.Error(err, "unable to update endpoints "+ep.Name+" on cluster "+adv.Spec.ClusterId)
				} else {
					log.Info("correctly update endpoints " + ep.Name + " on cluster " + adv.Spec.ClusterId)
				}
			}
		case watch.Modified:
			if err = UpdateEndpoints(remoteClient, ep); err != nil {
				log.Error(err, "unable to update endpoints "+ep.Name+" on cluster "+adv.Spec.ClusterId)
			} else {
				log.Info("correctly updated endpoints " + ep.Name + " on cluster " + adv.Spec.ClusterId)
			}
		case watch.Deleted:
			if err = DeleteEndpoints(remoteClient, ep); err != nil {
				log.Error(err, "unable to delete endpoints "+ep.Name+" on cluster "+adv.Spec.ClusterId)
			} else {
				log.Info("correctly deleted endpoints " + ep.Name + " on cluster " + adv.Spec.ClusterId)
			}
		}
	}
}

func CreateEndpoints(c *kubernetes.Clientset, ep *corev1.Endpoints) error {
	epRemote := corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ep.Name,
			Namespace:   ep.Namespace,
			Labels:      ep.Labels,
			Annotations: ep.Annotations,
		},
		Subsets: ep.Subsets,
	}

	_, err := c.CoreV1().Endpoints(ep.Namespace).Create(&epRemote)
	return err
}

func UpdateEndpoints(c *kubernetes.Clientset, ep *corev1.Endpoints) error {
	endpointsOld, err := c.CoreV1().Endpoints(ep.Namespace).Get(ep.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	ep.SetResourceVersion(endpointsOld.ResourceVersion)
	ep.SetUID(endpointsOld.UID)
	_, err = c.CoreV1().Endpoints(ep.Namespace).Update(ep)
	return err
}

func DeleteEndpoints(c *kubernetes.Clientset, ep *corev1.Endpoints) error {
	err := c.CoreV1().Endpoints(ep.Namespace).Delete(ep.Name, &metav1.DeleteOptions{})
	return err
}
