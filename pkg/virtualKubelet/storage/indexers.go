package storage

import (
	"errors"
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	corev1 "k8s.io/api/core/v1"
	discoveryv1beta1 "k8s.io/api/discovery/v1beta1"
	"k8s.io/client-go/tools/cache"
	"strings"
)

var InformerIndexers = map[apimgmt.ApiType]func() cache.Indexers{
	apimgmt.Configmaps:         configmapsIndexers,
	apimgmt.EndpointSlices:     endpointSlicesIndexers,
	apimgmt.Pods:               podsIndexers,
	apimgmt.ReplicaControllers: replicaControllerIndexers,
	apimgmt.Secrets:            secretsIndexers,
	apimgmt.Services:           servicesIndexers,
}

func configmapsIndexers() cache.Indexers {
	i := cache.Indexers{}
	i["configmaps"] = func(obj interface{}) ([]string, error) {
		cm, ok := obj.(*corev1.ConfigMap)
		if !ok {
			return []string{}, errors.New("cannot convert obj to configmap")
		}
		return []string{
			strings.Join([]string{cm.Namespace, cm.Name}, "/"),
		}, nil
	}
	return i
}

func endpointSlicesIndexers() cache.Indexers {
	i := cache.Indexers{}
	i["endpointslices"] = func(obj interface{}) ([]string, error) {
		endpointSlice, ok := obj.(*discoveryv1beta1.EndpointSlice)
		if !ok {
			return []string{}, errors.New("cannot convert obj to endpointslice")
		}
		return []string{
			strings.Join([]string{endpointSlice.Namespace, endpointSlice.Name}, "/"),
		}, nil
	}
	return i
}

func podsIndexers() cache.Indexers {
	i := cache.Indexers{}
	i["pods"] = func(obj interface{}) ([]string, error) {
		po, ok := obj.(*corev1.Pod)
		if !ok {
			return []string{}, errors.New("cannot convert obj to pod")
		}
		return []string{
			strings.Join([]string{po.Namespace, po.Name}, "/"),
			po.Name,
		}, nil
	}
	return i
}

func replicaControllerIndexers() cache.Indexers {
	i := cache.Indexers{}
	i["replicationcontrollers"] = func(obj interface{}) ([]string, error) {
		rc, ok := obj.(*corev1.ReplicationController)
		if !ok {
			return []string{}, errors.New("cannot convert obj to replicationController")
		}
		return []string{
			strings.Join([]string{rc.Namespace, rc.Name}, "/"),
			rc.Name,
		}, nil
	}
	return i
}

func secretsIndexers() cache.Indexers {
	i := cache.Indexers{}
	i["secrets"] = func(obj interface{}) ([]string, error) {
		secret, ok := obj.(*corev1.Secret)
		if !ok {
			return []string{}, errors.New("cannot convert obj to secret")
		}
		return []string{
			strings.Join([]string{secret.Namespace, secret.Name}, "/"),
		}, nil
	}
	return i
}

func servicesIndexers() cache.Indexers {
	i := cache.Indexers{}
	i["services"] = func(obj interface{}) ([]string, error) {
		svc, ok := obj.(*corev1.Service)
		if !ok {
			return []string{}, errors.New("cannot convert obj to service")
		}
		return []string{
			strings.Join([]string{svc.Namespace, svc.Name}, "/"),
		}, nil
	}
	return i
}
