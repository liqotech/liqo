package kubernetes

import (
	"errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sync"
	"time"
)

const (
	reflectedService = "liqo/reflection"
)

type namespaceRequest string

type counterStruct struct {
	counter int
	stop chan bool
}

type Reflector struct {
	stop  chan bool
	incNs chan namespaceRequest
	decNs chan namespaceRequest
	svcEvent chan watch.Event
	epEvent chan watch.Event

	svcwg *sync.WaitGroup
	epwg *sync.WaitGroup

	namespaces struct {
		sync.Mutex
		pods map[namespaceRequest]counterStruct
	}
}

// StartReflector initializes all the data structures
// and creates a new goroutine running the reflector control loop
func (p *KubernetesProvider) StartReflector() {
	p.log = ctrl.Log.WithName("reflector")
	p.log.Info("starting reflector for cluster " + p.clusterId)

	p.namespaces.pods = make(map[namespaceRequest]counterStruct)

	p.stop = make(chan bool, 1)
	p.incNs = make(chan namespaceRequest, 1)
	p.decNs = make(chan namespaceRequest, 1)
	p.svcEvent = make(chan watch.Event, 1)
	p.epEvent = make(chan watch.Event, 1)

	p.epwg = &sync.WaitGroup{}
	p.svcwg = &sync.WaitGroup{}

	go p.controlLoop()
}

// publishPod gets a pod, creates a namespaceRequest and pushes it
// into the incNs chan
func (p* KubernetesProvider) publishPod(pod *corev1.Pod) {
	ns := namespaceRequest(pod.Namespace)

	p.incNs <- ns
}

// deletePod gets a pod, creates a namespaceRequest and pushes it
// into the decNs chan
func (p* KubernetesProvider) deletePod(pod *corev1.Pod) {
	ns := namespaceRequest(pod.Namespace)

	p.decNs <- ns
}

// increaseNamespace gets a nr that represents that a new pod
// has been created in a certain ns. This function increases by one
// the pod counter of that ns
func (p* KubernetesProvider) increaseNamespace(nr namespaceRequest) error {
	p.namespaces.Lock()
	defer p.namespaces.Unlock()

	var err error

	if v, ok := p.namespaces.pods[nr]; ok {
		v.counter ++
		p.namespaces.pods[nr] = v
	} else {
		v := counterStruct{counter:1, stop:make(chan bool, 1)}
		p.namespaces.pods[nr] = v

		err = p.addServiceWatcher(string(nr), p.namespaces.pods[nr].stop)
		if err != nil {
			return err
		}
		err = p.addEndpointWatcher(string(nr), p.namespaces.pods[nr].stop)
		if err != nil {
			return err
		}
	}

	return nil
}

// decreaseNamespace gets a nr that represents that a pod
// has been deleted in a certain ns. This function decreases by one
// the pod counter of that ns
func (p* KubernetesProvider) decreaseNamespace(nr namespaceRequest) error {
	p.namespaces.Lock()
	defer p.namespaces.Unlock()

	if v, ok := p.namespaces.pods[nr]; ok {
		v.counter --
		p.namespaces.pods[nr] = v
	} else {
		return errors.New("namespace not reflected")
	}

	if p.namespaces.pods[nr].counter == 0 {
		nattedNS := p.NatNamespace(string(nr), false)
		if nattedNS == "" {
			return errors.New("trying to clean a non natted namespace")
		}
		close(p.namespaces.pods[nr].stop)
		delete(p.namespaces.pods, nr)

		return p.cleanupNamespace(nattedNS)
	}

	return nil
}

