// Copyright 2019-2021 The Liqo Authors
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

package forge

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	// LiqoOutgoingKey is a label to set on all offloaded resources.
	LiqoOutgoingKey = "virtualkubelet.liqo.io/outgoing"
	// LiqoOriginClusterID is a label to set on all offloaded resources to identify the origin cluster.
	LiqoOriginClusterID = "virtualkubelet.liqo.io/originClusterId"
	// LiqoIncomingKey is a label for incoming resources.
	LiqoIncomingKey = "virtualkubelet.liqo.io/incoming"
)

var (
	LiqoNodeName = func() string {
		if forger.virtualNodeName == nil {
			return ""
		}
		return forger.virtualNodeName.Value().ToString()
	}
)

func (f *apiForger) forgeForeignMeta(homeMeta, foreignMeta *metav1.ObjectMeta, foreignNamespace, reflectionType string) {
	forgeObjectMeta(homeMeta, foreignMeta)

	foreignMeta.Namespace = foreignNamespace
	foreignMeta.Labels[LiqoOriginClusterID] = f.offloadClusterID.Value().ToString()
	foreignMeta.Labels[reflectionType] = LiqoNodeName()
}

func (f *apiForger) forgeHomeMeta(foreignMeta, homeMeta *metav1.ObjectMeta, homeNamespace, reflectionType string) {
	forgeObjectMeta(foreignMeta, homeMeta)

	homeMeta.Namespace = homeNamespace
	homeMeta.Labels[reflectionType] = LiqoNodeName()
}

func forgeObjectMeta(inMeta, outMeta *metav1.ObjectMeta) {
	outMeta.Name = inMeta.Name

	if outMeta.Annotations == nil {
		outMeta.Annotations = make(map[string]string)
	}
	for k, v := range inMeta.Annotations {
		outMeta.Annotations[k] = v
	}

	if outMeta.Labels == nil {
		outMeta.Labels = make(map[string]string)
	}
	for k, v := range inMeta.Labels {
		outMeta.Labels[k] = v
	}
}
