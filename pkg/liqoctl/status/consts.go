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

package status

const (

	// ShortHelp contains the short help string for liqoctl status command.
	ShortHelp = "Show overall status of Liqo"
	// LongHelp contains the Long help string for liqoctl status command.
	LongHelp = `
Show overall status of Liqo.

The command shows the status of the Liqo control plane. The command checks that
every component is up and running.

$ liqoctl status --namespace ns-where-Liqo-is-running
`
	// UseCommand contains the name of the command.
	UseCommand = "status"

	// Namespace contains the name of namespace flag.
	Namespace = "namespace"

	// ClusterNameFilter contains the name of the cluster name filter flag.
	ClusterNameFilter = "cluster-name"

	// ClusterIDFilter contains the name of the cluster ID filter flag.
	ClusterIDFilter = "cluster-id"

	// ShowOnlyLocal contains the name of the show only local flag.
	ShowOnlyLocal = "show-only-local"

	redCross  = "\u274c"
	checkMark = "\u2714"

	reset = "\033[0m"

	red     = "\033[0;31m"
	green   = "\033[0;32m"
	yellow  = "\033[0;33m"
	byellow = "\033[1;33m"
	bpurple = "\033[1;35m"
	bcyan   = "\033[1;36m"
)
