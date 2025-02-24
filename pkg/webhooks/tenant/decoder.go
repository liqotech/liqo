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
//

package tenant

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
)

// NameExtractor extracts the Tenant name from the object.
func NameExtractor(obj client.Object) []string {
	tenant := obj.(*authv1beta1.Tenant)
	return []string{tenant.Name}
}

type tenantDecoder struct {
	decoder admission.Decoder
}

// DecodeTenant decodes the Tenant from the incoming request.
func (w *tenantDecoder) DecodeTenant(obj runtime.RawExtension) (*authv1beta1.Tenant, error) {
	var tenant authv1beta1.Tenant
	err := w.decoder.DecodeRaw(obj, &tenant)
	return &tenant, err
}
