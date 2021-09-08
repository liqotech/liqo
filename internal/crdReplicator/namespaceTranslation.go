// Copyright 2019-2021 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
