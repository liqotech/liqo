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
//

package common

import (
	"github.com/pterm/pterm"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
)

var statusStyles = map[ModuleStatus]*pterm.Style{
	ModuleHealthy:   pterm.NewStyle(pterm.FgGreen, pterm.Bold),
	ModuleUnhealthy: pterm.NewStyle(pterm.FgRed, pterm.Bold),
	ModuleDisabled:  pterm.NewStyle(pterm.FgLightCyan, pterm.Bold),
}

// CheckModuleStatusAndAlerts returns the status and the alerts of the given module.
func CheckModuleStatusAndAlerts(module liqov1beta1.Module) (status ModuleStatus, alerts []string) {
	status = CheckModuleStatus(module)
	// Collect the alerts if status if module is not healthy
	if status == ModuleUnhealthy {
		alerts = GetModuleAlerts(module)
	}
	return
}

// CheckModuleStatus based on the conditions of a module returns its status.
func CheckModuleStatus(module liqov1beta1.Module) ModuleStatus {
	if module.Enabled {
		for i := range module.Conditions {
			condition := &module.Conditions[i]

			if condition.Status != liqov1beta1.ConditionStatusEstablished && condition.Status != liqov1beta1.ConditionStatusReady {
				return ModuleUnhealthy
			}
		}
		return ModuleHealthy
	}

	return ModuleDisabled
}

// GetModuleAlerts returns the alerts for the given module.
func GetModuleAlerts(module liqov1beta1.Module) []string {
	alerts := []string{}
	for _, condition := range module.Conditions {
		if condition.Status != liqov1beta1.ConditionStatusEstablished && condition.Status != liqov1beta1.ConditionStatusReady {
			alerts = append(alerts, condition.Message)
		}
	}
	return alerts
}

// FormatStatus returns a formatted string with the provided status.
func FormatStatus(moduleStatus ModuleStatus) string {
	style := statusStyles[moduleStatus]
	return style.Sprint(moduleStatus)
}
