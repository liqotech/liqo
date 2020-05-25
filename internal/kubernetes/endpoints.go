package kubernetes

import (
	"errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"strconv"
)

func (p *KubernetesProvider) manageRemoteEpEvent(event watch.Event) error {
	foreignEps, ok := event.Object.(*corev1.Endpoints)
	if !ok {
		return errors.New("cannot cast endpoints")
	}

	denattedNS, err := p.DeNatNamespace(foreignEps.Namespace)
	if err != nil {
		return err
	}
	endpoints, err := p.homeClient.Client().CoreV1().Endpoints(denattedNS).Get(foreignEps.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	foreignEps.Subsets = p.updateEndpoints(endpoints.Subsets, foreignEps.Subsets)

	_, err = p.foreignClient.Client().CoreV1().Endpoints(foreignEps.Namespace).Update(foreignEps)

	return err
}

// manageEpEvent gets an event of type watch.MODIFIED, then cast it to the correct type,
// gets the natted namespace, and finally calls the updateEndpoints function
func (p *KubernetesProvider) manageEpEvent(event timestampedEvent) error {
	endpoints, ok := event.event.Object.(*corev1.Endpoints)
	if !ok {
		return errors.New("cannot cast object to endpoint")
	}

	nattedNS, err := p.NatNamespace(endpoints.Namespace)
	if err != nil {
		return err
	}

	foreignEps, err := p.foreignClient.Client().CoreV1().Endpoints(nattedNS).Get(endpoints.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	var t int64
	if v, ok := foreignEps.GetLabels()[timestampedLabel]; ok {
		t, err = strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil
		}

		// old event
		if event.ts < t{
			return nil
		}
	}

	foreignEps.Subsets = p.updateEndpoints(endpoints.Subsets, foreignEps.Subsets)

	if foreignEps.Labels == nil {
		foreignEps.Labels = make(map[string]string)
	}
	foreignEps.Labels[timestampedLabel] = strconv.FormatInt(event.ts, 10)
	foreignEps.Namespace = nattedNS
	_, err = p.foreignClient.Client().CoreV1().Endpoints(nattedNS).Update(foreignEps)

	return err
}

// updateEndpoints gets a local endpoints resource and a namespace, then fetches the remote
// endpoints, update the remote subset's addresses, and finally applies it to the remote
// cluster
func (p *KubernetesProvider) updateEndpoints(eps, foreignEps []corev1.EndpointSubset) []corev1.EndpointSubset {
	subsets := make([]corev1.EndpointSubset, 0)

	for i:=0; i<len(eps); i++ {
		subsets = append(subsets, corev1.EndpointSubset{})
		subsets[i].Addresses = make([]corev1.EndpointAddress, 0)
		subsets[i].Ports = eps[i].Ports

		for _, addr := range eps[i].Addresses {

			if addr.NodeName == nil {
				continue
			}

			if *addr.NodeName != p.nodeName {
				addr.NodeName = &p.homeClusterID
				addr.TargetRef = nil

				subsets[i].Addresses = append(subsets[i].Addresses, addr)
			}
		}

		if foreignEps == nil {
			continue
		}

		for _, e := range foreignEps[i].Addresses {
			if *e.NodeName != p.homeClusterID {
				subsets[i].Addresses = append(subsets[i].Addresses, e)
			}
		}
	}
	return subsets
}
