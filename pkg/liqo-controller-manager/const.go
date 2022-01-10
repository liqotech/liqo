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

package liqocontrollermanager

import (
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

// DeploymentLabelSelector returns the label selector associated with the liqo controller manager deployment/pod.
func DeploymentLabelSelector() labels.Selector {
	// These labels are configured through Helm at install time.
	req1, err := labels.NewRequirement("app.kubernetes.io/name", selection.Equals, []string{"controller-manager"})
	utilruntime.Must(err)
	req2, err := labels.NewRequirement("app.kubernetes.io/component", selection.Equals, []string{"controller-manager"})
	utilruntime.Must(err)

	return labels.NewSelector().Add(*req1, *req2)
}
