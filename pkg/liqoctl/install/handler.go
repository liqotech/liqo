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

package install

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	helm "github.com/mittwald/go-helm-client"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
	"helm.sh/helm/v3/pkg/strvals"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/install/util"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/utils"
)

// Provider defines the interface for an install provider.
type Provider interface {
	// Name returns the name of the given provider.
	Name() string
	// Examples returns the examples string for the given provider.
	Examples() string

	// RegisterFlags registers the flags for the given provider.
	RegisterFlags(cmd *cobra.Command)

	// Initialize performs the initialization tasks to retrieve the provider-specific parameters.
	Initialize(ctx context.Context) error
	// Values returns the customized provider-specifc values file parameters.
	Values() map[string]interface{}
}

// Options encapsulates the arguments of the install command.
type Options struct {
	*CommonOptions

	APIServer   string
	PodCIDR     string
	ServiceCIDR string
}

// CommonOptions encapsulates common arguments (not modified by providers) of the install command.
type CommonOptions struct {
	*factory.Factory
	CommandName string

	Version   string
	RepoURL   string
	ChartPath string

	OverrideValues       []string
	OverrideStringValues []string
	OverrideValuesFiles  []string
	chartValues          map[string]interface{}
	tmpDir               string

	DryRun           bool
	OnlyOutputValues bool
	ValuesPath       string

	Timeout time.Duration

	ClusterID     liqov1beta1.ClusterID
	ClusterLabels map[string]string

	EnableMetrics    bool
	DisableTelemetry bool

	ReservedSubnets []string

	DisableAPIServerSanityChecks bool
	DisableAPIServerDefaulting   bool
	SkipValidation               bool

	DisableKernelVersionCheck bool
}

// NewOptions returns a new Options struct.
func NewOptions(f *factory.Factory, commandName string) *Options {
	return &Options{
		CommonOptions: &CommonOptions{
			CommandName: commandName,
			Factory:     f,
		},
	}
}

