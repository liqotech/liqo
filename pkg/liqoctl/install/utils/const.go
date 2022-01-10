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

package installutils

const (
	// LiqoctlInstallShortHelp contains the short help message for install Liqoctl command.
	LiqoctlInstallShortHelp = "Install Liqo on a selected cluster"
	// LiqoctlInstallLongHelp contains the long help message for install Liqoctl command.
	LiqoctlInstallLongHelp = `Install Liqo on a selected cluster`
	// LiqoctlInstallCommand contains the use command for the Liqo installation command.
	LiqoctlInstallCommand = "install"

	// LiqoNamespace contains the default namespace for Liqo installation.
	LiqoNamespace = "liqo"

	// LiqoChartFullName indicates the name where the Liqo chart can be retrieved.
	LiqoChartFullName = "liqo/liqo"

	// LiqoReleaseName indicates the default release name when installing the Liqo chart.
	LiqoReleaseName = "liqo"
)
