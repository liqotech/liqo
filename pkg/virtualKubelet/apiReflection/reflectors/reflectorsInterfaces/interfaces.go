package reflectorsInterfaces

import (
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesMapping"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
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
	LocalInformer(string) cache.SharedIndexInformer
	ForeignInformer(string) cache.SharedIndexInformer
	GetForeignClient() kubernetes.Interface
	NattingTable() namespacesMapping.NamespaceNatter
	SetInformers(reflectionType ReflectionType, namespace, nattedNs string, homeInformer, foreignInformer cache.SharedIndexInformer)
	SetPreProcessingHandlers(PreProcessingHandlers)
	SetInforming(handler func(interface{}))
	PushToInforming(interface{})
}

type SpecializedAPIReflector interface {
	SetSpecializedPreProcessingHandlers()
	HandleEvent(interface{})
	KeyerFromObj(obj interface{}, remoteNamespace string) string
	CleanupNamespace(namespace string)
}

type OutgoingAPIReflector interface {
	APIReflector
	SpecializedAPIReflector
}

type IncomingAPIReflector interface {
	APIReflector
	SpecializedAPIReflector

	GetMirroredObject(namespace, name string) interface{}
	ListMirroredObjects(namespace string) []interface{}
}

type PreProcessingHandlers struct {
	IsAllowed  func(obj interface{}) bool
	AddFunc    func(obj interface{}) interface{}
	UpdateFunc func(newObj, oldObj interface{}) interface{}
	DeleteFunc func(obj interface{}) interface{}
}
