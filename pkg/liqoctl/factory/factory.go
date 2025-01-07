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

package factory

import (
	"strings"

	helm "github.com/mittwald/go-helm-client"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/completion"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

var verbose bool

// FlagNamespace -> the name of the namespace flag.
const FlagNamespace = "namespace"

// FlagLiqoNamespace -> the name of the Liqo namespace flag.
const FlagLiqoNamespace = "liqo-namespace"

type completionFuncRegisterer func(flagName string, f func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective)) error

// Factory provides a set of clients and configurations to authenticate and
// access a target Kubernetes cluster. Factory will ensure that its fields are
// populated and valid during command execution.
type Factory struct {
	// ConfigFlags wraps the logic to retrieve a REST config based on the flags.
	configFlags *genericclioptions.ConfigFlags
	// Factory wraps the logic to retrieve a REST config based on the flags.
	factory cmdutil.Factory
	// Preserve the namespace flag, since it is added only to a subset of commands.
	namespaceFlag *pflag.Flag
	// Whether it refers to a remote cluster.
	remote bool

	// Printer is the object used to output messages in the appropriate format.
	Printer *output.Printer
	// PrinterGlobal is the object used to output messages in the appropriate format. It is not scoped to local or remote cluster.
	PrinterGlobal *output.Printer
	// SkipConfirm determines whether to skip confirmations.
	SkipConfirm bool
	// Whether to add a scope to the printer (i.e., local/remote).
	ScopedPrinter bool

	// Namespace is the namespace that the user has requested with the "--namespace" / "-n" flag.
	Namespace string

	// GlobalLabels is a map of labels that will be added to all resources created by liqoctl
	GlobalLabels map[string]string

	// GlobalAnnotations is a map of annotations that will be added to all resources created by liqoctl
	GlobalAnnotations map[string]string

	// LiqoNamespace is the namespace (where Liqo is installed) that the user has requested with the "--liqo-namespace" / "-l" flag,
	// if registered.
	LiqoNamespace string

	// RESTConfig is a Kubernetes REST config that contains the user's authentication and access configuration.
	RESTConfig *rest.Config

	// crClient is the controller runtime client.
	CRClient client.Client

	// kubeClient is a Kubernetes clientset for interacting with the base Kubernetes APIs.
	KubeClient kubernetes.Interface

	// DynCLient
	DynClient *dynamic.DynamicClient

	helmClient helm.Client
}

// NewForLocal returns a new initialized Factory, to interact with a remote cluster.
func NewForLocal() *Factory {
	flags := genericclioptions.NewConfigFlags(true)
	return &Factory{
		configFlags: flags,
		factory:     cmdutil.NewFactory(flags),
		remote:      false,
	}
}

// NewForRemote returns a new initialized Factory, to interact with a remote cluster.
func NewForRemote() *Factory {
	factory := NewForLocal()
	factory.remote = true
	factory.ScopedPrinter = true
	return factory
}

// HelmClient returns an Helm client, initializing it if necessary. In case of error, it outputs
// the error (through the spinner if provided, or leveraging the printer) and exits.
func (f *Factory) HelmClient() helm.Client {
	cl, err := f.HelmClientOrError()
	f.Printer.CheckErr(err)
	return cl
}

// HelmClientOrError returns an Helm client, initializing it if necessary.
func (f *Factory) HelmClientOrError() (helm.Client, error) {
	if f.helmClient != nil {
		return f.helmClient, nil
	}

	opt := &helm.RestConfClientOptions{
		RestConfig: f.RESTConfig,
		Options: &helm.Options{
			Namespace: f.LiqoNamespace,
			Debug:     verbose,
			DebugLog:  f.Printer.Verbosef,
		},
	}

	var err error
	f.helmClient, err = helm.NewClientFromRestConf(opt)
	return f.helmClient, err
}

