package advertisement_operator

import (
	"github.com/go-logr/logr"
	protocolv1 "github.com/netgroup-polito/dronev2/api/advertisement-operator/v1"
	pkg "github.com/netgroup-polito/dronev2/pkg/advertisement-operator"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/discovery/v1beta1"
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

	// create a local service watcher in the given namespace
	go watchServices(log, localClient,remoteClient,namespace,adv)
    //go watchEP(localClient,remoteClient,namespace,adv)
    return
}

func watchServices(log logr.Logger, localClient *kubernetes.Clientset,remoteClient *kubernetes.Clientset, namespace string, adv protocolv1.Advertisement){
	svcWatch, err := localClient.CoreV1().Services(namespace).Watch(metav1.ListOptions{})
	if err != nil {
		log.Error(err, "cannot watch services in namespace "+namespace)
	}

	//mutex.Lock()
	for event := range svcWatch.ResultChan() {
		svc, ok := event.Object.(*corev1.Service)
		if !ok {
			continue
		}
		switch event.Type{
		case watch.Added:
			_, err := remoteClient.CoreV1().Services(namespace).Get(svc.Name, metav1.GetOptions{})
			if err != nil {
				log.Info("remote svc "+svc.Name + " doesn't exist: creating it")

				if err = CreateService(remoteClient, svc); err != nil{
					log.Error(err, "unable to create service "+svc.Name+" on cluster "+adv.Spec.ClusterId)
				} else {
					log.Info("correctly created service " + svc.Name + " on cluster " + adv.Spec.ClusterId)
				}
				addEndpoints(localClient, remoteClient, svc, adv.Spec.ClusterId)
			}
		case watch.Modified:
			if err = UpdateService(remoteClient, svc); err != nil{
				log.Error(err, "unable to update service "+svc.Name+" on cluster "+adv.Spec.ClusterId)
			} else {
				log.Info("correctly updated service " + svc.Name + " on cluster " + adv.Spec.ClusterId)
			}
		case watch.Deleted:
			if err = DeleteService(remoteClient, svc); err != nil{
				log.Error(err, "unable to delete service "+svc.Name+" on cluster "+adv.Spec.ClusterId)
			} else {
				log.Info("correctly deleted service " + svc.Name + " on cluster " + adv.Spec.ClusterId)
			}
		}

	}
	//mutex.Unlock()
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

func UpdateService(c *kubernetes.Clientset, svc *corev1.Service) error{
	servicePre, err := c.CoreV1().Services(svc.Namespace).Get(svc.Name, metav1.GetOptions{})
	if err != nil {
		log.Info("Remote svc "+svc.Name + " doesn't exist")
		return err
	}

	servicePost := svc.DeepCopy()
	svc.SetResourceVersion(servicePre.ResourceVersion)
	_, err = c.CoreV1().Services(svc.Namespace).Update(servicePost)
	return err
}

func DeleteService(c *kubernetes.Clientset, svc *corev1.Service) error{
	err := c.CoreV1().Services(svc.Namespace).Delete(svc.Name, &metav1.DeleteOptions{})
	return err
}

func addEndpoints(localClient *kubernetes.Clientset, remoteClient *kubernetes.Clientset, svc *corev1.Service, clusterId string){
	// get local and remote endpoints
	endpoints, err := localClient.CoreV1().Endpoints(svc.Namespace).Get(svc.Name, metav1.GetOptions{})
	if err != nil {
		log.Error(err, "Unable to get local endpoints "+svc.Name)
	}
	endpointsRemote, err := remoteClient.CoreV1().Endpoints(svc.Namespace).Get(svc.Name, metav1.GetOptions{})
	if err != nil {
		log.Error(err, "Unable to get endpoints "+svc.Name+" on cluster "+clusterId)
	}
	if endpointsRemote.Subsets == nil {
		endpointsRemote.Subsets = make([]corev1.EndpointSubset, len(endpoints.Subsets))
	}

	// add local endpoints to remote
	for i, ep := range endpoints.Subsets {
		for j, addr := range ep.Addresses {
			// filter remote ep
			if !strings.HasPrefix(*addr.NodeName, "vk") {
				endpointsRemote.Subsets[i] = ep
				endpointsRemote.Subsets[i].Addresses[j].NodeName = nil
			}
		}
	}

	_, err = remoteClient.CoreV1().Endpoints(svc.Namespace).Update(endpointsRemote)
	if err != nil {
		log.Error(err, "Unable to update endpoints "+endpointsRemote.Name+" on cluster "+clusterId)
	} else {
		log.Info("Correctly updated endpoints " + endpointsRemote.Name + " on cluster " + clusterId)
	}
}

func watchEP(localClient *kubernetes.Clientset,remoteClient *kubernetes.Clientset, namespace string, adv protocolv1.Advertisement){
	epWatch, err := remoteClient.CoreV1().Endpoints(namespace).Watch(metav1.ListOptions{})
	if err != nil {
		log.Error(err, "Cannot watch services in namespace "+namespace)
	}
	for event := range epWatch.ResultChan() {
		localEndpoints, ok := event.Object.(*corev1.Endpoints)
		if !ok {
			log.Error(err, "Unexpected type")
			continue
		}

		if event.Type == "Deleted"{
			//cleanRemote
		}

		svc, err := localClient.CoreV1().Services(namespace).Get(localEndpoints.Name, metav1.GetOptions{})
		if err != nil {
			log.Error(err, "Unable to get svc "+localEndpoints.Name)
			continue
		}

		endpointsRemote, err := remoteClient.CoreV1().Endpoints(namespace).Get(svc.Name, metav1.GetOptions{})
		if err != nil {
			log.Info("Unable to get endpoints "+localEndpoints.Name+" on cluster "+adv.Spec.ClusterId)
		}
		endpointsToPost := generateEP(localEndpoints, endpointsRemote)
		if endpointsToPost != nil && endpointsRemote == nil {
			_, err := remoteClient.CoreV1().Endpoints(namespace).Create(endpointsToPost)
			if err != nil {
				log.Error(err, "Unable to create endpoints "+endpointsRemote.Name+" on cluster "+adv.Spec.ClusterId)
			} else {
				log.Info("Correctly created endpoints " + endpointsRemote.Name + " on cluster " + adv.Spec.ClusterId)
			}
		} else if endpointsToPost != nil && endpointsRemote != nil {
			_, err := remoteClient.CoreV1().Endpoints(namespace).Update(endpointsToPost)
			if err != nil {
				log.Error(err, "Unable to update endpoints "+endpointsToPost.Name+" on cluster "+adv.Spec.ClusterId)
			} else {
				log.Info("Correctly updated endpoints " + endpointsToPost.Name + " on cluster " + adv.Spec.ClusterId)
			}
		}

	}
}

func generateEP(localEndpoints *corev1.Endpoints, endpointsRemote *corev1.Endpoints) (*corev1.Endpoints){
	new := false
	if endpointsRemote == nil {
		new = true
		endpointsRemote = &corev1.Endpoints{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      localEndpoints.Name,
				Namespace: localEndpoints.Namespace,
			},
			Subsets: []corev1.EndpointSubset{},
		}
	}

	flag := false
	for i, eps := range localEndpoints.Subsets {
		for _, addr := range eps.Addresses {
			if addr.NodeName == nil {
				continue
			}
			if !strings.HasPrefix(*addr.NodeName, "vk") {
				a := corev1.EndpointAddress{
					IP:        addr.IP,
					Hostname:  addr.Hostname,
					NodeName:  nil,
					TargetRef: nil,
				}
				endpointsRemote.Subsets[i].Addresses = append(endpointsRemote.Subsets[i].Addresses, a)
				flag = true
			}
		}
		if new == true {
			for _, port := range eps.Ports {
				endpointsRemote.Subsets[i].Ports = append(endpointsRemote.Subsets[i].Ports, port)
			}
		}

	}
	if flag != true {
		return nil
	}
	return endpointsRemote
}