// Run implements the install command.
func (o *Options) Run(ctx context.Context, provider Provider) error {
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	// Defer the function which performs the cleanup of the temporary directory,
	// in case it is created to download a custom version of the Liqo git repository.
	defer func() { o.Printer.CheckErr(o.cleanup()) }()

	s := o.Printer.StartSpinner("Initializing installer")
	if err := o.initialize(ctx, provider); err != nil {
		s.Fail("Error initializing installer: ", output.PrettyErr(err))
		return err
	}
	s.Success("Installer initialized")

	s = o.Printer.StartSpinner("Retrieving cluster configuration")

	err := provider.Initialize(ctx)
	if err != nil {
		s.Fail("Error retrieving provider specific configuration: ", output.PrettyErr(err))
		return err
	}

	if !o.SkipValidation {
		err = o.validate(ctx)
		if err != nil {
			s.Fail("Error retrieving configuration: ", output.PrettyErr(err))
			return err
		}
	}

	s.Success("Cluster configuration correctly retrieved")

	s = o.Printer.StartSpinner("Generating installation parameters")

	values, err := util.MergeMaps(o.chartValues, o.preProviderValues())
	if err != nil {
		s.Fail("Error generating installation parameters: ", output.PrettyErr(err))
		return err
	}

	values, err = util.MergeMaps(values, provider.Values())
	if err != nil {
		s.Fail("Error generating installation parameters: ", output.PrettyErr(err))
		return err
	}

	values, err = util.MergeMaps(values, o.postProviderValues())
	if err != nil {
		s.Fail("Error generating installation parameters: ", output.PrettyErr(err))
		return err
	}

	valuesFiles, err := o.valuesFiles()
	if err != nil {
		s.Fail("Error generating installation parameters: ", output.PrettyErr(err))
		return err
	}
	values, err = util.MergeMaps(values, valuesFiles)
	if err != nil {
		s.Fail("Error generating installation parameters: ", output.PrettyErr(err))
		return err
	}

	for _, value := range o.OverrideValues {
		if err := strvals.ParseInto(value, values); err != nil {
			err := fmt.Errorf("failed parsing --set data: %w", err)
			s.Fail("Error generating installation parameters: ", output.PrettyErr(err))
			return err
		}
	}

	for _, value := range o.OverrideStringValues {
		if err := strvals.ParseIntoString(value, values); err != nil {
			err := fmt.Errorf("failed parsing --set-string data: %w", err)
			s.Fail("Error generating installation parameters: ", output.PrettyErr(err))
			return err
		}
	}

	warnings, err := ValuesWarning(values)
	if err != nil {
		s.Fail("Error generating installation warnings: ", output.PrettyErr(err))
		return err
	}
	for _, warning := range warnings {
		s = o.Printer.SpinnerRunningWarning(s, warning)
	}

	rawValues, err := yaml.Marshal(values)
	if err != nil {
		s.Fail("Error generating values file: ", output.PrettyErr(err))
		return err
	}

	s.Success("Installation parameters correctly generated")

	if o.OnlyOutputValues {
		s = o.Printer.StartSpinner("Generating values.yaml file with the Liqo chart parameters for your cluster")
		if err = utils.WriteFile(o.ValuesPath, rawValues); err != nil {
			s.Fail(fmt.Sprintf("Unable to write the values file to %q: %v", o.ValuesPath, output.PrettyErr(err)))
			return err
		}
		s.Success(fmt.Sprintf("All Set! Chart values written to %q", o.ValuesPath))
		return nil
	}

	s = o.Printer.StartSpinner("Installing or upgrading Liqo... (this may take a few minutes)")
	err = o.installOrUpdate(ctx, string(rawValues), s)
	if err != nil {
		s.Fail("Error installing or upgrading Liqo: ", output.PrettyErr(err))
		if strings.Contains(output.PrettyErr(err), "timed out waiting for the condition") {
			o.Printer.Info.Println("Likely causes for the installation/upgrade timeout could include:")
			o.Printer.Info.Println("* One or more pods failed to start (e.g., they are in the ImagePullBackOff status)")
			o.Printer.Info.Println("* A service of type LoadBalancer has been configured, but no provider is available")
			o.Printer.Info.Println("You can add the --verbose flag for debug information concerning the failing resources")
			o.Printer.Info.Println("Additionally, if necessary, you can increase the timeout value with the --timeout flag")
		}

		return err
	}

	if o.DryRun {
		s.Success("Installation completed (dry-run)")
		return nil
	}

	s.Success(fmt.Sprintf("All Set! You can now proceed establishing a peering (%v peer --help for more information)", o.CommandName))
	o.Printer.Success.Println("Make sure to use the same version of Liqo on all remote clusters")
	return nil
}

func (o *Options) initialize(ctx context.Context, provider Provider) error {
	var err error
	helmClient := o.HelmClient()

	switch {
	// In case a local chart path is specified, use that.
	case o.ChartPath != "":
		// disable telemetry for local charts (development)
		o.DisableTelemetry = true

	// In case the specified version is valid, add the chart through the client.
	case o.isRelease():
		o.ChartPath = liqoChartFullName
		chartRepo := repo.Entry{URL: liqoRepo, Name: liqoChartName}
		if err = helmClient.AddOrUpdateChartRepo(chartRepo); err != nil {
			return err
		}

	// Otherwise, clone the repository and configure it as a local chart path.
	default:
		o.Printer.Warning.Printfln("Non-released version selected. Downloading repository from %q...", o.RepoURL)
		if err = o.cloneRepository(ctx); err != nil {
			if strings.Contains(err.Error(), "object not found") {
				return errors.New("commit not found. Did you select the correct repository?")
			}

			return err
		}

		// disable telemetry if the version is not a release
		o.DisableTelemetry = true
	}

	// Retrieve the default chart values.
	o.Printer.Verbosef("Using chart from %q", o.ChartPath)
	chart, _, err := helmClient.GetChart(o.ChartPath, &action.ChartPathOptions{Version: o.Version})
	if err != nil {
		return err
	}
	o.chartValues = chart.Values
	// Explicitly set the version to the retrieved one, if not previously set.
	if o.Version == "" {
		o.Version = chart.Metadata.Version
	}

	// Retrieve the cluster name used for previous installations, in case it was not specified.
	if o.ClusterID == "" {
		tmp, err := utils.GetClusterID(ctx, o.KubeClient, o.LiqoNamespace)
		if client.IgnoreNotFound(err) != nil {
			return err
		}
		o.ClusterID = tmp
	}

	// Add a label stating the provider name.
	if provider.Name() != "" {
		o.ClusterLabels[consts.ProviderClusterLabel] = provider.Name()
	}

	return nil
}

