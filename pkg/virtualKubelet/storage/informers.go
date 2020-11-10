package storage

import (
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

var InformerBuilders = map[apimgmt.ApiType]func(informers.SharedInformerFactory) cache.SharedIndexInformer{
	apimgmt.Configmaps:         configmapsInformerBuilder,
	apimgmt.EndpointSlices:     endpointSlicesInformerBuilder,
	apimgmt.Pods:               podsInformerBuilder,
	apimgmt.ReplicaControllers: replicaControllersInformerBuilder,
	apimgmt.Services:           servicesInformerBuilder,
	apimgmt.Secrets:            secretsInformerBuilder,
}

func configmapsInformerBuilder(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
	return factory.Core().V1().ConfigMaps().Informer()
}

func endpointSlicesInformerBuilder(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
	return factory.Discovery().V1beta1().EndpointSlices().Informer()
}

func podsInformerBuilder(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
	return factory.Core().V1().Pods().Informer()
}

func replicaControllersInformerBuilder(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
	return factory.Core().V1().ReplicationControllers().Informer()
}

func servicesInformerBuilder(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
	return factory.Core().V1().Services().Informer()
}

func secretsInformerBuilder(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
	return factory.Core().V1().Secrets().Informer()
}
