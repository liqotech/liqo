package controller

import (
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesMapping"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	"sync"
)

const (
	nOutgoingReflectionWorkers = 2
	nIncomingReflectionWorkers = 2
)

type Controller struct {
	mapper                       namespacesMapping.MapperController
	outgoingReflectorsController OutGoingAPIReflectorsController
	incomingReflectorsController IncomingAPIReflectorsController

	outgoingReflectionGroup *sync.WaitGroup
	incomingReflectionGroup *sync.WaitGroup

	outgoingReflectionInforming chan apiReflection.ApiEvent
	incomingReflectionInforming chan apiReflection.ApiEvent

	started        bool
	stopReflection chan struct{}
	stopController chan struct{}
}

func NewApiController(homeClient, foreignClient kubernetes.Interface, mapper namespacesMapping.MapperController, opts map[options.OptionKey]options.Option) *Controller {
	klog.V(2).Infof("starting reflection manager")

	outgoingReflectionInforming := make(chan apiReflection.ApiEvent)
	incomingReflectionInforming := make(chan apiReflection.ApiEvent)

	c := &Controller{
		mapper:                       mapper,
		outgoingReflectorsController: NewOutgoingReflectorsController(homeClient, foreignClient, outgoingReflectionInforming, mapper, opts),
		incomingReflectorsController: NewIncomingReflectorsController(homeClient, foreignClient, incomingReflectionInforming, mapper, opts),
		outgoingReflectionGroup:      &sync.WaitGroup{},
		incomingReflectionGroup:      &sync.WaitGroup{},
		outgoingReflectionInforming:  outgoingReflectionInforming,
		incomingReflectionInforming:  incomingReflectionInforming,
		started:                      false,
		stopController:               make(chan struct{}),
	}

	go func() {
		for {
			select {
			case <-c.mapper.PollStartMapper():
				c.StartController()
			case <-c.mapper.PollStopMapper():
				c.StopReflection()
			case <-c.stopController:
				return
			default:
				break
			}
		}
	}()

	return c
}

func (c *Controller) outgoingReflectionControlLoop() {
	for {
		select {
		case <-c.stopReflection:
			c.outgoingReflectionGroup.Done()
			return

		case e := <-c.outgoingReflectionInforming:
			c.outgoingReflectorsController.DispatchEvent(e)
		default:
			break
		}
	}
}

func (c *Controller) incomingReflectionControlLoop() {
	for {
		select {
		case <-c.stopReflection:
			c.incomingReflectionGroup.Done()
			return

		case e := <-c.incomingReflectionInforming:
			c.incomingReflectorsController.DispatchEvent(e)
		default:
			break
		}
	}
}

func (c *Controller) SetInformingFunc(api apiReflection.ApiType, handler func(interface{})) {
	c.incomingReflectorsController.SetInforming(api, handler)
}

func (c *Controller) GetMirroredObjectByKey(api apiReflection.ApiType, namespace string, name string) interface{} {
	return c.incomingReflectorsController.GetMirroredObject(api, namespace, name)
}

func (c *Controller) ListMirroredObjects(api apiReflection.ApiType, namespace string) []interface{} {
	return c.incomingReflectorsController.ListMirroredObjects(api, namespace)
}

func (c *Controller) StartController() {
	klog.V(2).Info("starting api controller")

	c.stopReflection = make(chan struct{})

	for i := 0; i < nOutgoingReflectionWorkers; i++ {
		c.outgoingReflectionGroup.Add(1)
		go c.outgoingReflectionControlLoop()
	}

	for i := 0; i < nIncomingReflectionWorkers; i++ {
		c.incomingReflectionGroup.Add(1)
		go c.incomingReflectionControlLoop()
	}

	go c.outgoingReflectorsController.Start()
	go c.incomingReflectorsController.Start()

	c.started = true
	klog.V(2).Infof("api controller started with %v workers", nOutgoingReflectionWorkers)
}

func (c *Controller) StopController() {
	close(c.stopController)
}

func (c *Controller) StopReflection() {
	klog.V(2).Info("stopping reflection manager")

	c.outgoingReflectorsController.Stop()
	c.incomingReflectorsController.Stop()

	close(c.stopReflection)

	c.outgoingReflectionGroup.Wait()
	c.incomingReflectionGroup.Wait()

	c.started = false
	c.mapper.ReadyForRestart()

	klog.V(2).Info("api controller stopped")
}
