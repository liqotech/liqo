package serviceEnv

import (
	goerrors "errors"
	apimgmgt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"github.com/liqotech/liqo/pkg/virtualKubelet/storage"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog"
	v1helper "k8s.io/kubernetes/pkg/apis/core/v1/helper"
	"k8s.io/kubernetes/pkg/kubelet/envvars"
	"k8s.io/utils/net"
	"os"
	"strings"
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

	kubernetesIp, ok := os.LookupEnv("HOME_KUBERNETES_IP")
	if !ok {
		err = goerrors.New("HOME_KUBERNETES_IP env var not set")
		return nil, err
	}
	kubernetesPort, ok := os.LookupEnv("HOME_KUBERNETES_PORT")
	if !ok {
		err = goerrors.New("HOME_KUBERNETES_PORT env var not set")
		return nil, err
	}
	port, err := net.ParsePort(kubernetesPort, false)
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

		if service.Namespace == ns && enableServiceLinks {
			if err = addService(&serviceMap, cacheManager, remoteNs, serviceName, true); err != nil {
				klog.Error(err)
				continue
			}
		}
	}

	// kubernetes service is translated to our local API Server IP+Port
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kubernetes",
		},
		Spec: v1.ServiceSpec{
			ClusterIP: kubernetesIp,
			Ports: []v1.ServicePort{
				{
					Name:       "https",
					Protocol:   "tcp",
					Port:       int32(port),
					TargetPort: intstr.FromInt(port),
				},
			},
			Type: v1.ServiceTypeClusterIP,
		},
	}
	serviceMap[svc.Name] = svc

	mappedServices := make([]*v1.Service, 0, len(serviceMap))
	for key := range serviceMap {
		mappedServices = append(mappedServices, serviceMap[key])
	}

	for _, e := range envvars.FromServices(mappedServices) {
		if strings.Contains(e.Name, strings.Join([]string{"KUBERNETES_PORT", kubernetesPort}, "_")) {
			// this avoids that these labels will be recreated by remote kubelet
			e.Name = strings.Replace(e.Name, kubernetesPort, "443", -1)
		}
		envVars[e.Name] = e.Value
	}
	return envVars, nil
}

func addService(serviceMap *map[string]*v1.Service, cacheManager storage.CacheManagerReader, namespace string, name string, checkNamespace bool) error {
	tmp, err := cacheManager.GetForeignNamespacedObject(apimgmgt.Services, namespace, name)
	if err != nil {
		return err
	}
	if tmp == nil {
		klog.V(4).Infof("nil object for service %v in namespace %v", name, namespace)
		return nil
	}
	remoteSvc := tmp.(*v1.Service)
	// ignore services where ClusterIP is "None" or empty
	if !v1helper.IsServiceIPSet(remoteSvc) {
		return nil
	}

	if _, exists := (*serviceMap)[name]; !exists && (!checkNamespace || remoteSvc.Namespace == namespace) {
		(*serviceMap)[name] = remoteSvc
	}
	return nil
}
