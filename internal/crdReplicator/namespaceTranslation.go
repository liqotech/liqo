package crdreplicator

import (
	"fmt"

	"k8s.io/klog/v2"
)

func (c *Controller) localToRemoteNamespace(namespace string) string {
	if namespace == "" {
		// if the namespaces is empty, the resource is cluster scoped, so we do no need namespace translations
		return namespace
	}

	if ns, ok := c.LocalToRemoteNamespaceMapper[namespace]; ok {
		return ns
	}
	klog.V(5).Infof("local namespace %v translation not found, returning the original namespace", namespace)
	return namespace
}

func (c *Controller) remoteToLocalNamespace(namespace string) string {
	if namespace == "" {
		// if the namespaces is empty, the resource is cluster scoped, so we do no need namespace translations
		return namespace
	}

	if ns, ok := c.RemoteToLocalNamespaceMapper[namespace]; ok {
		return ns
	}
	klog.V(5).Infof("remote namespace %v translation not found, returning the original namespace", namespace)
	return namespace
}

func (c *Controller) clusterIDToRemoteNamespace(clusterID string) (string, error) {
	if ns, ok := c.ClusterIDToRemoteNamespaceMapper[clusterID]; ok {
		return ns, nil
	}
	err := fmt.Errorf("clusterID %v translation not found", clusterID)
	klog.Error(err)
	return "", err
}

func (c *Controller) clusterIDToLocalNamespace(clusterID string) (string, error) {
	if ns, ok := c.ClusterIDToLocalNamespaceMapper[clusterID]; ok {
		return ns, nil
	}
	err := fmt.Errorf("clusterID %v translation not found", clusterID)
	klog.Error(err)
	return "", err
}
