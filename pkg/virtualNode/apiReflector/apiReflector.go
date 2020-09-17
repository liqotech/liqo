package apiReflector

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"sync"
)

type PreProcessingHandlers struct {
	addFunc func(obj interface{}) interface{}
	updateFunc func(newObj, oldObj interface{}) interface{}
	deleteFunc func(obj interface{}) interface{}
}

type APIPreProcessing interface {
	PreProcessAdd(obj interface{}) interface{}
	PreProcessUpdate(newObj, oldObj interface{}) interface{}
	PreProcessDelete(obj interface{}) interface{}
}

type APIReflector interface {
	APIPreProcessing

	ReflectNamespace(namespace string, informer cache.SharedIndexInformer)
	Out(interface{})
	Wait()
	done()
}

type SpecializedAPIReflector interface {
	HandleEvent(interface{}) error
}

type GenericAPIReflector struct {
	preProcessingHandlers PreProcessingHandlers
	waitGroup             *sync.WaitGroup
	outputChan	chan interface{}

	client kubernetes.Interface
	informer              map[string]cache.SharedIndexInformer
}

func (c *GenericAPIReflector) PreProcessAdd(obj interface{}) interface{} {
	if c.preProcessingHandlers.updateFunc == nil {
		return obj
	}
	return c.preProcessingHandlers.addFunc(obj)
}

func (c *GenericAPIReflector) PreProcessUpdate(newObj, oldObj interface{}) interface{} {
	if c.preProcessingHandlers.updateFunc == nil {
		return newObj
	}
	return c.preProcessingHandlers.updateFunc(newObj, oldObj)
}

func (c *GenericAPIReflector) PreProcessDelete(obj interface{}) interface{} {
	if c.preProcessingHandlers.deleteFunc == nil {
		return obj
	}
	return c.preProcessingHandlers.deleteFunc(obj)
}

func (c *GenericAPIReflector) ReflectNamespace(namespace string, informer cache.SharedIndexInformer) {
	c.informer[namespace] = informer
}

func (c *GenericAPIReflector) Wait() {
	c.waitGroup.Wait()
}

func (c *GenericAPIReflector) Out(obj interface{}) {
	c.outputChan <- obj
}

func (c *GenericAPIReflector) done() {
	c.waitGroup.Done()
}

