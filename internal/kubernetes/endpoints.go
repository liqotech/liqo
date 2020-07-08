package kubernetes

import (
	"errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog"
	"strconv"
)

func (p *KubernetesProvider) manageRemoteEpEvent(event watch.Event) error {
	klog.V(3).Info("FOREIGN EP EVENT - starting remote ep reconciliation")
	foreignEps, ok := event.Object.(*corev1.Endpoints)
	if !ok {
		return errors.New("cannot cast endpoints")
	}
	klog.V(3).Infof("FOREIGN EP EVENT - event %v on endpoint %v with version %v", event.Type, foreignEps.Name, foreignEps.ResourceVersion)

	denattedNS, err := p.DeNatNamespace(foreignEps.Namespace)
	if err != nil {
		return err
	}
	endpoints, err := p.homeClient.Client().CoreV1().Endpoints(denattedNS).Get(foreignEps.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if !hasToBeUpdated(endpoints.Subsets, foreignEps.Subsets) {
		klog.V(3).Infof("FOREIGN EP EVENT - endpoint %v has not to be updated", foreignEps.Name)
		return nil
	}

	foreignEps.Subsets = p.updateEndpoints(endpoints.Subsets, foreignEps.Subsets)

	_, err = p.foreignClient.Client().CoreV1().Endpoints(foreignEps.Namespace).Update(foreignEps)
	if err != nil {
		return err
	}

	klog.V(3).Infof("FOREIGN EP EVENT - event %v on endpoint %v with version %v correctly reconciliated", event.Type, foreignEps.Name, foreignEps.ResourceVersion)

	return nil
}

// manageEpEvent gets an event of type watch.MODIFIED, then cast it to the correct type,
// gets the natted namespace, and finally calls the updateEndpoints function
func (p *KubernetesProvider) manageEpEvent(event timestampedEvent) error {
	klog.V(3).Info("HOME EP EVENT - starting home ep reconciliation")
	endpoints, ok := event.event.Object.(*corev1.Endpoints)
	if !ok {
		return errors.New("cannot cast object to endpoint")
	}
	klog.V(3).Infof("HOME EP EVENT - event %v on endpoint %v with version %v", event.event.Type, endpoints.Name, endpoints.ResourceVersion)

	nattedNS, err := p.NatNamespace(endpoints.Namespace, false)
	if err != nil {
		return err
	}

	foreignEps, err := p.foreignClient.Client().CoreV1().Endpoints(nattedNS).Get(endpoints.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if !hasToBeUpdated(endpoints.Subsets, foreignEps.Subsets) {
		klog.V(3).Infof("HOME EP EVENT - endpoint %v has not to be updated", foreignEps.Name)
		return nil
	}

	var t int64
	if v, ok := foreignEps.GetLabels()[timestampedLabel]; ok {
		t, err = strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil
		}

		// old event
		if event.ts < t {
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
	if err != nil {
		return err
	}

	klog.V(3).Infof("HOME EP EVENT - event %v on endpoint %v with version %v correctly reconciliated", event.event.Type, endpoints.Name, endpoints.ResourceVersion)
	return nil
}

// updateEndpoints gets a local endpoints resource and a namespace, then fetches the remote
// endpoints, update the remote subset's addresses, and finally applies it to the remote
// cluster
func (p *KubernetesProvider) updateEndpoints(eps, foreignEps []corev1.EndpointSubset) []corev1.EndpointSubset {
	subsets := make([]corev1.EndpointSubset, 0)

	for i := 0; i < len(eps); i++ {
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

func hasToBeUpdated(home, foreign []corev1.EndpointSubset) bool {
	if len(home) != len(foreign) {
		klog.V(4).Info("the ep has to be updated because home and foreign subsets lengths are different")
		return true
	}
	for i := 0; i < len(home); i++ {
		if len(home[i].Addresses) != len(foreign[i].Addresses) {
			klog.V(4).Info("the ep has to be updated because home and foreign addresses lengths are different")
			return true
		}
		for j := 0; j < len(home[i].Addresses); j++ {
			if home[i].Addresses[j].IP != foreign[i].Addresses[j].IP {
				klog.V(4).Info("the ep has to be updated because home and foreign IPs are different")
				return true
			}
		}
	}

	return false
}
