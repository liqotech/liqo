// Copyright 2019-2025 The Liqo Authors
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

package v1beta1

// StorageType defines the type of storage offered by a resource offer.
type StorageType struct {
	// StorageClassName indicates the name of the storage class.
	StorageClassName string `json:"storageClassName"`
	// Default indicates whether this storage class is the default storage class for Liqo.
	Default bool `json:"default,omitempty"`
}

// IngressType defines the type of ingress offered by a resource offer.
type IngressType struct {
	// IngressClassName indicates the name of the ingress class.
	IngressClassName string `json:"ingressClassName"`
	// Default indicates whether this ingress class is the default ingress class for Liqo.
	Default bool `json:"default,omitempty"`
}

// LoadBalancerType defines the type of load balancer offered by a resource offer.
type LoadBalancerType struct {
	// LoadBalancerClassName indicates the name of the load balancer class.
	LoadBalancerClassName string `json:"loadBalancerClassName"`
	// Default indicates whether this load balancer class is the default load balancer class for Liqo.
	Default bool `json:"default,omitempty"`
}
