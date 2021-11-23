// Copyright 2019-2022 The Liqo Authors
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

package foreignclusteroperator

import (
	"context"

	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/discoverymanager/utils"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

// check if the ForeignCluster CR does not have a value in one of the required fields (Namespace and ClusterID)
// and needs a value defaulting.
func (r *ForeignClusterReconciler) needsClusterIdentityDefaulting(fc *v1alpha1.ForeignCluster) bool {
	return fc.Spec.ClusterIdentity.ClusterID == ""
}

// clusterIdentityDefaulting loads the default values for that ForeignCluster basing on the AuthUrl value, an HTTP request
// is sent and the retrieved values are applied for the following fields (if they are empty):
// Cluster.ClusterID, Cluster.ClusterName.
func (r *ForeignClusterReconciler) clusterIdentityDefaulting(ctx context.Context, fc *v1alpha1.ForeignCluster) error {
	klog.V(4).Infof("Defaulting Cluster values for ForeignCluster %v", fc.Name)
	ids, err := utils.GetClusterInfo(ctx, r.transport(foreignclusterutils.InsecureSkipTLSVerify(fc)), fc.Spec.ForeignAuthURL)
	if err != nil {
		klog.Error(err)
		return err
	}

	if fc.Spec.ClusterIdentity.ClusterID == "" {
		fc.Spec.ClusterIdentity.ClusterID = ids.ClusterID
	}
	if fc.Spec.ClusterIdentity.ClusterName == "" {
		fc.Spec.ClusterIdentity.ClusterName = ids.ClusterName
	}

	klog.V(4).Infof("New values:\n\tClusterId:\t%v\n\tClusterName:\t%v",
		fc.Spec.ClusterIdentity.ClusterID,
		fc.Spec.ClusterIdentity.ClusterName)
	return nil
}
