package controller

import (
	"errors"
	"sync"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"

	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesMapping"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options"
	"github.com/liqotech/liqo/pkg/virtualKubelet/storage"
)

const (
	nOutgoingReflectionWorkers = 2
	nIncomingReflectionWorkers = 2
)

type ApiController interface {
	SetInformingFunc(apiReflection.ApiType, func(interface{}))
	CacheManager() storage.CacheManagerReaderAdder
	StartController()
	StopController() error
	StopReflection(restart bool)
}

type Controller struct {
	mapper                       namespacesMapping.MapperController
	cacheManager                 storage.CacheManagerReaderAdder
	outgoingReflectorsController OutGoingAPIReflectorsController
	incomingReflectorsController IncomingAPIReflectorsController

	mainControllerRoutine   *sync.WaitGroup
	outgoingReflectionGroup *sync.WaitGroup
	incomingReflectionGroup *sync.WaitGroup

	outgoingReflectionInforming chan apiReflection.ApiEvent
	incomingReflectionInforming chan apiReflection.ApiEvent

	started        bool
	stopReflection chan struct{}
	stopController chan struct{}
}

func NewApiController(homeClient, foreignClient kubernetes.Interface, informerResyncPeriod time.Duration, mapper namespacesMapping.MapperController, opts map[options.OptionKey]options.Option, tepReady chan struct{}) *Controller {
	klog.V(2).Infof("starting reflection manager")

	outgoingReflectionInforming := make(chan apiReflection.ApiEvent)
	incomingReflectionInforming := make(chan apiReflection.ApiEvent)
	cacheManager := storage.NewManager(homeClient, foreignClient, informerResyncPeriod)

	c := &Controller{
		mapper:                       mapper,
		outgoingReflectorsController: NewOutgoingReflectorsController(homeClient, foreignClient, cacheManager, outgoingReflectionInforming, mapper, opts),
		incomingReflectorsController: NewIncomingReflectorsController(homeClient, foreignClient, cacheManager, incomingReflectionInforming, mapper, opts),
		outgoingReflectionGroup:      &sync.WaitGroup{},
		incomingReflectionGroup:      &sync.WaitGroup{},
		mainControllerRoutine:        &sync.WaitGroup{},
		outgoingReflectionInforming:  outgoingReflectionInforming,
		incomingReflectionInforming:  incomingReflectionInforming,
		cacheManager:                 cacheManager,
		started:                      false,
		stopController:               make(chan struct{}),
	}

	c.mainControllerRoutine.Add(1)
	go func() {
		<-tepReady
		for {
			select {
			case <-c.mapper.PollStartMapper():
				c.StartController()
			case <-c.mapper.PollStopMapper():
				c.StopReflection(true)
			case <-c.stopController:
				c.mainControllerRoutine.Done()
				return
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
		}
	}
}

func (c *Controller) SetInformingFunc(api apiReflection.ApiType, handler func(interface{})) {
	c.incomingReflectorsController.SetInforming(api, handler)
}

func (c *Controller) CacheManager() storage.CacheManagerReaderAdder {
	return c.cacheManager
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

func (c *Controller) StopController() error {
	select {
	case <-c.stopController:
		return errors.New("controller stop has already been called")
	default:
		break
	}
	close(c.stopController)
	c.mainControllerRoutine.Wait()
	c.StopReflection(false)
	klog.V(2).Info("Reflection controller stopped")

	return nil
}

func (c *Controller) StopReflection(restart bool) {
	klog.V(2).Info("stopping reflection manager")

	c.outgoingReflectorsController.Stop()
	c.incomingReflectorsController.Stop()

	close(c.stopReflection)

	c.outgoingReflectionGroup.Wait()
	c.incomingReflectionGroup.Wait()

	c.started = false
	if restart {
		c.mapper.ReadyForRestart()
	}
}