// AddFlags registers the flags to interact with the Kubernetes access options, and the corresponding completion functions.
func (f *Factory) AddFlags(flags *pflag.FlagSet, register completionFuncRegisterer) {
	// We use an accessory flagset to mutate the flags before adding them to the command.
	tmp := pflag.NewFlagSet("factory", pflag.PanicOnError)
	f.configFlags.AddFlags(tmp)

	tmp.VisitAll(func(flag *pflag.Flag) {
		if flag.Name == FlagNamespace {
			// Exclude the flag concerning the namespace, as manually added only to the relevant subcommands.
			flag.Usage = "The namespace scope for this request"
			f.namespaceFlag = flag
			return
		}

		flag.Usage = strings.TrimRight(flag.Usage, ".")
		// Hide most non-essential flags
		if flag.Name != "kubeconfig" && flag.Name != "context" && flag.Name != "user" && flag.Name != "cluster" {
			flag.Hidden = true
		}

		flags.AddFlag(f.remotifyFlag(flag))
	})

	if !f.remote {
		flags.BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logs (default false)")
	}

	utilruntime.Must(register(f.remotify("context"), f.completion(completion.ListContextsInConfig)))
	utilruntime.Must(register(f.remotify("cluster"), f.completion(completion.ListClustersInConfig)))
	utilruntime.Must(register(f.remotify("user"), f.completion(completion.ListUsersInConfig)))
}

// AddNamespaceFlag registers the flag to select the target namespace (alternative to AddLiqoNamespaceFlag).
func (f *Factory) AddNamespaceFlag(flags *pflag.FlagSet) {
	otherFlag := flags.Lookup(f.remotify(FlagNamespace))
	if otherFlag != nil {
		// The flag is already registered.
		panic("the namespace flag is already registered, make sure to call AddNamespaceFlag before AddLiqoNamespaceFlag")
	}
	flags.AddFlag(f.remotifyFlag(f.namespaceFlag))
}

// AddLiqoNamespaceFlag registers the flag to select the Liqo namespace (alternative to AddNamespaceFlag).
func (f *Factory) AddLiqoNamespaceFlag(flags *pflag.FlagSet) {
	tmp := pflag.NewFlagSet("factory", pflag.PanicOnError)
	var flagName string
	var short string
	otherFlag := flags.Lookup(f.remotify(FlagNamespace))
	if otherFlag == nil {
		flagName = FlagNamespace
		short = "n"
	} else {
		flagName = FlagLiqoNamespace
		short = ""
	}
	tmp.StringVarP(&f.LiqoNamespace, flagName, short, consts.DefaultLiqoNamespace, "The namespace where Liqo is installed in")
	fl := tmp.Lookup(flagName)
	flags.AddFlag(f.remotifyFlag(fl))
}

type options struct{ scoped bool }

// Options represents an option for the initialize function.
type Options func(*options)

// WithScopedPrinter marks the generated printer as scoped.
func WithScopedPrinter(o *options) { o.scoped = true }

// Initialize populates the object based on the provided flags.
func (f *Factory) Initialize(opts ...Options) (err error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}

	if f.remote {
		f.Printer = output.NewRemotePrinter(o.scoped, verbose)
	} else {
		f.Printer = output.NewLocalPrinter(o.scoped, verbose)
	}

	f.PrinterGlobal = output.NewGlobalPrinter(o.scoped, verbose)

	if f.Namespace == "" {
		f.Namespace, _, err = f.factory.ToRawKubeConfigLoader().Namespace()
		if err != nil {
			return err
		}
	}

	f.RESTConfig, err = f.factory.ToRESTConfig()
	if err != nil {
		return err
	}
	restcfg.SetRateLimiter(f.RESTConfig)

	restMapper, err := f.factory.ToRESTMapper()
	if err != nil {
		return err
	}

	f.KubeClient, err = f.factory.KubernetesClientSet()
	if err != nil {
		return err
	}

	f.DynClient, err = dynamic.NewForConfig(f.RESTConfig)
	if err != nil {
		return err
	}

	// Leverage the REST mapper retrieved from the factory, to defer the loading of the mappings until the first API
	// request is made. This prevents errors in case of invalid kubeconfigs, if no interaction is required.
	f.CRClient, err = client.New(f.RESTConfig, client.Options{Mapper: restMapper})
	return err
}

func (f *Factory) remotifyFlag(flag *pflag.Flag) *pflag.Flag {
	flag.Name = f.remotify(flag.Name)
	if f.remote {
		// Add the remote prefix, and disable shorthands.
		flag.Shorthand = ""
		flag.Usage += " (in the remote cluster)"
	}

	return flag
}

func (f *Factory) remotify(name string) string {
	if f.remote {
		return "remote-" + name
	}
	return name
}

func (f *Factory) completion(cmpl func(string) []string) func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		completion.SetFactoryForCompletion(f.factory)
		return cmpl(toComplete), cobra.ShellCompDirectiveNoFileComp
	}
}
