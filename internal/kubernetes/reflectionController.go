package kubernetes

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sync"
	"time"
)

const (
	reflectedService   = "liqo/reflection"
	timestampedLabel   = "localLastTimestamp"
	nReflectionWorkers = 10
)

type timestampedEvent struct {
	event watch.Event

	ts int64
}

type Reflector struct {
	stop     chan bool
	svcEvent chan watch.Event
	repEvent chan watch.Event
	epEvent  chan timestampedEvent
	cmEvent  chan watch.Event
	secEvent chan watch.Event

	workers *sync.WaitGroup
	svcwg   *sync.WaitGroup
	repwg   *sync.WaitGroup
	epwg    *sync.WaitGroup
	powg    *sync.WaitGroup
	cmwg    *sync.WaitGroup
	secwg   *sync.WaitGroup

	reflectedNamespaces struct {
		sync.Mutex
		ns map[string]chan struct{}
	}
}

// StartReflector initializes all the data structures
// and creates a new goroutine running the reflector control loop
func (p *KubernetesProvider) StartReflector() {
	p.log = ctrl.Log.WithName("reflector")
	p.log.Info("starting reflector for cluster " + p.foreignClusterId)

	p.reflectedNamespaces.ns = make(map[string]chan struct{})
	p.stop = make(chan bool, 1000)
	p.svcEvent = make(chan watch.Event, 1000)
	p.epEvent = make(chan timestampedEvent, 1000)
	p.repEvent = make(chan watch.Event, 1000)
	p.cmEvent = make(chan watch.Event, 1000)
	p.secEvent = make(chan watch.Event, 1000)

	p.workers = &sync.WaitGroup{}
	p.powg = &sync.WaitGroup{}
	p.epwg = &sync.WaitGroup{}
	p.svcwg = &sync.WaitGroup{}
	p.repwg = &sync.WaitGroup{}
	p.cmwg = &sync.WaitGroup{}
	p.secwg = &sync.WaitGroup{}

	for i := 0; i < nReflectionWorkers; i++ {
		p.workers.Add(1)
		go p.controlLoop()
	}
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
			p.closeChannels()
			p.workers.Done()
			return

		case e := <-p.svcEvent:
			if err = p.manageSvcEvent(e); err != nil {
				p.log.Error(err, "error in managing svc event")
			}

		case e := <-p.epEvent:
			if e.event.Type != watch.Modified {
				break
			}
			if err = p.manageEpEvent(e); err != nil {
				// if the resource is not found, it has not been remotely created yet:
				// we launch a goroutine that waits one second, then pushes the event again
				// in the channel
				go func(e timestampedEvent, ch chan timestampedEvent) {
					time.Sleep(time.Second)
					ch <- e
				}(e, p.epEvent)
			}
		case e := <-p.repEvent:
			if e.Type != watch.Modified {
				break
			}
			if err := p.manageRemoteEpEvent(e); err != nil {
				p.log.Error(err, "error in managing remote ep event")
			}
		case e := <-p.cmEvent:
			if err = p.manageCmEvent(e); err != nil {
				p.log.Error(err, "error in managing cm event")
			}
		case e := <-p.secEvent:
			if err = p.manageSecEvent(e); err != nil {
				p.log.Error(err, "error in managing sec event")
			}
		}
	}
}

