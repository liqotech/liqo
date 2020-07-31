package kubernetes

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
	"strings"
	"time"
)

const (
	// epUpdateRate defines the minimum interval in which to push the endpoints in the ep queue
	epUpdateRate = 500 * time.Millisecond
	// epExpirationRate defines whether if the ep in the queue is too old to be handled
	epExpirationRate = 100 * time.Millisecond

	// resync periods of the local and foreign caches
	epHomeCacheResyncPeriod    = 1 * time.Second
	epForeignCacheResyncPeriod = 2 * time.Second
)

// manageEpEvent gets an event a local endpoints resource,
// translates the namespace, then gets the reflected endpoints,
// checks whether the remote instance has to be updated according the the local one,
// and finally calls the updateEndpoints function
func (p *KubernetesProvider) manageEpEvent(endpoints *corev1.Endpoints) error {
	klog.V(3).Infof("received update of endpoint %v", endpoints.Name)

	nattedNS, err := p.NatNamespace(endpoints.Namespace, false)
	if err != nil {
		return err
	}

	foreignEps, err := p.foreignEpCaches[nattedNS].get(endpoints.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if !hasToBeUpdated(endpoints.Subsets, foreignEps.Subsets) {
		klog.V(5).Infof("ep %v hasn't to be updated", endpoints.Name)
		return nil
	}

	klog.V(5).Infof("ep %v has to be updated", endpoints.Name)
	foreignEps.Subsets = p.updateEndpoints(endpoints.Subsets, foreignEps.Subsets)

	if foreignEps.Labels == nil {
		foreignEps.Labels = make(map[string]string)
	}
	foreignEps.Namespace = nattedNS
	_, err = p.foreignClient.Client().CoreV1().Endpoints(nattedNS).Update(context.TODO(), foreignEps, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	klog.V(3).Infof("Endpoint %v with version %v correctly reconciliated", endpoints.Name, endpoints.ResourceVersion)
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

		if len(foreignEps) == 0 {
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

// hasToBeUpdated gets two different endpoints instances (local and remote) and
// check if the remote instance has to be updated according to the local one
func hasToBeUpdated(home, foreign []corev1.EndpointSubset) bool {
	if len(home) != len(foreign) {
		klog.V(6).Info("the ep has to be updated because home and foreign subsets lengths are different")
		return true
	}
	for i := 0; i < len(home); i++ {
		if len(home[i].Addresses) != len(foreign[i].Addresses) {
			klog.V(6).Info("the ep has to be updated because home and foreign addresses lengths are different")
			return true
		}
		for j := 0; j < len(home[i].Addresses); j++ {
			if home[i].Addresses[j].IP != foreign[i].Addresses[j].IP {
				klog.V(6).Info("the ep has to be updated because home and foreign IPs are different")
				return true
			}
		}
	}

	return false
}

// epCache is a structure that serves as endpoints cache and can be used both locally an remotely
type epCache struct {
	client    kubernetes.Interface
	namespace string
	store     cache.Store
	stop      chan struct{}
}

// epCache.get is a method that allows to fetch a specific endpoints from the cache and, if not found,
// fetches it remotely by using the client
func (c *epCache) get(name string, options metav1.GetOptions) (*corev1.Endpoints, error) {
	n := strings.Join([]string{c.namespace, name}, "/")
	ep, found, _ := c.store.GetByKey(n)
	if found {
		klog.V(6).Infof("endpoint %v fetched from cache", name)
		return ep.(*corev1.Endpoints), nil
	}

	ep, err := c.client.CoreV1().Endpoints(c.namespace).Get(context.TODO(), name, options)
	if err != nil {
		return nil, err
	}

	klog.V(6).Infof("endpoint %v fetched from remote", name)
	return ep.(*corev1.Endpoints), nil
}

// epCache.get is a method that allows to list all the endpoints from the cache and,
// if not eps are found locally, then they are fetched remotely
func (c *epCache) list(options metav1.ListOptions) (*corev1.EndpointsList, error) {
	epList := &corev1.EndpointsList{
		Items: []corev1.Endpoints{},
	}
	if c.store == nil {
		klog.V(6).Info("endpoints listed from remote")
		return c.client.CoreV1().Endpoints(c.namespace).List(context.TODO(), options)
	}

	eps := c.store.List()
	if eps == nil {
		klog.V(6).Info("endpoints listed from remote")
		return c.client.CoreV1().Endpoints(c.namespace).List(context.TODO(), options)
	}

	for _, ep := range eps {
		epList.Items = append(epList.Items, ep.(corev1.Endpoints))
	}

	klog.V(6).Info("endpoints listed from cache")
	return epList, nil
}

// newForeignEpCache creates a new cache that serves the remote endpoints.
func (p *KubernetesProvider) newForeignEpCache(c kubernetes.Interface, namespace string, stopChan chan struct{}) *epCache {
	listFunc := func(ls metav1.ListOptions) (result runtime.Object, err error) {
		return c.CoreV1().Endpoints(namespace).List(context.TODO(), ls)
	}
	watchFunc := func(ls metav1.ListOptions) (watch.Interface, error) {
		return c.CoreV1().Endpoints(namespace).Watch(context.TODO(), ls)
	}

	store, controller := cache.NewInformer(
		&cache.ListWatch{
			ListFunc:  listFunc,
			WatchFunc: watchFunc,
		},
		&corev1.Endpoints{},
		epForeignCacheResyncPeriod,
		cache.ResourceEventHandlerFuncs{},
	)

	go controller.Run(stopChan)

	return &epCache{
		client:    c,
		store:     store,
		stop:      stopChan,
		namespace: namespace,
	}
}

// newForeignEpCache creates a new cache that serves the remote endpoints.
// whenever the onUpdate function is triggered, the endpoints resource is pushed to the channel handled by the
// control loop mechanism
func (p *KubernetesProvider) newHomeEpCache(c kubernetes.Interface, namespace string, stopChan chan struct{}) *epCache {
	lastUpdates := make(map[string]time.Time)
	listFunc := func(ls metav1.ListOptions) (result runtime.Object, err error) {
		eps, err := c.CoreV1().Endpoints(namespace).List(context.TODO(), ls)
		if err != nil {
			return eps, err
		}
		for k := range eps.Items {
			ep := eps.Items[k]
			p.epEvent <- timestampedEndpoints{
				ep: &ep,
				t:  time.Now(),
			}
		}

		return eps, err
	}
	watchFunc := func(ls metav1.ListOptions) (watch.Interface, error) {
		return c.CoreV1().Endpoints(namespace).Watch(context.TODO(), ls)
	}

	store, controller := cache.NewInformer(
		&cache.ListWatch{
			ListFunc:  listFunc,
			WatchFunc: watchFunc,
		},
		&corev1.Endpoints{},
		epHomeCacheResyncPeriod,
		cache.ResourceEventHandlerFuncs{
			UpdateFunc: func(oldObj, newObj interface{}) {
				ep := newObj.(*corev1.Endpoints)
				if t, ok := lastUpdates[ep.Name]; !ok || time.Since(t) >= epUpdateRate {
					p.epEvent <- timestampedEndpoints{
						ep: ep,
						t:  time.Now(),
					}
					lastUpdates[ep.Name] = time.Now()
				}
			},
		},
	)

	go controller.Run(stopChan)

	return &epCache{
		client:    c,
		store:     store,
		stop:      stopChan,
		namespace: namespace,
	}
}
