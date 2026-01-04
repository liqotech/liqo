// Copyright 2019-2026 The Liqo Authors
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

package telemetry

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/consts"
)

func createClusterIDTelemetryConfigMap(ctx context.Context, cl client.Client, namespace string) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      consts.ClusterIDTelemetryConfigMapName,
			Namespace: namespace,
			Labels: map[string]string{
				consts.K8sAppNameKey: consts.ClusterIDTelemetryConfigMapNameLabelValue,
			},
		},
		Data: map[string]string{
			consts.ClusterIDTelemetryConfigMapKey: generateClusterIDTelemetry(),
		},
	}

	if err := cl.Create(ctx, cm); err != nil {
		return fmt.Errorf("failed to create configmap: %w", err)
	}
	return nil
}