// when a namespace counter reaches 0, the namespace has to be cleaned up (the reflected service must be deleted)
func (p *KubernetesProvider) cleanupNamespace(ns string) error {

	svcs, err := p.foreignClient.Client().CoreV1().Services(ns).List(metav1.ListOptions{LabelSelector: reflectedService})
	if err != nil {
		return err
	}

	for _, svc := range svcs.Items {
		err = p.foreignClient.Client().CoreV1().Services(ns).Delete(svc.Name, &metav1.DeleteOptions{})
		if err != nil {
			p.log.Error(err, "cannot delete remote service")
		}
	}

	cms, err := p.foreignClient.Client().CoreV1().ConfigMaps(ns).List(metav1.ListOptions{LabelSelector: reflectedService})
	if err != nil {
		return err
	}

	for _, cm := range cms.Items {
		err = p.foreignClient.Client().CoreV1().ConfigMaps(ns).Delete(cm.Name, &metav1.DeleteOptions{})
		if err != nil {
			p.log.Error(err, "cannot delete remote configMap")
		}
	}

	secs, err := p.foreignClient.Client().CoreV1().Secrets(ns).List(metav1.ListOptions{LabelSelector: reflectedService})
	if err != nil {
		return err
	}

	for _, sec := range secs.Items {
		err = p.foreignClient.Client().CoreV1().Secrets(ns).Delete(sec.Name, &metav1.DeleteOptions{})
		if err != nil {
			p.log.Error(err, "cannot delete remote secret")
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
	go eventAggregator(svcWatch, p.svcEvent, stop, p.svcwg)

	return nil
}

// addEndpointWatcher receives a namespace to watch, creates an endpoints watching chan and starts a routine
// that watches the local events regarding the endpoints
func (p *KubernetesProvider) addEndpointWatcher(namespace string, stop chan struct{}) error {
	epWatch, err := p.homeClient.Client().CoreV1().Endpoints(namespace).Watch(metav1.ListOptions{})
	if err != nil {
		return err
	}

	p.epwg.Add(1)
	go epEventsAggregator(epWatch, p.epEvent, stop, p.epwg)

	return nil
}

// addEndpointWatcher receives a namespace to watch, creates an endpoints watching chan and starts a routine
// that watches the local events regarding the endpoints
func (p *KubernetesProvider) addRemoteEndpointWatcher(namespace string, stop chan struct{}) error {
	epWatch, err := p.foreignClient.Client().CoreV1().Endpoints(namespace).Watch(metav1.ListOptions{})
	if err != nil {
		return err
	}

	p.repwg.Add(1)
	go eventAggregator(epWatch, p.repEvent, stop, p.repwg)

	return nil
}

func (p *KubernetesProvider) addConfigMapWatcher(namespace string, stop chan struct{}) error {
	cmWatch, err := p.homeClient.Client().CoreV1().ConfigMaps(namespace).Watch(metav1.ListOptions{})
	if err != nil {
		p.log.Error(err, "cannot watch configMaps in namespace "+namespace)
		return err
	}

	p.cmwg.Add(1)
	go eventAggregator(cmWatch, p.cmEvent, stop, p.cmwg)
	return nil
}

func (p *KubernetesProvider) addSecretWatcher(namespace string, stop chan struct{}) error {
	secWatch, err := p.homeClient.Client().CoreV1().Secrets(namespace).Watch(metav1.ListOptions{})
	if err != nil {
		p.log.Error(err, "cannot watch secrets in namespace "+namespace)
		return err
	}

	p.secwg.Add(1)
	go eventAggregator(secWatch, p.secEvent, stop, p.cmwg)
	return nil
}

func (p *KubernetesProvider) AddPodWatcher(ns string, stop chan struct{}) error {
	poWatch, err := p.foreignClient.Client().CoreV1().Pods(ns).Watch(metav1.ListOptions{})
	if err != nil {
		return err
	}

	p.powg.Add(1)
	go p.watchForeignPods(poWatch, stop)

	return nil
}

func epEventsAggregator(watcher watch.Interface, outChan chan timestampedEvent, stop chan struct{}, wg *sync.WaitGroup) {
	for {
		select {
		case <-stop:
			watcher.Stop()
			wg.Done()
			return

		case e := <-watcher.ResultChan():
			outChan <- timestampedEvent{
				event: e,
				ts:    time.Now().UnixNano(),
			}
		}
	}
}

// eventAggregator iterates over all the received channels and whenever a new event comes from the input chan,
// it pushes it to the output chan
func eventAggregator(watcher watch.Interface, outChan chan watch.Event, stop chan struct{}, wg *sync.WaitGroup) {
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
// and the eventAggregator goroutines closing are waited
func (p *KubernetesProvider) StopReflector() {
	p.log.Info("stopping reflector for cluster " + p.foreignClusterId)

	if p.svcEvent == nil || p.epEvent == nil || p.cmEvent == nil || p.secEvent == nil {
		p.log.Info("reflector was not active for cluster " + p.foreignClusterId)
		return
	}

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

	nattedNS, err = p.NatNamespace(namespace)
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

	if err := p.addRemoteEndpointWatcher(nattedNS, stop); err != nil {
		close(stop)
		return err
	}

	if err := p.AddPodWatcher(nattedNS, stop); err != nil {
		close(stop)
		return err
	}

	p.reflectedNamespaces.ns[namespace] = stop

	return nil
}
