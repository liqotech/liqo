package provider

import (
	flag "github.com/spf13/pflag"

	argsutils "github.com/liqotech/liqo/pkg/utils/args"
)

// GenericProvider includes the fields and the logic required by every install provider.
type GenericProvider struct {
	ReservedSubnets []string
	ClusterLabels   map[string]string
	ClusterName     string
}

// ValidateGenericCommandArguments validates the flags required by every install provider.
func (p *GenericProvider) ValidateGenericCommandArguments(flags *flag.FlagSet) (err error) {
	p.ClusterName, err = flags.GetString("cluster-name")
	if err != nil {
		return err
	}

	subnetString, err := flags.GetString("reserved-subnets")
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
