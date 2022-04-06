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
	"fmt"
	"math/rand"
	"net"
	"net/url"
	"strings"

	"github.com/goombaio/namegenerator"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/liqotech/liqo/pkg/consts"
	installutils "github.com/liqotech/liqo/pkg/liqoctl/install/utils"
	argsutils "github.com/liqotech/liqo/pkg/utils/args"
	logsutils "github.com/liqotech/liqo/pkg/utils/logs"
)

// Providers list of providers supported by liqoctl.
var Providers = []string{"kubeadm", "kind", "k3s", "eks", "gke", "aks", "openshift"}

// GenericProviderName is the name of the generic provider.
const GenericProviderName = ""

var (
	localhostValues = []string{
		"localhost",
		"127.0.0.1",
	}
)

const (
	podCidrFlag     = "pod-cidr"
	serviceCidrFlag = "service-cidr"
	apiServerFlag   = "api-server"
)

// GenericProvider includes the fields and the logic required by every install provider.
type GenericProvider struct {
	ReservedSubnets     []string
	ClusterLabels       map[string]string
	GenerateClusterName bool
	ClusterName         string
	LanDiscovery        *bool

	PodCIDR     string
	ServiceCIDR string
	APIServer   string
	K8sClient   kubernetes.Interface
	Config      *rest.Config
}

// NewProvider initializes a new generic provider.
func NewProvider() InstallProviderInterface {
	return &GenericProvider{}
}

// PreValidateGenericCommandArguments validates the flags required by every install provider
// before the specific provider validation.
func (p *GenericProvider) PreValidateGenericCommandArguments(flags *flag.FlagSet) (err error) {
	p.GenerateClusterName, err = flags.GetBool(consts.GenerateNameParameter)
	if err != nil {
		return err
	}

	p.ClusterName, err = flags.GetString(consts.ClusterNameParameter)
	if err != nil {
		return err
	}

	clusterLabels, err := flags.GetString(consts.ClusterLabelsParameter)
	if err != nil {
		return err
	}
	clusterLabelsVar := argsutils.StringMap{}
	if err := clusterLabelsVar.Set(clusterLabels); err != nil {
		return err
	}
	resultMap, err := installutils.MergeMaps(installutils.GetInterfaceMap(p.ClusterLabels), installutils.GetInterfaceMap(clusterLabelsVar.StringMap))
	if err != nil {
		return err
	}
	p.ClusterLabels = installutils.GetStringMap(resultMap)

	// Changed returns true if the flag has been explicitly set by the user in the command issued via the command line.
	// Used to tell a default value from a user-defined one, when they are equal.
	// In case of a user-defined value, the provider value will be overridden, otherwise, the provider value will be used (later).
	if flags.Changed(consts.EnableLanDiscoveryParameter) {
		lanDiscovery, err := flags.GetBool(consts.EnableLanDiscoveryParameter)
		if err != nil {
			return err
		}
		p.LanDiscovery = &lanDiscovery
	}

	subnetString, err := flags.GetString(consts.ReservedSubnetsParameter)
	if err != nil {
		return err
	}
	reservedSubnets := argsutils.CIDRList{}
	if err = reservedSubnets.Set(subnetString); err != nil {
		return err
	}
	p.ReservedSubnets = reservedSubnets.StringList.StringList

	return nil
}

// PostValidateGenericCommandArguments validates the flags required by every install provider
// after the specific provider validation.
func (p *GenericProvider) PostValidateGenericCommandArguments(oldClusterName string) (err error) {
	switch {
	// no cluster name is provided, no cluster name is generated and there is no old cluster name
	// -> throw an error
	case p.ClusterName == "" && !p.GenerateClusterName && oldClusterName == "":
		p.ClusterName = ""
		return fmt.Errorf("you must provide a cluster name or use --generate-name")
		// both cluster name and generate name are provided
		// -> throw an error
	case p.ClusterName != "" && p.GenerateClusterName:
		p.ClusterName = ""
		return fmt.Errorf("cannot set a cluster name and use --generate-name at the same time")
	// we have to generate a cluster name, and there is no old cluster name
	case p.GenerateClusterName && oldClusterName == "":
		randomName := namegenerator.NewNameGenerator(rand.Int63()).Generate() // nolint:gosec // don't need crypto/rand
		randomName = strings.Replace(randomName, "_", "-", 1)
		p.ClusterName = randomName
	// no cluster name provided, but we can use the old one (we don't care about generation)
	case p.ClusterName == "" && oldClusterName != "":
		p.ClusterName = oldClusterName
	// else, do not change the cluster name, use the user provided one
	default:
	}

	errs := validation.IsDNS1123Label(p.ClusterName)
	if len(errs) != 0 {
		p.ClusterName = ""
		return fmt.Errorf("the cluster name may only contain lowercase letters, numbers and hyphens, and must not be no longer than 63 characters")
	}

	return nil
}

