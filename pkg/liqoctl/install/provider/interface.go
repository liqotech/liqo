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

package provider

import (
	"context"

	flag "github.com/spf13/pflag"
	"k8s.io/client-go/rest"
)

// InstallProviderInterface defines the methods required to support the Liqo install for a given provider.
type InstallProviderInterface interface {
	// PreValidateGenericCommandArguments validates the flags passed to a generic provider,
	// before the specific provider validation.
	PreValidateGenericCommandArguments(*flag.FlagSet) error
	// ValidateCommandArguments validates the flags passed as arguments to the install command
	ValidateCommandArguments(*flag.FlagSet) error
	// PostValidateGenericCommandArguments validates the flags passed to a generic provider,
	// after the specific provider validation.
	PostValidateGenericCommandArguments(oldClusterName string) error
	// ExtractChartParameters retrieves the install parameters required for a correct installation. This may require
	// instantiating extra clients to interact with cloud provider or the target cluster.
	ExtractChartParameters(context.Context, *rest.Config, *CommonArguments) error
	// UpdateChartValues patches the values map of a selected chart, modifying keys and entries to correctly install Liqo on a given
	// provider
	UpdateChartValues(values map[string]interface{})
}