func (o *Options) installOrUpdate(ctx context.Context, rawValues string, s *pterm.SpinnerPrinter) error {
	chartSpec := helm.ChartSpec{
		ReleaseName: LiqoReleaseName,
		ChartName:   o.ChartPath,
		Version:     o.Version,

		Namespace:       o.LiqoNamespace,
		CreateNamespace: true,
		UpgradeCRDs:     true,
		ValuesYaml:      rawValues,

		Timeout:       o.Timeout,
		DryRun:        o.DryRun,
		Atomic:        true,
		Wait:          true,
		CleanupOnFail: true,
	}

	// Update the text in case the parent context is canceled, to give a feedback that it was considered.
	ctxp, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		<-ctxp.Done()
		if s != nil {
			s.UpdateText("Operation canceled: rolling back...")
		}
	}()

	_, err := o.HelmClient().InstallOrUpgradeChart(ctx, &chartSpec, nil)
	s = nil // Do not print the message in case installation/upgrade succeeded.
	return err
}

func (o *Options) isRelease() bool {
	return o.Version == "" || semver.IsValid(o.Version)
}

func (o *Options) preProviderValues() map[string]interface{} {
	return map[string]interface{}{
		"tag": o.Version,

		"requirements": map[string]interface{}{
			"kernel": map[string]interface{}{
				"disabled": o.DisableKernelVersionCheck,
			},
		},

		"apiServer": map[string]interface{}{
			"address": o.APIServer,
		},

		"discovery": map[string]interface{}{
			"config": map[string]interface{}{
				"clusterID":     string(o.ClusterID),
				"clusterLabels": util.GetInterfaceMap(o.ClusterLabels),
			},
		},

		"ipam": map[string]interface{}{
			"podCIDR":         o.PodCIDR,
			"serviceCIDR":     o.ServiceCIDR,
			"reservedSubnets": util.GetInterfaceSlice(o.ReservedSubnets),
		},

		"metrics": map[string]interface{}{
			"enabled": o.EnableMetrics,
			"prometheusOperator": map[string]interface{}{
				"enabled": o.EnableMetrics,
			},
		},

		"virtualKubelet": map[string]interface{}{
			"metrics": map[string]interface{}{
				"enabled": o.EnableMetrics,
				"podMonitor": map[string]interface{}{
					"enabled": o.EnableMetrics,
				},
			},
		},

		"telemetry": map[string]interface{}{
			"enabled": !o.DisableTelemetry,
		},
	}
}

func (o *Options) postProviderValues() map[string]interface{} {
	return map[string]interface{}{}
}

func (o *Options) valuesFiles() (map[string]interface{}, error) {
	values := map[string]interface{}{}
	// Near copy from Helm https://github.com/helm/helm/blob/eb4edc96c581c1978fe935197e20ced4071af5d5/pkg/cli/values/options.go#L48
	for _, filePath := range o.OverrideValuesFiles {
		currentMap := map[string]interface{}{}
		bytes, err := readFile(filePath, getter.All(cli.New()))
		if err != nil {
			return nil, err
		}

		if err := yaml.Unmarshal(bytes, &currentMap); err != nil {
			return nil, err
		}
		// Merge with the previous map
		values, err = util.MergeMaps(currentMap, values)
		if err != nil {
			return nil, err
		}
	}
	return values, nil
}

// copy from Helm https://github.com/helm/helm/blob/eb4edc96c581c1978fe935197e20ced4071af5d5/pkg/cli/values/options.go#L128
func readFile(filePath string, p getter.Providers) ([]byte, error) {
	if strings.TrimSpace(filePath) == "-" {
		return io.ReadAll(os.Stdin)
	}
	u, err := url.Parse(filePath)
	if err != nil {
		return nil, err
	}

	// FIXME: maybe someone handle other protocols like ftp.
	g, err := p.ByScheme(u.Scheme)
	if err != nil {
		return os.ReadFile(filepath.Clean(filePath))
	}
	data, err := g.Get(filePath, getter.WithURL(filePath))
	if err != nil {
		return nil, err
	}
	return data.Bytes(), err
}

func (o *Options) cleanup() error {
	if o.tmpDir != "" {
		return os.RemoveAll(o.tmpDir)
	}
	return nil
}
