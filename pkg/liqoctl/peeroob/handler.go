// Copyright 2019-2023 The Liqo Authors
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

package peeroob

import (
	"context"
	"fmt"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/peer"
	"github.com/liqotech/liqo/pkg/utils"
	authenticationtokenutils "github.com/liqotech/liqo/pkg/utils/authenticationtoken"
	foreigncluster "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

// Options encapsulates the arguments of the peer out-of-band command.
type Options struct {
	*peer.Options

	ClusterToken   string
	ClusterAuthURL string
	ClusterID      string
}

// Run implements the peer out-of-band command.
func (o *Options) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	s := o.Printer.StartSpinner("Processing cluster peering")

	fc, err := o.peer(ctx)
	if err != nil {
		s.Fail("Failed peering clusters: ", output.PrettyErr(err))
		return err
	}
	s.Success("Peering enabled")

	if err := o.Wait(ctx, &fc.Spec.ClusterIdentity); err != nil {
		return err
	}

	o.Printer.Success.Println("Peering successfully established")
	return nil
}

func (o *Options) peer(ctx context.Context) (*discoveryv1alpha1.ForeignCluster, error) {
	// Retrieve the cluster identity associated with the current cluster.
	clusterIdentity, err := utils.GetClusterIdentityWithControllerClient(ctx, o.CRClient, o.LiqoNamespace)
	if err != nil {
		return nil, err
	}

	// Check whether cluster IDs are the same, as we cannot peer with ourselves.
	if clusterIdentity.ClusterID == o.ClusterID {
		return nil, fmt.Errorf("the Cluster ID of the remote cluster is the same of that of the local cluster")
	}

	// Create the secret containing the authentication token.
	err = authenticationtokenutils.StoreInSecret(ctx, o.KubeClient, o.ClusterID, o.ClusterToken, o.LiqoNamespace)
	if err != nil {
		return nil, err
	}

	// Enforce the presence of the ForeignCluster resource.
	return o.enforceForeignCluster(ctx)
}

func (o *Options) enforceForeignCluster(ctx context.Context) (*discoveryv1alpha1.ForeignCluster, error) {
	fc, err := foreigncluster.GetForeignClusterByID(ctx, o.CRClient, o.ClusterID)
	if kerrors.IsNotFound(err) {
		fc = &discoveryv1alpha1.ForeignCluster{ObjectMeta: metav1.ObjectMeta{Name: o.ClusterName,
			Labels: map[string]string{discovery.ClusterIDLabel: o.ClusterID}}}
	} else if err != nil {
		return nil, err
	}

	_, err = controllerutil.CreateOrUpdate(ctx, o.CRClient, fc, func() error {
		if fc.Spec.PeeringType != discoveryv1alpha1.PeeringTypeUnknown && fc.Spec.PeeringType != discoveryv1alpha1.PeeringTypeOutOfBand {
			return fmt.Errorf("a peering of type %s already exists towards remote cluster %q, cannot be changed to %s",
				fc.Spec.PeeringType, o.ClusterName, discoveryv1alpha1.PeeringTypeOutOfBand)
		}

		fc.Spec.PeeringType = discoveryv1alpha1.PeeringTypeOutOfBand
		fc.Spec.ClusterIdentity.ClusterID = o.ClusterID
		if fc.Spec.ClusterIdentity.ClusterName == "" {
			fc.Spec.ClusterIdentity.ClusterName = o.ClusterName
		}

		fc.Spec.ForeignAuthURL = o.ClusterAuthURL
		fc.Spec.ForeignProxyURL = ""
		fc.Spec.OutgoingPeeringEnabled = discoveryv1alpha1.PeeringEnabledYes
		if fc.Spec.IncomingPeeringEnabled == "" {
			fc.Spec.IncomingPeeringEnabled = discoveryv1alpha1.PeeringEnabledAuto
		}
		if fc.Spec.InsecureSkipTLSVerify == nil {
			fc.Spec.InsecureSkipTLSVerify = pointer.BoolPtr(true)
		}
		return nil
	})

	return fc, err
}
