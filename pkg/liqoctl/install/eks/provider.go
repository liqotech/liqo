package eks

import (
	"context"

	"github.com/aws/aws-sdk-go/aws/session"
	flag "github.com/spf13/pflag"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/liqoctl/install/provider"
	installutils "github.com/liqotech/liqo/pkg/liqoctl/install/utils"
)

const (
	providerPrefix = "eks"
)

type eksProvider struct {
	region      string
	clusterName string

	endpoint    string
	serviceCIDR string
	podCIDR     string

	iamLiqoUser iamLiqoUser
}

type iamLiqoUser struct {
	userName   string
	policyName string

	accessKeyID     string
	secretAccessKey string
}

// NewProvider initializes a new EKS provider struct.
func NewProvider() provider.InstallProviderInterface {
	return &eksProvider{}
}

// ValidateCommandArguments validates specific arguments passed to the install command.
func (k *eksProvider) ValidateCommandArguments(flags *flag.FlagSet) (err error) {
	k.region, err = installutils.CheckStringFlagIsSet(flags, providerPrefix, "region")
	if err != nil {
		return err
	}
	klog.V(3).Infof("EKS Region: %v", k.region)

	k.clusterName, err = installutils.CheckStringFlagIsSet(flags, providerPrefix, "cluster-name")
	if err != nil {
		return err
	}
	klog.V(3).Infof("EKS ClusterName: %v", k.clusterName)

	k.iamLiqoUser.userName, err = installutils.CheckStringFlagIsSet(flags, providerPrefix, "user-name")
	if err != nil {
		return err
	}
	klog.V(3).Infof("Liqo IAM username: %v", k.iamLiqoUser.userName)

	k.iamLiqoUser.policyName, err = installutils.CheckStringFlagIsSet(flags, providerPrefix, "policy-name")
	if err != nil {
		return err
	}
	klog.V(3).Infof("Liqo IAM policy name: %v", k.iamLiqoUser.policyName)

	// optional values

	k.iamLiqoUser.accessKeyID, err = flags.GetString(installutils.PrefixedName(providerPrefix, "access-key-id"))
	if err != nil {
		return err
	}

	k.iamLiqoUser.secretAccessKey, err = flags.GetString(installutils.PrefixedName(providerPrefix, "secret-access-key"))
	if err != nil {
		return err
	}

	storedAccessKeyID, storedSecretAccessKey, err := retrieveIamAccessKey(k.iamLiqoUser.userName)
	if err != nil {
		return err
	}

	if storedAccessKeyID != "" && k.iamLiqoUser.accessKeyID == "" {
		k.iamLiqoUser.accessKeyID = storedAccessKeyID
	}
	if storedSecretAccessKey != "" && k.iamLiqoUser.secretAccessKey == "" {
		k.iamLiqoUser.secretAccessKey = storedSecretAccessKey
	}

	return nil
}

// ExtractChartParameters fetches the parameters used to customize the Liqo installation on a specific cluster of a
// given provider.
func (k *eksProvider) ExtractChartParameters(ctx context.Context, _ *rest.Config) error {
	sess, err := session.NewSession()
	if err != nil {
		return err
	}

	if err = k.getClusterInfo(sess); err != nil {
		return err
	}

	if err = k.createIamIdentity(sess); err != nil {
		return err
	}

	return nil
}

// UpdateChartValues patches the values map with the values required for the selected cluster.
func (k *eksProvider) UpdateChartValues(values map[string]interface{}) {
	values["apiServer"] = map[string]interface{}{
		"address": k.endpoint,
	}
	values["networkManager"] = map[string]interface{}{
		"config": map[string]interface{}{
			"serviceCIDR": k.serviceCIDR,
			"podCIDR":     k.podCIDR,
		},
	}
	values["awsConfig"] = map[string]interface{}{
		"accessKeyId":     k.iamLiqoUser.accessKeyID,
		"secretAccessKey": k.iamLiqoUser.secretAccessKey,
		"region":          k.region,
		"clusterName":     k.clusterName,
	}
}

// GenerateFlags generates the set of specific subpath and flags are accepted for a specific provider.
func GenerateFlags(flags *flag.FlagSet) {
	subFlag := flag.NewFlagSet(providerPrefix, flag.ExitOnError)
	subFlag.SetNormalizeFunc(func(f *flag.FlagSet, name string) flag.NormalizedName {
		return flag.NormalizedName(installutils.PrefixedName(providerPrefix, name))
	})

	subFlag.String("region", "", "The EKS region where your cluster is running")
	subFlag.String("cluster-name", "", "The EKS clusterName of your cluster")

	subFlag.String("user-name", "liqo-cluster-user", "The username to assign to the Liqo user. "+
		"This user will be created if no access keys have been provided, "+
		"otherwise liqoctl assumes that the provided keys are related to this user (optional)")
	subFlag.String("policy-name", "liqo-cluster-policy", "The name of the policy to assign to the Liqo user (optional)")

	subFlag.String("access-key-id", "", "The IAM accessKeyID for the Liqo user (optional)")
	subFlag.String("secret-access-key", "", "The IAM secretAccessKey for the Liqo user (optional)")

	flags.AddFlagSet(subFlag)
}
