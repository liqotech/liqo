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
	"fmt"
	"math/rand"
	"strings"

	"github.com/goombaio/namegenerator"
	flag "github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/liqotech/liqo/pkg/consts"
	installutils "github.com/liqotech/liqo/pkg/liqoctl/install/utils"
	argsutils "github.com/liqotech/liqo/pkg/utils/args"
)

// Providers list of providers supported by liqoctl.
var Providers = []string{"kubeadm", "kind", "k3s", "eks", "gke", "aks", "openshift"}

// GenericProvider includes the fields and the logic required by every install provider.
type GenericProvider struct {
	ReservedSubnets     []string
	ClusterLabels       map[string]string
	GenerateClusterName bool
	ClusterName         string
	LanDiscovery        *bool
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
		fmt.Printf("* A random cluster name was generated for you: %s\n", p.ClusterName)
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