func generateSlice(endpoints *corev1.Endpoints, svc *corev1.Service) (*v1beta1.EndpointSlice){
	epSlice:= v1beta1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svc.Name,
			Namespace: svc.Namespace,
			Labels: map[string]string{
				"endpointslice.kubernetes.io/managed-by" : "vk",
				"kubernetes.io/service-name": svc.Name,
			},
		},
		AddressType: v1beta1.AddressTypeIPv4,
		Endpoints: []v1beta1.Endpoint{},
		Ports: []v1beta1.EndpointPort{},
	}

	for _, ep := range endpoints.Subsets {
		flag := false
		for _, addr := range ep.Addresses {
			// filter remote ep
			if !strings.HasPrefix(*addr.NodeName, "vk") {
				t := true
				e := v1beta1.Endpoint{
					Addresses: []string{
						addr.IP,
					},
					Conditions: v1beta1.EndpointConditions{
						Ready: &t,
					},
					TargetRef:  nil,
					Topology:   nil,
				}
				epSlice.Endpoints = append(epSlice.Endpoints, e)
				flag = true
			}
		}
		if flag != true {
			return nil
		}
		for _, port := range ep.Ports {
			p := v1beta1.EndpointPort{
				Name:     &port.Name,
				Protocol: &port.Protocol,
				Port:     &port.Port,
			}
			epSlice.Ports = append(epSlice.Ports, p)
		}
	}

	return &epSlice
}