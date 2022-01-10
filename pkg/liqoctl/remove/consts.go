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

package remove

const (
	// LiqoctlRemoveShortHelp contains the short help string for liqoctl remove command.
	LiqoctlRemoveShortHelp = "Disable a peering with a remote cluster"
	// LiqoctlRemoveLongHelp contains the Long help string for liqoctl remove command.
	LiqoctlRemoveLongHelp = `Disable a peering with a remote cluster

$ liqoctl remove cluster my-cluster

`
	// UseCommand contains the verb of the remove command.
	UseCommand = "remove"
	// ClusterResourceName contains the name of the resource removed in liqoctl remove.
	ClusterResourceName = "cluster"
	// SuccessfulMessage is printed when a remove cluster command has scucceded.
	SuccessfulMessage = `
	Success 👌! You have correctly removed the cluster %s and disabled an outgoing peering towards it.
You can now:

* Check the status of the peering to see when it is completely disabled. 
The field OutgoingPeering of the foreigncluster should be in "None":

kubectl get foreignclusters %s

* For more information about Liqo have a look to: https://doc.liqo.io
`
)
