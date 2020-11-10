package reflectorsInterfaces

import (
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesMapping"
	"github.com/liqotech/liqo/pkg/virtualKubelet/storage"
	"k8s.io/client-go/kubernetes"
)

type ReflectionType int

const (
	OutgoingReflection ReflectionType = iota
	IncomingReflection
)

type APIPreProcessing interface {
	PreProcessIsAllowed(obj interface{}) bool
	PreProcessAdd(obj interface{}) interface{}
	PreProcessUpdate(newObj, oldObj interface{}) interface{}
	PreProcessDelete(obj interface{}) interface{}
}

type APIReflector interface {
	APIPreProcessing

	Inform(obj apimgmt.ApiEvent)
	Keyer(namespace, name string) string

	GetForeignClient() kubernetes.Interface
	GetHomeClient() kubernetes.Interface
	GetCacheManager() storage.CacheManagerReader
	NattingTable() namespacesMapping.NamespaceNatter
	SetupHandlers(api apimgmt.ApiType, reflectionType ReflectionType, namespace, nattedNs string)
	SetPreProcessingHandlers(PreProcessingHandlers)

	SetInforming(handler func(interface{}))
	PushToInforming(interface{})
}

type SpecializedAPIReflector interface {
	SetSpecializedPreProcessingHandlers()
	HandleEvent(interface{})
	CleanupNamespace(namespace string)
}

type OutgoingAPIReflector interface {
	APIReflector
	SpecializedAPIReflector
}

type IncomingAPIReflector interface {
	APIReflector
	SpecializedAPIReflector
}

type PreProcessingHandlers struct {
	IsAllowed  func(obj interface{}) bool
	AddFunc    func(obj interface{}) interface{}
	UpdateFunc func(newObj, oldObj interface{}) interface{}
	DeleteFunc func(obj interface{}) interface{}
}
