package kubernetes

import (
	"context"
	"github.com/liqotech/liqo/pkg/virtualNode/apiReflection"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"sync"
	"time"
)

const (
	reflectedService    = "liqo/reflection"
	nReflectionWorkers  = 2
	defaultResyncPeriod = 10*time.Second
)

type timestampedEndpoints struct {
	ep *v1.Endpoints
	t  time.Time
}

type ReflectionManager struct {
	stop     chan struct{}

	reflectorController *apiReflection.APIReflectorController

	workers   *sync.WaitGroup
	powg      *sync.WaitGroup
	informing chan apiReflection.ApiEvent

	reflectedNamespaces struct {
		sync.Mutex
		ns map[string]chan struct{}
	}

	started bool
}

// StartReflector initializes all the data structures
// and creates a new goroutine running the reflector control loop
func (p *KubernetesProvider) StartReflector() {
	klog.Infof("starting reflector for cluster %v", p.foreignClusterId)

	p.reflectedNamespaces.ns = make(map[string]chan struct{})
	p.informing = make(chan apiReflection.ApiEvent)
	p.stop = make(chan struct{})
	p.reflectorController = apiReflection.NewAPIReflectorController(p.homeClient.Client(), p.foreignClient.Client(), p.informing)

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

		case e := <-p.informing:
			if err = p.reflectorController.DispatchEvent(e); err != nil {
				klog.Errorf("error in managing event - ERR: %v", err)
			}
		default:

		}
	}
}

func (p *KubernetesProvider) cleanupNamespace(ns string) error {

	pods, err := p.foreignClient.Client().CoreV1().Pods(ns).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, po := range pods.Items {
		err = p.foreignClient.Client().CoreV1().Pods(ns).Delete(context.TODO(), po.Name, metav1.DeleteOptions{})
		if err != nil {
			klog.Errorf("cannot delete remote pod %v - %v", po.Name, err)
		}
	}

	svcs, err := p.foreignClient.Client().CoreV1().Services(ns).List(context.TODO(), metav1.ListOptions{LabelSelector: reflectedService})
	if err != nil {
		return err
	}

	for _, svc := range svcs.Items {
		err = p.foreignClient.Client().CoreV1().Services(ns).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
		if err != nil {
			klog.Errorf("cannot delete remote service %v - %v", svc.Name, err)
		}
	}

	cms, err := p.foreignClient.Client().CoreV1().ConfigMaps(ns).List(context.TODO(), metav1.ListOptions{LabelSelector: reflectedService})
	if err != nil {
		return err
	}

	for _, cm := range cms.Items {
		err = p.foreignClient.Client().CoreV1().ConfigMaps(ns).Delete(context.TODO(), cm.Name, metav1.DeleteOptions{})
		if err != nil {
			klog.Errorf("cannot delete remote configMap %v - %v", cm.Name, err)
		}
	}

	secs, err := p.foreignClient.Client().CoreV1().Secrets(ns).List(context.TODO(), metav1.ListOptions{LabelSelector: reflectedService})
	if err != nil {
		return err
	}

	for _, sec := range secs.Items {
		err = p.foreignClient.Client().CoreV1().Secrets(ns).Delete(context.TODO(), sec.Name, metav1.DeleteOptions{})
		if err != nil {
			klog.Errorf("cannot delete remote secret %v - %v", sec.Name, err)
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

func (p *KubernetesProvider) AddPodWatcher(namespace string, stop chan struct{}) error {
	poWatch, err := p.foreignClient.Client().CoreV1().Pods(namespace).Watch(context.TODO(), metav1.ListOptions{})
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

// StopReflector must be called when the virtual kubelet end up: all the channels are correctly closed
// and the eventsAggregator goroutines closing are waited
func (p *KubernetesProvider) StopReflector() {
	klog.Info("stopping reflector for cluster " + p.foreignClusterId)

	p.started = false

	p.closeChannels()
	close(p.stop)

	p.workers.Wait()
	p.powg.Wait()
}

func (p *KubernetesProvider) reflectNamespace(namespace string) error {
	var nattedNS string
	var err error

	nattedNS, err = p.NatNamespace(namespace, false)
	if err != nil {
		return err
	}

	stop := make(chan struct{})

	if err := p.AddPodWatcher(nattedNS, stop); err != nil {
		close(stop)
		return err
	}

	p.reflectorController.ReflectNamespace(namespace, defaultResyncPeriod, nil)
	p.reflectedNamespaces.ns[namespace] = stop

	klog.Infof("reflection setup completed - namespace \"%v\" is reflected in namespace \"%v\"", namespace, nattedNS)

	return nil
}

func (p *KubernetesProvider) isNamespaceReflected(ns string) bool {
	p.ReflectionManager.reflectedNamespaces.Lock()
	defer p.ReflectionManager.reflectedNamespaces.Unlock()

	_, ok := p.reflectedNamespaces.ns[ns]
	return ok
}
