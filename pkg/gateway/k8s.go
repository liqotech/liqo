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

package gateway

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

// SetOwnerReferenceWithMode sets the owner reference of the object according to the mode.
func SetOwnerReferenceWithMode(opts *Options, obj metav1.Object, scheme *runtime.Scheme) error {
	meta := metav1.ObjectMeta{
		Name:      opts.Name,
		Namespace: opts.Namespace,
		UID:       types.UID(opts.GatewayUID),
	}
	switch opts.Mode {
	case ModeServer:
		return controllerutil.SetOwnerReference(&networkingv1beta1.GatewayServer{ObjectMeta: meta}, obj, scheme)
	case ModeClient:
		return controllerutil.SetOwnerReference(&networkingv1beta1.GatewayClient{ObjectMeta: meta}, obj, scheme)
	}
	return fmt.Errorf("invalid mode %v", opts.Mode)
}
