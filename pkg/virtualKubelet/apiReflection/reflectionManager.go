package apiReflection

import (
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/api"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesMapping"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	"sync"
)

const (
	nReflectionWorkers = 2
)

type reflectionManager struct {
	mapper              namespacesMapping.NamespaceController
	reflectorController *APIReflectorsController

	workersGroup *sync.WaitGroup
	podsGroup    *sync.WaitGroup
	informing    chan api.ApiEvent

	started bool
	stop    chan struct{}
}

func NewReflectionManager(homeClient, foreignClient kubernetes.Interface, controller namespacesMapping.NamespaceController) *reflectionManager {
	klog.V(2).Infof("starting reflection manager")

	informing := make(chan api.ApiEvent)
	stop := make(chan struct{})

	manager := &reflectionManager{
		mapper:              controller,
		reflectorController: NewAPIReflectorsController(homeClient, foreignClient, informing, controller),
		workersGroup:        &sync.WaitGroup{},
		podsGroup:           &sync.WaitGroup{},
		informing:           informing,
		started:             false,
		stop:                stop,
	}

	for i := 0; i < nReflectionWorkers; i++ {
		manager.workersGroup.Add(1)
		go manager.controlLoop()
	}

	manager.started = true
	klog.V(2).Infof("Reflection manager started with %v workers", nReflectionWorkers)

	return manager
}

func (m *reflectionManager) controlLoop() {
	for {
		select {
		case <-m.stop:
			m.workersGroup.Done()
			return

		case e := <-m.informing:
			m.reflectorController.DispatchEvent(e)
		default:
		}
	}
}

/*
func (m *reflectionManager) AddPodWatcher(namespace string, stop chan struct{}) error {
	poWatch, err := p.foreignClient.Client().CoreV1().Pods(namespace).Watch(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	p.powg.Add(1)

	c := kubernetes.newForeignPodCache(p.foreignClient.Client(), namespace)
	p.foreignPodCaches[namespace] = c

	go p.watchForeignPods(poWatch, stop)

	klog.V(3).Infof("foreign podWatcher for home namespace \"%v\" started", namespace)
	return nil
}
*/

func (m *reflectionManager) StopReflector() {
	klog.V(2).Info("stopping reflection manager")

	close(m.stop)
	m.reflectorController.Stop()

	m.workersGroup.Wait()
	m.podsGroup.Wait()

	klog.V(2).Info("Reflection manager stopped")

	m.started = false
}