// main function of the reflector: this control loop watches 5 different channels
// having distinct meanings:
// * p.stop: the vk has been stopped, stop closes all opened channels
// * p.Reflector.incNs: a new pod has been created in a certain namespace
// * p.Reflector.decNs: a pod has been deleted in a certain namespace
// * p.svcEvent: event regarding the creation, delete or update of a local service
// 				 in a monitored namespace
// * p.epEvent: event regarding the creation, delete or update of a local endpoint
// 				in a monitored namespace (we are only interested in the update events)
func (p* KubernetesProvider) controlLoop() {
	var err error

	for {
		select {
		case <-p.stop:
			p.closeChannels()
			return

		case nr := <-p.Reflector.incNs:
			if err = p.increaseNamespace(nr); err != nil {
				p.log.Error(err, "error in namespace cardinality increase")
			}

		case nr := <-p.Reflector.decNs:
			if err = p.decreaseNamespace(nr); err != nil {
				p.log.Error(err, "error in namespace cardinality decrease")
			}

		case e := <-p.svcEvent:
			if err = p.manageSvcEvent(e); err != nil {
				p.log.Error(err, "error in managing svc event")
			}

		case e := <-p.epEvent:
			if e.Type != watch.Modified {
				break
			}

			if err = p.manageEpEvent(e); err != nil {
				if k8serrors.IsNotFound(err) {
					// if the resource is not found, it has not been remotely created yet:
					// we launch a goroutine that waits one second, then pushes the event again
					// in the channel
					go func(e watch.Event, ch chan watch.Event) {
						time.Sleep(1000 * time.Millisecond)
						ch <- e
					}(e, p.epEvent)
				} else {
					p.log.Error(err, "error in managing ep event")
				}
			}
		}
	}
}

// when a namespace counter reaches 0, the namespace has to be cleaned up (the reflected service must be deleted)
func (p* KubernetesProvider) cleanupNamespace(ns string) error {

	svcs, err := p.foreignClient.CoreV1().Services(ns).List(metav1.ListOptions{LabelSelector:reflectedService})
	if err != nil {
		return err
	}

	for _, svc := range svcs.Items {
		err = p.foreignClient.CoreV1().Services(ns).Delete(svc.Name, &metav1.DeleteOptions{})
		if err != nil {
			p.log.Error(err, "cannot delete remote service")
		}
	}

	return nil
}

// close all the channels used by the reflector module
func (p* KubernetesProvider) closeChannels() {
	p.namespaces.Lock()
	defer p.namespaces.Unlock()

	for _, v := range p.namespaces.pods {
		close(v.stop)
	}
}

// addServiceWatcher receives a namespace to watch, creates a service watching chan and starts a routine
// that watches the local events regarding the services
func (p* KubernetesProvider) addServiceWatcher(namespace string, stop chan bool) error {
	svcWatch, err := p.homeClient.CoreV1().Services(namespace).Watch(metav1.ListOptions{})
	if err != nil {
		return err
	}

	p.svcwg.Add(1)
	go eventAggregator(svcWatch, p.svcEvent, stop, p.svcwg)

	return nil
}

// addEndpointWatcher receives a namespace to watch, creates an endpoints watching chan and starts a routine
// that watches the local events regarding the endpoints
func (p* KubernetesProvider) addEndpointWatcher(namespace string, stop chan bool) error {
	epWatch, err := p.homeClient.CoreV1().Endpoints(namespace).Watch(metav1.ListOptions{})
	if err != nil {
		return err
	}

	p.epwg.Add(1)
	go eventAggregator(epWatch, p.epEvent, stop, p.epwg)

	return nil
}

// eventAggregator iterates over all the received channels and whenever a new event comes from the input chan,
// it pushes it to the output chan
func eventAggregator(watcher watch.Interface, outChan chan watch.Event, stop chan bool, wg *sync.WaitGroup) {
	for {
		select{
		case <- stop:
			watcher.Stop()
			wg.Done()
			return

		case e := <- watcher.ResultChan():
			outChan <- e
		}
	}
}

// StopReflector must be called when the virtual kubelet end up: all the channels are correctly closed
// and the eventAggregator goroutines closing are waited
func (p *KubernetesProvider) StopReflector() {
	p.log.Info("stopping reflector for cluster " + p.clusterId)

	if p.svcEvent == nil || p.epEvent == nil {
		p.log.Info("reflector was not active for cluster " + p.clusterId)
		return
	}

	close(p.stop)

	p.svcwg.Wait()
	p.epwg.Wait()
}

