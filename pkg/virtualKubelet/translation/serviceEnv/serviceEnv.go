package serviceEnv

import (
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog"

	"github.com/liqotech/liqo/internal/kubernetes/envvars"
	apimgmgt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"github.com/liqotech/liqo/pkg/virtualKubelet/storage"
)

func TranslateServiceEnvVariables(pod *v1.Pod, localNS string, nattedNS string, cacheManager storage.CacheManagerReader) (*v1.Pod, error) {
	enableServiceLinks := v1.DefaultEnableServiceLinks
	if pod.Spec.EnableServiceLinks != nil {
		enableServiceLinks = *pod.Spec.EnableServiceLinks
	}

	envs, err := getServiceEnvVarMap(localNS, enableServiceLinks, nattedNS, cacheManager)
	if err != nil {
		return nil, err
	}
	for i, container := range pod.Spec.Containers {
		pod.Spec.Containers[i] = *setInContainer(envs, container)
	}
	for i, container := range pod.Spec.InitContainers {
		pod.Spec.InitContainers[i] = *setInContainer(envs, container)
	}
	return pod, nil
}

func setInContainer(envs map[string]string, container v1.Container) *v1.Container {
	for k, v := range envs {
		found := false
		for _, env := range container.Env {
			if env.Name == k {
				found = true
				break
			}
		}
		if !found {
			container.Env = append(container.Env, v1.EnvVar{
				Name:  k,
				Value: v,
			})
		}
	}
	return container.DeepCopy()
}

func getServiceEnvVarMap(ns string, enableServiceLinks bool, remoteNs string, cacheManager storage.CacheManagerReader) (map[string]string, error) {
	var (
		serviceMap = make(map[string]*v1.Service)
		envVars    = make(map[string]string)
	)

	// search for services in the same namespaces of the pod
	services, err := cacheManager.ListHomeNamespacedObject(apimgmgt.Services, ns)
	if err != nil {
		return nil, err
	}

	// project the services in namespace ns onto the master services
	for i := range services {
		// We always want to add environment variables for master kubernetes service
		// from the default namespace, even if enableServiceLinks is false.
		// We also add environment variables for other services in the same
		// namespace, if enableServiceLinks is true.

		tmp := services[i]
		service, ok := tmp.(*v1.Service)
		if !ok {
			klog.V(4).Infof("this object is not a service: %v", tmp)
			continue
		}
		serviceName := service.Name

		// Skipping the default/kubernetes service, as not reflected in the foreign namespace.
		if service.Namespace == v1.NamespaceDefault && service.Name == "kubernetes" && remoteNs != v1.NamespaceDefault {
			continue
		}

		if service.Namespace == ns && enableServiceLinks {
			if err = addService(&serviceMap, cacheManager, remoteNs, serviceName, true); err != nil {
				err := errors.Wrapf(err, "cannot add remote service")
				klog.V(4).Info(err)
				continue
			}
		}
	}

	mappedServices := make([]*v1.Service, 0, len(serviceMap))
	for key := range serviceMap {
		mappedServices = append(mappedServices, serviceMap[key])
	}

	for _, e := range envvars.FromServices(mappedServices) {
		envVars[e.Name] = e.Value
	}
	return envVars, nil
}

func addService(serviceMap *map[string]*v1.Service, cacheManager storage.CacheManagerReader, namespace string,
	name string, checkNamespace bool) error {
	tmp, err := cacheManager.GetForeignNamespacedObject(apimgmgt.Services, namespace, name)
	if err != nil {
		return err
	}
	if tmp == nil {
		klog.V(3).Infof("nil object for service %v in namespace %v", name, namespace)
		return nil
	}
	remoteSvc := tmp.(*v1.Service)
	// ignore services where ClusterIP is "None" or empty
	if !envvars.IsServiceIPSet(remoteSvc) {
		return nil
	}

	if _, exists := (*serviceMap)[name]; !exists && (!checkNamespace || remoteSvc.Namespace == namespace) {
		(*serviceMap)[name] = remoteSvc
	}
	return nil
}
