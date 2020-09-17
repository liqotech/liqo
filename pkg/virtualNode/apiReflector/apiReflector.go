package apiReflector

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"reflect"
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

	Wait()
	HandleEvent(interface{}) error

	done()
}

type GenericAPIReflector struct {
	preProcessingHandlers PreProcessingHandlers
	Informer              map[string]cache.SharedIndexInformer
	waitGroup             *sync.WaitGroup
	Output                chan interface{}

	client kubernetes.Interface
}

func NewApiReflector(api int) APIReflector {
	return reflect.New(apiTypes[api]).Interface().(APIReflector)
}

func (c *GenericAPIReflector) Wait() {
	c.waitGroup.Wait()
}

func (c *GenericAPIReflector) done() {
	c.waitGroup.Done()
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