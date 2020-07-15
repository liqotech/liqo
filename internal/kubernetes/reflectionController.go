package kubernetes

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog"
	"sync"
	"time"
)

const (
	reflectedService   = "liqo/reflection"
	nReflectionWorkers = 2
)

type timestampedEndpoints struct {
	ep *v1.Endpoints
	t  time.Time
}

type Reflector struct {
	stop     chan struct{}
	svcEvent chan watch.Event
	epEvent  chan timestampedEndpoints
	cmEvent  chan watch.Event
	secEvent chan watch.Event

	workers *sync.WaitGroup
	svcwg   *sync.WaitGroup
	epwg    *sync.WaitGroup
	powg    *sync.WaitGroup
	cmwg    *sync.WaitGroup
	secwg   *sync.WaitGroup

	reflectedNamespaces struct {
		sync.Mutex
		ns map[string]chan struct{}
	}

	started bool
}

// StartReflector initializes all the data structures
// and creates a new goroutine running the reflector control loop
func (p *KubernetesProvider) StartReflector() {
	klog.Info("starting reflector for cluster " + p.foreignClusterId)

	p.reflectedNamespaces.ns = make(map[string]chan struct{})
	p.stop = make(chan struct{}, 1)
	p.svcEvent = make(chan watch.Event, 1000)
	p.epEvent = make(chan timestampedEndpoints, 1000)
	p.cmEvent = make(chan watch.Event, 1000)
	p.secEvent = make(chan watch.Event, 1000)

	p.workers = &sync.WaitGroup{}
	p.powg = &sync.WaitGroup{}
	p.epwg = &sync.WaitGroup{}
	p.svcwg = &sync.WaitGroup{}
	p.cmwg = &sync.WaitGroup{}
	p.secwg = &sync.WaitGroup{}

	for i := 0; i < nReflectionWorkers; i++ {
		p.workers.Add(1)
		go p.controlLoop()
	}

	p.started = true
	klog.Infof("vk reflector started with %d workers", nReflectionWorkers)
}

// main function of the reflector: this control loop watches 5 different channels
// having distinct meanings:
// * p.stop: the vk has been stopped, stop closes all opened channels
// * p.svcEvent: event regarding the creation, delete or update of a local service
// 				 in a monitored namespace
// * p.epEvent: event regarding the creation, delete or update of a local endpoint
// 				in a monitored namespace (we are only interested in the update events)
func (p *KubernetesProvider) controlLoop() {
	var err error

	for {
		select {
		case <-p.stop:
			p.workers.Done()
			return

		case e := <-p.svcEvent:
			if err = p.manageSvcEvent(e); err != nil {
				klog.Errorf("error in managing svc event - ERR: %v", err)
			}

		case ep := <-p.epEvent:
			if time.Since(ep.t) > epExpirationRate {
				klog.V(7).Infof("ep %v in namespace %v update ignored due to its expiration", ep.ep.Name, ep.ep.Namespace)
				break
			}
			if err = p.manageEpEvent(ep.ep); err != nil {
				klog.Errorf("error in managing ep event - ERR: %v", err)
			}
		case e := <-p.cmEvent:
			if err = p.manageCmEvent(e); err != nil {
				klog.Errorf("error in managing cm event - ERR: %v", err)
			}
		case e := <-p.secEvent:
			if err = p.manageSecEvent(e); err != nil {
				klog.Errorf("error in managing sec event - ERR: %v", err)
			}
		}
	}
}

func (p *KubernetesProvider) cleanupNamespace(ns string) error {

	svcs, err := p.foreignClient.Client().CoreV1().Services(ns).List(metav1.ListOptions{LabelSelector: reflectedService})
	if err != nil {
		return err
	}

	for _, svc := range svcs.Items {
		err = p.foreignClient.Client().CoreV1().Services(ns).Delete(svc.Name, &metav1.DeleteOptions{})
		if err != nil {
			klog.Error(err, "cannot delete remote service")
		}
	}

	cms, err := p.foreignClient.Client().CoreV1().ConfigMaps(ns).List(metav1.ListOptions{LabelSelector: reflectedService})
	if err != nil {
		return err
	}

	for _, cm := range cms.Items {
		err = p.foreignClient.Client().CoreV1().ConfigMaps(ns).Delete(cm.Name, &metav1.DeleteOptions{})
		if err != nil {
			klog.Error(err, "cannot delete remote configMap")
		}
	}

	secs, err := p.foreignClient.Client().CoreV1().Secrets(ns).List(metav1.ListOptions{LabelSelector: reflectedService})
	if err != nil {
		return err
	}

	for _, sec := range secs.Items {
		err = p.foreignClient.Client().CoreV1().Secrets(ns).Delete(sec.Name, &metav1.DeleteOptions{})
		if err != nil {
			klog.Error(err, "cannot delete remote secret")
		}
	}

	return nil
}

// close all the channels used by the reflector module
func (p *KubernetesProvider) closeChannels() {
	p.reflectedNamespaces.Lock()
	defer p.reflectedNamespaces.Unlock()

	for _, v := range p.reflectedNamespaces.ns {
		close(v)
	}
}