// ValidateCommandArguments validates specific arguments passed to the install command.
func (p *GenericProvider) ValidateCommandArguments(flags *flag.FlagSet) (err error) {
	p.PodCIDR, err = flags.GetString(podCidrFlag)
	if err != nil {
		return err
	}
	logsutils.Infof("PodCIDR: %v", p.PodCIDR)

	p.ServiceCIDR, err = flags.GetString(serviceCidrFlag)
	if err != nil {
		return err
	}
	logsutils.Infof("ServiceCIDR: %v", p.ServiceCIDR)

	p.APIServer, err = flags.GetString(apiServerFlag)
	if err != nil {
		return err
	}
	if p.APIServer != "" {
		logsutils.Infof("API Server: %v", p.APIServer)
	}

	return nil
}

// ExtractChartParameters fetches the parameters used to customize the Liqo installation on a specific cluster of a
// given provider.
func (p *GenericProvider) ExtractChartParameters(ctx context.Context, config *rest.Config, _ *CommonArguments) error {
	k8sClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("Unable to create client: %s", err)
		return err
	}

	p.K8sClient = k8sClient
	p.Config = config

	if p.APIServer == "" {
		p.APIServer = config.Host
	}

	if err = ValidateAPIServer(p.APIServer); err != nil {
		return err
	}

	if err = ValidatePodCIDR(ctx, p.K8sClient, p.PodCIDR); err != nil {
		return err
	}

	return ValidateServiceCIDR(ctx, p.K8sClient, p.ServiceCIDR)
}

// UpdateChartValues patches the values map with the values required for the selected cluster.
func (p *GenericProvider) UpdateChartValues(values map[string]interface{}) {
	values["apiServer"] = map[string]interface{}{
		"address": p.APIServer,
	}
	values["networkManager"] = map[string]interface{}{
		"config": map[string]interface{}{
			"serviceCIDR":     p.ServiceCIDR,
			"podCIDR":         p.PodCIDR,
			"reservedSubnets": installutils.GetInterfaceSlice(p.ReservedSubnets),
		},
	}
	values["discovery"] = map[string]interface{}{
		"config": map[string]interface{}{
			"clusterLabels": installutils.GetInterfaceMap(p.ClusterLabels),
			"clusterName":   p.ClusterName,
		},
	}
}

// GenerateFlags generates the set of specific subpath and flags are accepted for a specific provider.
func GenerateFlags(command *cobra.Command) {
	flags := command.Flags()

	flags.String(podCidrFlag, "", "The Pod CIDR for your cluster")
	flags.String(serviceCidrFlag, "", "The Service CIDR for your cluster")
	flags.String(apiServerFlag, "", "Your cluster API Server URL")

	utilruntime.Must(command.MarkFlagRequired(podCidrFlag))
	utilruntime.Must(command.MarkFlagRequired(serviceCidrFlag))
	utilruntime.Must(command.MarkFlagRequired(apiServerFlag))
}

// ValidateServiceCIDR validates that the services in the target cluster matches the provided service CIDR.
// This is necessary for some distributions (for example k3s) do not provide us an API to retrieve
// the service CIDR of the cluster. We can only check if the provided one fits the services IPs.
func ValidateServiceCIDR(ctx context.Context, k8sClient kubernetes.Interface, serviceCIDR string) error {
	svcList, err := k8sClient.CoreV1().Services(v1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	_, svcNet, err := net.ParseCIDR(serviceCIDR)
	if err != nil {
		return err
	}

	for i := range svcList.Items {
		svc := &svcList.Items[i]
		clusterIP := svc.Spec.ClusterIP
		if clusterIP != "None" && clusterIP != "" && !svcNet.Contains(net.ParseIP(clusterIP)) {
			return fmt.Errorf(
				"it seems that the specified service CIDR (%v) is not correct as it does not match the services in your cluster", serviceCIDR)
		}
	}

	return nil
}

// ValidatePodCIDR validates that the pods in the target cluster matches the provided pod CIDR.
// This is necessary for some distributions (for example k3s) do not provide us an API to retrieve
// the pod CIDR of the cluster. We can only check if the provided one fits the pods IPs
// (excluding the HostNetwork Pods and the offloaded ones).
func ValidatePodCIDR(ctx context.Context, k8sClient kubernetes.Interface, podCIDR string) error {
	podList, err := k8sClient.CoreV1().Pods(v1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("!%v", consts.LocalPodLabelKey),
	})
	if err != nil {
		return err
	}

	_, podNet, err := net.ParseCIDR(podCIDR)
	if err != nil {
		return err
	}

	for i := range podList.Items {
		pod := &podList.Items[i]
		podIP := pod.Status.PodIP
		if podIP != "" && !pod.Spec.HostNetwork && !podNet.Contains(net.ParseIP(podIP)) {
			return fmt.Errorf(
				"it seems that the specified pod CIDR (%v) is not correct as it does not match the pods in your cluster", podCIDR)
		}
	}

	return nil
}

// ValidateAPIServer validates that the API server URL.
func ValidateAPIServer(apiServer string) error {
	apiServerURL, err := url.Parse(apiServer)
	if err != nil {
		return err
	}

	hostname := apiServerURL.Hostname()
	for i := range localhostValues {
		if hostname == localhostValues[i] {
			return fmt.Errorf("cannot use localhost (%v) as API Server address, set an external address in your kubeconfig", hostname)
		}
	}

	return nil
}
