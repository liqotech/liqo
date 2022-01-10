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

package natmappinginflater

import (
	"crypto/rand"
	"fmt"
	"math/big"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	liqonetapi "github.com/liqotech/liqo/apis/net/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

// ForgeNatMapping forges a NatMapping resource for a cluster received as parameter.
func ForgeNatMapping(clusterID, podCIDR, externalCIDR string, mappings map[string]string) (*unstructured.Unstructured, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		return nil, err
	}
	natMapping := &liqonetapi.NatMapping{
		TypeMeta: v1.TypeMeta{
			APIVersion: "net.liqo.io/v1alpha1",
			Kind:       "NatMapping",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: fmt.Sprintf("natmapping-%d", n),
			Labels: map[string]string{
				"net.liqo.io/natmapping":     "true",
				liqoconst.ClusterIDLabelName: clusterID,
			},
		},
		Spec: liqonetapi.NatMappingSpec{
			ClusterID:       clusterID,
			PodCIDR:         podCIDR,
			ExternalCIDR:    externalCIDR,
			ClusterMappings: mappings,
		},
	}

	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(natMapping)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: unstructuredObj}, nil
}
