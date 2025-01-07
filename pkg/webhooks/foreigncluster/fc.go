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

package fcwh

import (
	"context"
	"encoding/json"
	"net/http"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

type fcwh struct {
	decoder admission.Decoder
}

type fcwhm struct {
	fcwh
}

// NewMutator returns a new ForeignCluster mutating webhook.
func NewMutator() *webhook.Admission {
	return &webhook.Admission{Handler: &fcwhm{
		fcwh: fcwh{
			decoder: admission.NewDecoder(runtime.NewScheme()),
		},
	}}
}

// DecodeForeignCluster decodes the ForeignCluster from the incoming request.
func (w *fcwh) DecodeForeignCluster(obj runtime.RawExtension) (*liqov1beta1.ForeignCluster, error) {
	var fc liqov1beta1.ForeignCluster
	err := w.decoder.DecodeRaw(obj, &fc)
	return &fc, err
}

// Handle implements the ForeignCluster mutating webhook logic.
//
//nolint:gocritic // The signature of this method is imposed by controller runtime.
func (w *fcwhm) Handle(_ context.Context, req admission.Request) admission.Response {
	fc, err := w.DecodeForeignCluster(req.Object)
	if err != nil {
		klog.Errorf("Failed decoding ForeignCluster object: %v", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Enforce the ClusterID label, to allow retrieving the foreign cluster by ID.
	if fc.Spec.ClusterID != "" {
		if fc.ObjectMeta.Labels == nil {
			fc.ObjectMeta.Labels = map[string]string{}
		}
		fc.ObjectMeta.Labels[consts.RemoteClusterID] = string(fc.Spec.ClusterID)
	}

	marshaledFc, err := json.Marshal(fc)
	if err != nil {
		klog.Errorf("Failed marshaling ForeignCluster object: %v", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledFc)
}
