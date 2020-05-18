package kubernetes

import (
	"errors"
	corev1 "k8s.io/api/core/v1"
	k8sApiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

func (p *KubernetesProvider) manageEpEvent(event watch.Event) error {
	endpoints, ok := event.Object.(*corev1.Endpoints)
	if !ok {
		return errors.New("cannot cast object to endpoint")
	}

	nattedNS := p.NatNamespace(endpoints.Namespace, false)
	if nattedNS == "" {
		return errors.New("namespace not nattable")
	}

	switch event.Type {
	case watch.Added:
		// the endpoints resource has been created locally
		// if it only contains remote addresses, it doesn't need to be created on the remote cluster, because the cluster will manage everything
		// otherwise, if an address is only local, the remote cluster will never know it unless we directly create it
		for i, ep := range endpoints.Subsets {
			for j, addr := range ep.Addresses {
				// filter remote ep
				if p.nodeName != *addr.NodeName {
					endpoints.Subsets[i].Addresses[j].NodeName = nil
					endpoints.Subsets[i].Addresses[j].TargetRef = nil
					_, err := p.foreignClient.CoreV1().Endpoints(nattedNS).Get(endpoints.Name, metav1.GetOptions{})

					if err == nil {
						// the endpoints resource already exists on the remote cluster, just update it with the local addresses
						p.log.Info("remote endpoints " + endpoints.Name + " already exist: updating it")

						if err = p.updateEndpoints(endpoints, nattedNS); err != nil {
							p.log.Info("ERROR: unable to update endpoints " + endpoints.Name + " on cluster " + p.clusterId + " : " + err.Error())
						} else {
							p.log.Info("correctly update endpoints " + endpoints.Name + " on cluster " + p.clusterId)
						}
					}

					if k8sApiErrors.IsNotFound(err) {
						// the endpoints resource doesn't exist on the remote cluster, try to create it
						p.log.Info("remote endpoints " + endpoints.Name + " doesn't exist: creating it")

						if err = CreateEndpoints(p.foreignClient, endpoints, nattedNS); err != nil {
							// the endpoints resource could not be created, maybe because it has been created concurrently by someone else: try to update it
							p.log.Info("unable to create endpoints " + endpoints.Name + " on cluster " + p.clusterId + " : " + err.Error() + " . Trying to update ")

							if err = p.updateEndpoints(endpoints, nattedNS); err != nil {
								// if we end here, something wrong has occured
								p.log.Info("ERROR: unable to update endpoints " + endpoints.Name + " on cluster " + p.clusterId + " : " + err.Error())
							} else {
								p.log.Info("correctly update endpoints " + endpoints.Name + " on cluster " + p.clusterId)
							}
						} else {
							p.log.Info("correctly created endpoints " + endpoints.Name + " on cluster " + p.clusterId)
						}
					} else {
						return err
					}
				}
			}
		}

	case watch.Modified:
		return p.updateEndpoints(endpoints, nattedNS)
	}

	return nil
}

func CreateEndpoints(c *kubernetes.Clientset, ep *corev1.Endpoints, namespace string) error {

	epRemote := corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ep.Name,
			Namespace:   namespace,
			Labels:      ep.Labels,
			Annotations: ep.Annotations,
		},
		Subsets: ep.Subsets,
	}

	_, err := c.CoreV1().Endpoints(namespace).Create(&epRemote)

	return err
}

func (p *KubernetesProvider) updateEndpoints(eps *corev1.Endpoints, namespace string) error {
	foreignEps, err := p.foreignClient.CoreV1().Endpoints(namespace).Get(eps.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	for i:=0; i<len(eps.Subsets); i++ {
		for _, addr := range eps.Subsets[i].Addresses {
			if len(foreignEps.Subsets) == 0 {
				return k8sApiErrors.NewNotFound(schema.GroupResource{}, "endpoint subset")
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
