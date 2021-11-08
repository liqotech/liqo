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

package generate

// LiqoctlGenerateShortHelp contains the short help message for the generate-add-cluster Liqoctl command.
const LiqoctlGenerateShortHelp = "Generate the command to execute on another cluster to peer with the selected cluster."

// LiqoctlGenerateLongHelp contains the short help message for the generate-add-cluster Liqoctl command.
const LiqoctlGenerateLongHelp = `Generate the command to execute on another cluster to peer with the selected cluster.

liqoctl generate-add-command command requires Liqo to be installed on the target cluster.
`

// LiqoctlGenerateAddCommand contains the use command for the generate-add-cluster Liqoctl command.
const LiqoctlGenerateAddCommand = "generate-add-command"
