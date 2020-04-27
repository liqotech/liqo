package advertisement_operator

import (
	"github.com/go-logr/logr"
	protocolv1 "github.com/netgroup-polito/dronev2/api/advertisement-operator/v1"
	pkg "github.com/netgroup-polito/dronev2/pkg/advertisement-operator"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/discovery/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

func StartReflector(log logr.Logger, namespace string, adv protocolv1.Advertisement) {

	log.Info("starting reflector")

	// create a client to the local cluster
	localClient, err := pkg.NewK8sClient("/home/francesco/kind/kubeconfig-cluster1", nil)
	if err != nil {
		log.Error(err, "Unable to create client to local cluster")
		return
	}

	// create a client to the remote cluster
	cm, err := localClient.CoreV1().ConfigMaps("default").Get("foreign-kubeconfig-"+adv.Spec.ClusterId, metav1.GetOptions{})
	if err != nil {
		log.Error(err, "Unable to get ConfigMap foreign-kubeconfig-"+adv.Spec.ClusterId)
		return
	}
	remoteClient, err := pkg.NewK8sClient("/home/francesco/kind/kubeconfig-cluster2", cm)

	// create a local service watcher in the given namespace
	svcWatch, err := localClient.CoreV1().Services(namespace).Watch(metav1.ListOptions{})
	if err != nil {
		log.Error(err, "Cannot watch services in namespace "+namespace)
	}
	go func() {
		for event := range svcWatch.ResultChan() {
			svc, ok := event.Object.(*corev1.Service)
			if !ok {
				log.Error(err, "Unexpected type")
			}

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

			// send svc to foreign
			_, err := remoteClient.CoreV1().Services(namespace).Create(&svcRemote)
			if err != nil {
				log.Error(err, "Unable to create service "+svcRemote.Name+" on cluster "+adv.Spec.ClusterId)
			} else {
				log.Info("Correctly created service " + svcRemote.Name + " on cluster " + adv.Spec.ClusterId)
			}

			// get local and remote endpoints
			endpoints, err := localClient.CoreV1().Endpoints(namespace).Get(svc.Name, metav1.GetOptions{})
			if err != nil {
				log.Error(err, "Unable to get local endpoints "+svc.Name)
			}

			endpointsRemote, err := remoteClient.DiscoveryV1beta1().EndpointSlices(namespace).Get(svc.Name, metav1.GetOptions{})
			if err != nil {
				log.Error(err, "Unable to get endpoints "+svcRemote.Name+" on cluster "+adv.Spec.ClusterId)
			}

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
				Endpoints: []v1beta1.Endpoint{
					{
						Addresses:  nil,
						Conditions: v1beta1.EndpointConditions{},
						Hostname:   nil,
						TargetRef:  nil,
						Topology:   nil,
					},
				},
				Ports: []v1beta1.EndpointPort{
					{
						Name:        nil,
						Protocol:    nil,
						Port:        nil,
						AppProtocol: nil,
					},
				},
			}

			// add local endpoints to remote
			for _, ep := range endpoints.Subsets {
				for _, addr := range ep.Addresses {
					// filter remote ep
					if !strings.HasPrefix(*addr.NodeName, "vk") {
						e := v1beta1.Endpoint{
							Addresses:  []string{
								addr.IP,
							},
							Conditions: v1beta1.EndpointConditions{},
							Hostname:   &addr.Hostname,
							TargetRef:  addr.TargetRef,
							Topology:   nil,
						}
						epSlice.Endpoints = append(epSlice.Endpoints, e)

						port := v1beta1.EndpointPort{
							Name:        &ep.Ports[0].Name,
							Protocol:    &ep.Ports[0].Protocol,
							Port:        &ep.Ports[0].Port,
							AppProtocol: nil,
						}
						epSlice.Ports = append(epSlice.Ports, port)
					}
				}
			}

			_, err = remoteClient.DiscoveryV1beta1().EndpointSlices(namespace).Create(&epSlice)
			if err != nil {
				log.Error(err, "Unable to update endpoints "+endpointsRemote.Name+" on cluster "+adv.Spec.ClusterId)
			} else {
				log.Info("Correctly updated endpoints " + endpointsRemote.Name + " on cluster " + adv.Spec.ClusterId)
			}
		}
	}()
}
