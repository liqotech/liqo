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

package add

const (
	// LiqoctlAddShortHelp contains the short help string for liqoctl add command.
	LiqoctlAddShortHelp = "Enable a peering with a remote cluster"
	// LiqoctlAddLongHelp contains the Long help string for liqoctl add command.
	LiqoctlAddLongHelp = `Enable a peering with a remote cluster

$ liqoctl add cluster my-cluster --auth-url https://my-cluster --id e8e3cdec-b007-48ad-b2d5-64a8f03dc5f4 --token 525972c1d0a791...

`
	// AuthURLFlagName contains the name of auth-url flag.
	AuthURLFlagName = "auth-url"
	// ClusterNameFlagName contains the name of cluster name flag.
	ClusterNameFlagName = "name"
	// ClusterIDFlagName contains the name of cluster-id flag.
	ClusterIDFlagName = "id"
	// ClusterTokenFlagName contains the name for the token flag.
	ClusterTokenFlagName = "token"
	// UseCommand contains the verb of the add command.
	UseCommand = "add"
	// ClusterResourceName contains the name of the resource added in liqoctl add.
	ClusterResourceName = "cluster"
	// ClusterLiqoNamespaceFlagName contains the default namespace where Liqo is installed.
	ClusterLiqoNamespaceFlagName = "namespace"
	// ClusterLiqoNamespace contains the default namespace where Liqo is installed.
	ClusterLiqoNamespace = "liqo"
	sameClusterError     = "the ClusterID of the adding cluster is the same of the local cluster"
	// SuccessfulMessage is printed when ad add cluster command has succeeded.
	SuccessfulMessage = `
Hooray 🎉! You have correctly added the cluster %s and activated an outgoing peering towards it.
You can now:

* Check the status of the peering to see when it is completely established 👓.
Every field of the foreigncluster (but IncomingPeering) should be in "Established":

kubectl get foreignclusters %s

* Check if the virtual node is correctly created (this should take less than ~30s) 📦:

kubectl get nodes liqo-%s

* Ready to go! Let's deploy a simple cross-cluster application using Liqo 🚜:

kubectl create ns liqo-demo # Let's create a demo namespace
kubectl label ns liqo-demo liqo.io/enabled=true # Enable Liqo offloading on this namespace (Check out https://doc.liqo.io/usage for more details).
kubectl apply -n liqo-demo -f https://get.liqo.io/app.yaml # Deploy a sample application in the namespace to trigger the offloading.

* For more information about Liqo have a look to: https://doc.liqo.io
`
)
