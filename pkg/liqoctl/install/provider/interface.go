package provider

import (
	"context"

	flag "github.com/spf13/pflag"
	"k8s.io/client-go/rest"
)

// InstallProviderInterface defines the methods required to support the Liqo install for a given provider.
type InstallProviderInterface interface {
	// ValidateCommandArguments validates the flags passed as arguments to the install command
	ValidateCommandArguments(*flag.FlagSet) error
	// ExtractChartParameters retrieves the install parameters required for a correct installation. This may require
	// instantiating extra clients to interact with cloud provider or the target cluster.
	ExtractChartParameters(context.Context, *rest.Config) error
	// UpdateChartValues patches the values map of a selected chart, modifying keys and entries to correctly install Liqo on a given
	// provider
	UpdateChartValues(values map[string]interface{})
}
