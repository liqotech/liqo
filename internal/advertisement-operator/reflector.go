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

var (
	svcWatch watch.Interface
	epWatch  watch.Interface
)

// start the reflector module for one remote cluster
// parameters
// - namespace: the namespace on which the watchers will be started. It corresponds to the namespace on which the virtual-kubelet is active
// - adv: the advertisement of the remote cluster for which the reflector is being started
func StartReflector(namespace string, adv protocolv1.Advertisement) {

	log := ctrl.Log.WithName("advertisement-reflector")
	log.Info("starting reflector for cluster " + adv.Spec.ClusterId)

	// create a client to the local cluster
	localClient, err := pkg.NewK8sClient("", nil)
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
	go watchServices(log, localClient, remoteClient, namespace, adv)
	go watchEndpoints(log, localClient, remoteClient, namespace, adv)
	return
}

// stop the reflector module for the given advertisement
func StopReflector(advName string) {
	clusterId := strings.TrimPrefix(advName, "advertisement-")

	log := ctrl.Log.WithName("advertisement-reflector")
	log.Info("stopping reflector for cluster " + clusterId)

	if svcWatch == nil || epWatch == nil {
		log.Info("reflector was not active for cluster " + clusterId)
		return
	}
	svcWatch.Stop()
	epWatch.Stop()
}

// create a watcher for the services in the given namespace
// every event on a service is replicated on the remote cluster
// parameters:
// - log: the logger to use to record events
// - localClient: a client to the local kubernetes cluster
// - remoteClient: a client to the remote kubernetes cluster
// - namespace: the namespace on which the watcher will be started
// - adv: the advertisement of the remote cluster
func watchServices(log logr.Logger, localClient *kubernetes.Clientset, remoteClient *kubernetes.Clientset, namespace string, adv protocolv1.Advertisement) {
	var err error

	svcWatch, err = localClient.CoreV1().Services(namespace).Watch(metav1.ListOptions{})
	if err != nil {
		log.Error(err, "cannot watch services in namespace "+namespace)
		return
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

// create a watcher for the endpoints in the given namespace
// only CREATE and UPDATE events on a endpoints resource are replicated on the remote cluster
// parameters:
// - log: the logger to use to record events
// - localClient: a client to the local kubernetes cluster
// - remoteClient: a client to the remote kubernetes cluster
// - namespace: the namespace on which the watcher will be started
// - adv: the advertisement of the remote cluster
func watchEndpoints(log logr.Logger, localClient *kubernetes.Clientset, remoteClient *kubernetes.Clientset, namespace string, adv protocolv1.Advertisement) {
	var err error

	epWatch, err = localClient.CoreV1().Endpoints(namespace).Watch(metav1.ListOptions{})
	if err != nil {
		log.Error(err, "Cannot watch endpoints in namespace "+namespace)
		return
	}

	for event := range epWatch.ResultChan() {
		endpoints, ok := event.Object.(*corev1.Endpoints)
		if !ok {
			continue
		}

		switch event.Type {
		case watch.Added:
			// the endpoints resource has been created locally
			// if it only contains remote addresses, it doesn't need to be created on the remote cluster, because the cluster will manage everything
			// otherwise, if an address is only local, the remote cluster will never know it unless we directly create it
			for i, ep := range endpoints.Subsets {
				for j, addr := range ep.Addresses {
					// filter remote ep
					if !strings.HasPrefix(*addr.NodeName, "vk") {
						endpoints.Subsets[i].Addresses[j].NodeName = nil
						endpoints.Subsets[i].Addresses[j].TargetRef = nil

						_, err := remoteClient.CoreV1().Endpoints(namespace).Get(endpoints.Name, metav1.GetOptions{})
						if err != nil {
							// the endpoints resource doesn't exist on the remote cluster, try to create it
							log.Info("remote endpoints " + endpoints.Name + " doesn't exist: creating it")

							if err = CreateEndpoints(remoteClient, endpoints); err != nil {
								// the endpoints resource could not be created, maybe because it has been created concurrently by someone else: try to update it
								log.Info("unable to create endpoints " + endpoints.Name + " on cluster " + adv.Spec.ClusterId + " : " + err.Error() + " . Trying to update ")

								if err = UpdateEndpoints(remoteClient, endpoints); err != nil {
									// if we end here, something wrong has occured
									log.Info("ERROR: unable to update endpoints " + endpoints.Name + " on cluster " + adv.Spec.ClusterId + " : " + err.Error())
								} else {
									log.Info("correctly update endpoints " + endpoints.Name + " on cluster " + adv.Spec.ClusterId)
								}
							} else {
								log.Info("correctly created endpoints " + endpoints.Name + " on cluster " + adv.Spec.ClusterId)
							}
						} else {
							// the endpoints resource already exists on the remote cluster, just update it with the local addresses
							log.Info("remote endpoints " + endpoints.Name + " already exist: updating it")

							if err = UpdateEndpoints(remoteClient, endpoints); err != nil {
								log.Info("ERROR: unable to update endpoints " + endpoints.Name + " on cluster " + adv.Spec.ClusterId + " : " + err.Error())
							} else {
								log.Info("correctly update endpoints " + endpoints.Name + " on cluster " + adv.Spec.ClusterId)
							}
						}
					}
				}
			}
		case watch.Modified:
			for i, ep := range endpoints.Subsets {
				for j, addr := range ep.Addresses {
					// filter remote ep
					if !strings.HasPrefix(*addr.NodeName, "vk") {
						endpoints.Subsets[i].Addresses[j].NodeName = nil
						endpoints.Subsets[i].Addresses[j].TargetRef = nil
						if err = UpdateEndpoints(remoteClient, endpoints); err != nil {
							log.Info("ERROR: unable to update endpoints " + endpoints.Name + " on cluster " + adv.Spec.ClusterId + " : " + err.Error())
						} else {
							log.Info("correctly updated endpoints " + endpoints.Name + " on cluster " + adv.Spec.ClusterId)
						}
					}
				}
			}
		case watch.Deleted:
			// do nothing
			// the service deletion already deletes all the endpoints
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
