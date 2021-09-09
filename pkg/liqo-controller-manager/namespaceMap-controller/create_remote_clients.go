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

package namespacemapctrl

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/clusterid"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
)

// checkRemoteClientPresence creates a new controller-runtime Client for a remote cluster, if it isn't already present
// in RemoteClients Struct of NamespaceMap Controller.
func (r *NamespaceMapReconciler) checkRemoteClientPresence(remoteClusterID string) error {
	if r.RemoteClients == nil {
		r.RemoteClients = map[string]kubernetes.Interface{}
	}

	if _, ok := r.RemoteClients[remoteClusterID]; !ok {
		clusterID := clusterid.NewStaticClusterID(r.LocalClusterID)
		tenantNamespaceManager := tenantnamespace.NewTenantNamespaceManager(r.IdentityManagerClient)
		identityManager := identitymanager.NewCertificateIdentityReader(r.IdentityManagerClient, clusterID, tenantNamespaceManager)
		restConfig, err := identityManager.GetConfig(remoteClusterID, "")
		if err != nil {
			klog.Error(err)
			return err
		}

		if r.RemoteClients[remoteClusterID], err = kubernetes.NewForConfig(restConfig); err != nil {
			klog.Errorf("%s -> unable to create client for cluster '%s'", err, remoteClusterID)
			return err
		}
	}
	return nil
}
