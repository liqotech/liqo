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
	klog.Warningf("local namespace %v translation not found, returning the original namespace", namespace)
	return namespace
}

func (c *Controller) remoteToLocalNamespace(remoteClusterID, namespace string) string {
	if namespace == "" {
		// if the namespaces is empty, the resource is cluster scoped, so we do no need namespace translations
		return namespace
	}

	if ns, ok := c.RemoteToLocalNamespaceMapper[remoteNamespaceKeyer(remoteClusterID, namespace)]; ok {
		return ns
	}
	klog.Warningf("remote namespace %v translation not found, returning the original namespace", namespace)
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

func remoteNamespaceKeyer(remoteClusterID, namespace string) string {
	return fmt.Sprintf("%s/%s", remoteClusterID, namespace)
}