// addServiceWatcher receives a namespace to watch, creates a service watching chan and starts a routine
// that watches the local events regarding the services
func (p *KubernetesProvider) addServiceWatcher(namespace string, stop chan struct{}) error {
	svcWatch, err := p.homeClient.Client().CoreV1().Services(namespace).Watch(metav1.ListOptions{})
	if err != nil {
		return err
	}

	p.svcwg.Add(1)
	go eventsAggregator(svcWatch, p.svcEvent, stop, p.svcwg)

	klog.V(3).Infof("service reflector for home namespace \"%v\" started", namespace)
	return nil
}

func (p *KubernetesProvider) addEndpointWatcher(namespace string, stop chan struct{}) error {
	p.epwg.Add(1)

	nattedNS, err := p.NatNamespace(namespace, false)
	if err != nil {
		return err
	}
	p.homeEpCaches[namespace] = p.newHomeEpCache(p.homeClient.Client(), namespace, stop)
	p.foreignEpCaches[nattedNS] = p.newForeignEpCache(p.foreignClient.Client(), nattedNS, stop)

	klog.V(3).Infof("endpoint reflector for home namespace \"%v\" started", namespace)
	return nil
}

func (p *KubernetesProvider) addConfigMapWatcher(namespace string, stop chan struct{}) error {
	cmWatch, err := p.homeClient.Client().CoreV1().ConfigMaps(namespace).Watch(metav1.ListOptions{})
	if err != nil {
		klog.Errorf("error: %v - cannot watch configMaps in namespace %v", err, namespace)
		return err
	}

	p.cmwg.Add(1)
	go eventsAggregator(cmWatch, p.cmEvent, stop, p.cmwg)

	klog.V(3).Infof("configmap reflector for home namespace \"%v\" started", namespace)
	return nil
}

func (p *KubernetesProvider) addSecretWatcher(namespace string, stop chan struct{}) error {
	secWatch, err := p.homeClient.Client().CoreV1().Secrets(namespace).Watch(metav1.ListOptions{})
	if err != nil {
		klog.Error(err, "cannot watch secrets in namespace "+namespace)
		return err
	}

	p.secwg.Add(1)
	go eventsAggregator(secWatch, p.secEvent, stop, p.secwg)

	klog.V(3).Infof("secret reflector for home namespace \"%v\" started", namespace)
	return nil
}

func (p *KubernetesProvider) AddPodWatcher(namespace string, stop chan struct{}) error {
	poWatch, err := p.foreignClient.Client().CoreV1().Pods(namespace).Watch(metav1.ListOptions{})
	if err != nil {
		return err
	}

	p.powg.Add(1)

	c := newForeignPodCache(p.foreignClient.Client(), namespace)
	p.foreignPodCaches[namespace] = c

	go p.watchForeignPods(poWatch, stop)

	klog.V(3).Infof("foreign podWatcher for home namespace \"%v\" started", namespace)
	return nil
}

// eventsAggregator iterates over all the received channels and whenever a new event comes from the input chan,
// it pushes it to the output chan
func eventsAggregator(watcher watch.Interface, outChan chan watch.Event, stop chan struct{}, wg *sync.WaitGroup) {
	for {
		select {
		case <-stop:
			watcher.Stop()
			wg.Done()
			return

		case e := <-watcher.ResultChan():
			outChan <- e
		}
	}
}

// StopReflector must be called when the virtual kubelet end up: all the channels are correctly closed
// and the eventsAggregator goroutines closing are waited
func (p *KubernetesProvider) StopReflector() {
	klog.Info("stopping reflector for cluster " + p.foreignClusterId)

	p.started = false

	if p.svcEvent == nil || p.epEvent == nil || p.cmEvent == nil || p.secEvent == nil {
		klog.Info("reflector was not active for cluster " + p.foreignClusterId)
		return
	}

	p.closeChannels()
	close(p.stop)

	p.workers.Wait()
	p.powg.Wait()
	p.svcwg.Wait()
	p.epwg.Wait()
	p.cmwg.Wait()
	p.secwg.Wait()

}

func (p *KubernetesProvider) reflectNamespace(namespace string) error {
	var nattedNS string
	var err error

	nattedNS, err = p.NatNamespace(namespace, false)
	if err != nil {
		return err
	}

	stop := make(chan struct{}, 1)
	if err := p.addServiceWatcher(namespace, stop); err != nil {
		close(stop)
		return err
	}

	if err := p.addEndpointWatcher(namespace, stop); err != nil {
		close(stop)
		return err
	}

	if err := p.addConfigMapWatcher(namespace, stop); err != nil {
		close(stop)
		return err
	}

	if err := p.addSecretWatcher(namespace, stop); err != nil {
		close(stop)
		return err
	}

	if err := p.AddPodWatcher(nattedNS, stop); err != nil {
		close(stop)
		return err
	}

	p.reflectedNamespaces.ns[namespace] = stop

	klog.Infof("reflection setup completed - namespace \"%v\" is reflected in namespace \"%v\"", namespace, nattedNS)

	return nil
}

func (p *KubernetesProvider) isNamespaceReflected(ns string) bool {
	p.Reflector.reflectedNamespaces.Lock()
	defer p.Reflector.reflectedNamespaces.Unlock()

	_, ok := p.reflectedNamespaces.ns[ns]
	return ok
}
