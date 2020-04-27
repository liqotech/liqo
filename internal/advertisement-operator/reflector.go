package advertisement_operator

import (
	"github.com/go-logr/logr"
	protocolv1 "github.com/netgroup-polito/dronev2/api/advertisement-operator/v1"
	mutation "github.com/netgroup-polito/dronev2/internal/kubernetes"
	pkg "github.com/netgroup-polito/dronev2/pkg/advertisement-operator"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

func StartReflector(log logr.Logger, namespace string, adv protocolv1.Advertisement){

	log.Info("starting reflector")

	// create a client to the local cluster
	localClient, err := pkg.NewK8sClient("", nil)
	if err != nil {
		log.Error(err, "Unable to create client to local cluster")
		return
	}

	// create a client to the remote cluster
	cm, err := localClient.CoreV1().ConfigMaps("default").Get("foreign-kubeconfig-" + adv.Spec.ClusterId, metav1.GetOptions{})
	if err != nil {
		log.Error(err, "Unable to get ConfigMap foreign-kubeconfig-" + adv.Spec.ClusterId)
		return
	}
	remoteClient, err := pkg.NewK8sClient("", cm)

	// create a local service watcher in the given namespace
	svcWatch, err := localClient.CoreV1().Services(namespace).Watch(metav1.ListOptions{})
	if err != nil {
		log.Error(err, "Cannot watch services in namespace " + namespace)
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
					Name:                       svc.Name,
					Namespace:                  svc.Namespace,
					Labels:                     svc.Labels,
					Annotations:                nil,
				},
				Spec:       corev1.ServiceSpec{
					Ports:                    svc.Spec.Ports,
					Selector:                 svc.Spec.Selector,
					Type:                     svc.Spec.Type,
				},
			}

			// send svc to foreign
			_, err := remoteClient.CoreV1().Services(namespace).Create(&svcRemote)
			if err != nil {
				log.Error(err, "Unable to create service " + svcRemote.Name + " on cluster " + adv.Spec.ClusterId)
			} else {
				log.Info("Correctly created service " + svcRemote.Name + " on cluster " + adv.Spec.ClusterId)
			}

			// get local and remote endpoints
			endpoints, err := localClient.CoreV1().Endpoints(namespace).Get(svc.Name, metav1.GetOptions{})
			if err != nil {
				log.Error(err, "Unable to get local endpoints " + svc.Name)
			}
			endpointsRemote, err := remoteClient.CoreV1().Endpoints(namespace).Get(svc.Name, metav1.GetOptions{})
			if err != nil {
				log.Error(err, "Unable to get endpoints " + svcRemote.Name + " on cluster " + adv.Spec.ClusterId)
			}
			if endpointsRemote.Subsets == nil {
				endpointsRemote.Subsets = make([]corev1.EndpointSubset, len(endpoints.Subsets))
			}

			// add local endpoints to remote
			for i, ep := range endpoints.Subsets{
				for j, addr := range ep.Addresses {
					// filter remote ep
					if !strings.HasPrefix(*addr.NodeName, "vk"){
						endpointsRemote.Subsets[i] = ep
						endpointsRemote.Subsets[i].Addresses[j].IP = mutation.ChangePodIp(adv.Spec.Network.PodCIDR, addr.IP)
						endpointsRemote.Subsets[i].Addresses[j].NodeName = nil
					}
				}
			}

			_, err = remoteClient.CoreV1().Endpoints(namespace).Update(endpointsRemote)
			if err != nil {
				log.Error(err, "Unable to update endpoints " + endpointsRemote.Name + " on cluster " + adv.Spec.ClusterId)
			} else {
				log.Info("Correctly updated endpoints " + endpointsRemote.Name + " on cluster " + adv.Spec.ClusterId)
			}
		}
	}()
}
