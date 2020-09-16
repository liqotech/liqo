package kubernetes

import (
	"k8s.io/client-go/tools/cache"
	"sync"
)

type preProcessingUpdateHandler func(newObj, oldObj interface{}) bool

type APIController struct {
	preProcessing preProcessingUpdateHandler
	informer map[string]cache.SharedIndexInformer
	waitGroup *sync.WaitGroup
	output chan interface{}
}

func (c *APIController) Wait() {
	c.waitGroup.Wait()
}