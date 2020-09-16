package apiController

import (
	"k8s.io/client-go/tools/cache"
	"sync"
)

type preProcessingUpdateHandler func(newObj, oldObj interface{}) bool

type APIController struct {
	PreProcessing preProcessingUpdateHandler
	Informer      map[string]cache.SharedIndexInformer
	waitGroup     *sync.WaitGroup
	Output        chan interface{}
}

func (c *APIController) Wait() {
	c.waitGroup.Wait()
}