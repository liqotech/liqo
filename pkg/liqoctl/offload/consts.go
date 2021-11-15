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

package offload

const (
	// LiqoctlOffloadShortHelp contains the short help string for liqoctl Offload command.
	LiqoctlOffloadShortHelp = "Offload a namespace to remote clusters"
	// LiqoctlOffloadLongHelp contains the Long help string for liqoctl Offload command.
	LiqoctlOffloadLongHelp = `Offload a namespace to remote clusters with the default values:
$ liqoctl offload namespace liqo-demo

To just enable service reflection, offload a namespace with the EnforceSameName namespace mapping strategy and
and a Local --pod-offloading-strategy.

$ liqoctl offload namespace liqo-demo --namespace-mapping-strategy=EnforceSameName --pod-offloading-strategy=Local
`
	// UseCommand contains the verb of the Offload command.
	UseCommand = "offload"
	// ClusterResourceName contains the name of the resource offloaded in liqoctl Offload.
	ClusterResourceName = "namespace"
	// SuccessfulMessage is printed when a Offload cluster command has scucceded.
	SuccessfulMessage = `
Success 👌! The offloading rules for the namespace %s has been created/updated on the cluster! 🚀
Check them out by typing: 
kubectl get namespaceoffloading -n %s %s
`
	// PodOffloadingStrategyFlag specifies the pod offloading strategy flag name.
	PodOffloadingStrategyFlag = "pod-offloading-strategy"
	// PodOffloadingStrategyHelp specifies the help message for the PodOffloadingStrategy flag.
	PodOffloadingStrategyHelp = "Select the desired pod offloading strategy (Local, LocalAndRemote, Remote) "

	// NamespaceMappingStrategyFlag specifies the namespace mapping flag name.
	NamespaceMappingStrategyFlag = "namespace-mapping-strategy"
	// NamespaceMappingStrategyHelp specifies the help message for the NamespaceMappingStrategy flag.
	NamespaceMappingStrategyHelp = "Select the desired namespace mapping strategy (EnforceSameName, DefaultName) "

	// AcceptedLabelsFlag specifies the accepted labels flag name.
	AcceptedLabelsFlag = "accepted-cluster-labels"
	// AcceptedLabelsDefault specifies the accepted default labels used to forge the clusterSelector field.
	AcceptedLabelsDefault = "liqo.io/type=virtual-node"
	// AcceptedLabelsHelp specifies the help message for the AcceptedLabels flag.
	AcceptedLabelsHelp = "The set of labels accepted as valid clusterLabels for the clusterSelector field"
	// DeniedLabelsFlag specifies the accepted labels flag name.
	DeniedLabelsFlag = "denied-cluster-labels"
	// DeniedLabelDefault specifies the denied default labels used to forge the clusterSelector field.
	DeniedLabelDefault = ""
	// DeniedLabelsHelp specifies the help message for the DeniedLabels flag.
	DeniedLabelsHelp = "The set of labels identified as forbidden clusterLabels for the clusterSelector field"
)
