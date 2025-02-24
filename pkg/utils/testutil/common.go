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

package testutil

import (
	"regexp"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

// SqueezeWhitespaces squeezes a string replacing multiple whitespaces with a single one.
func SqueezeWhitespaces(s string) string {
	whitespaces := regexp.MustCompile(`\s+`)
	return whitespaces.ReplaceAllString(s, " ")
}

// FakeNamespaceWithClusterID generate a fake tenant namespace object.
func FakeNamespaceWithClusterID(clusterID liqov1beta1.ClusterID, namespace string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
			Labels: map[string]string{
				consts.RemoteClusterID:      string(clusterID),
				consts.TenantNamespaceLabel: "true",
			},
		},
	}
}
