package kubernetes

import (
	"errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// manageEpEvent gets an event of type watch.MODIFIED, then cast it to the correct type,
// gets the natted namespace, and finally calls the updateEndpoints function
func (p *KubernetesProvider) manageEpEvent(event watch.Event) error {
	endpoints, ok := event.Object.(*corev1.Endpoints)
	if !ok {
		return errors.New("cannot cast object to endpoint")
	}

	nattedNS := p.NatNamespace(endpoints.Namespace, false)
	if nattedNS == "" {
		return errors.New("namespace not nattable")
	}

	return p.updateEndpoints(endpoints, nattedNS)
}

// updateEndpoints gets a local endpoints resource and a namespace, then fetches the remote
// endpoints, update the remote subset's addresses, and finally applies it to the remote
// cluster
func (p *KubernetesProvider) updateEndpoints(eps *corev1.Endpoints, namespace string) error {
	foreignEps, err := p.foreignClient.CoreV1().Endpoints(namespace).Get(eps.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	for i:=0; i<len(eps.Subsets); i++ {
		for _, addr := range eps.Subsets[i].Addresses {
			if foreignEps.Subsets == nil {
				foreignEps.Subsets = make([]corev1.EndpointSubset, 0)
			}

			if len(foreignEps.Subsets) <= i {
				foreignEps.Subsets = append(foreignEps.Subsets, corev1.EndpointSubset{})
				foreignEps.Subsets[i].Addresses = make([]corev1.EndpointAddress, 0)
			}

			if addr.NodeName == nil {
				continue
			}

			if *addr.NodeName != p.nodeName {
				addr.NodeName = nil
				addr.TargetRef = nil

				foreignEps.Subsets[i].Addresses = append(foreignEps.Subsets[i].Addresses, addr)
			}
		}
	}

	_, err = p.foreignClient.CoreV1().Endpoints(namespace).Update(foreignEps)
	return err
}
