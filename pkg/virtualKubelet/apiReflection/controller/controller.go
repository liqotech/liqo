// Copyright 2019-2021 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"errors"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesmapping"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options"
	"github.com/liqotech/liqo/pkg/virtualKubelet/storage"
)

const (
	nOutgoingReflectionWorkers = 2
	nIncomingReflectionWorkers = 2
)

// APIController defines the interface exposed by a controller implementing API reflection.
type APIController interface {
	SetInformingFunc(apiReflection.ApiType, func(*corev1.Pod))
	CacheManager() storage.CacheManagerReaderAdder
	StartController()
	StopController() error
	StopReflection(restart bool)
}

// Controller is a concrete implementation of the ApiController interface.
type Controller struct {
	mapper                       namespacesmapping.MapperController
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

// NewAPIController returns a Controller instance for a given set of home and foreign clients.
func NewAPIController(homeClient, foreignClient kubernetes.Interface, informerResyncPeriod time.Duration,
	mapper namespacesmapping.MapperController, opts map[options.OptionKey]options.Option, tepReady chan struct{}) *Controller {
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

// SetInformingFunc configures the handlers triggered for a certain API type by incoming reflection events.
func (c *Controller) SetInformingFunc(api apiReflection.ApiType, handler func(*corev1.Pod)) {
	c.incomingReflectorsController.SetInforming(api, handler)
}

// CacheManager returns the CacheManager associated with the controller.
func (c *Controller) CacheManager() storage.CacheManagerReaderAdder {
	return c.cacheManager
}

// StartController spawns the worker threads and starts the reflection control loops.
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

// StopController stops the controller and the reflection control loops.
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

// StopReflection stops the reflection control loops, optionally restarting them.
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
