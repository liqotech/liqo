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

package uninstall

import (
	"context"
	"fmt"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	offloadingv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	"github.com/liqotech/liqo/pkg/utils/errors"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

type errorMap struct {
	outgoingPeering []string
	incomingPeering []string
	networking      []string
	offloading      []string
}

func newErrorMap() errorMap {
	return errorMap{
		outgoingPeering: []string{},
		incomingPeering: []string{},
		networking:      []string{},
		offloading:      []string{},
	}
}

func (em *errorMap) getError() error {
	str := ""
	hasErr := false
	if len(em.outgoingPeering) > 0 {
		str += "\ndisable outgoing peering for clusters:\n"
		for _, fc := range em.outgoingPeering {
			str += fmt.Sprintf("- %s\n", fc)
		}
		hasErr = true
	}
	if len(em.incomingPeering) > 0 {
		str += "\ndisable incoming peering for clusters:\n"
		for _, fc := range em.incomingPeering {
			str += fmt.Sprintf("- %s\n", fc)
		}
		hasErr = true
	}
	if len(em.networking) > 0 {
		str += "\ndisable networking for clusters:\n"
		for _, fc := range em.networking {
			str += fmt.Sprintf("- %s\n", fc)
		}
		hasErr = true
	}
	if len(em.offloading) > 0 {
		str += "\ndisable offloading for namespaces:\n"
		for _, fc := range em.offloading {
			str += fmt.Sprintf("- %s\n", fc)
		}
		hasErr = true
	}

	if hasErr {
		return fmt.Errorf("you should:\n%s", str)
	}
	return nil
}

func (o *Options) preUninstall(ctx context.Context) error {
	var foreignClusterList discoveryv1alpha1.ForeignClusterList
	if err := o.CRClient.List(ctx, &foreignClusterList); errors.IgnoreNoMatchError(err) != nil {
		return err
	}

	errMap := newErrorMap()
	for i := range foreignClusterList.Items {
		fc := &foreignClusterList.Items[i]

		if foreignclusterutils.IsOutgoingEnabled(fc) {
			errMap.outgoingPeering = append(errMap.outgoingPeering, fc.Name)
		}
		if foreignclusterutils.IsIncomingEnabled(fc) {
			errMap.incomingPeering = append(errMap.incomingPeering, fc.Name)
		}
		if foreignclusterutils.IsNetworkingEstablished(fc) {
			errMap.networking = append(errMap.networking, fc.Name)
		}
	}

	var namespaceOffloadings offloadingv1alpha1.NamespaceOffloadingList
	if err := o.CRClient.List(ctx, &namespaceOffloadings); errors.IgnoreNoMatchError(err) != nil {
		return err
	}

	for i := range namespaceOffloadings.Items {
		offloading := &namespaceOffloadings.Items[i]
		errMap.offloading = append(errMap.offloading, offloading.Namespace)
	}

	return errMap.getError()
}
