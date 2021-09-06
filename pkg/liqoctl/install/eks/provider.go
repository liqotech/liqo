package eks

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/install/provider"
	installutils "github.com/liqotech/liqo/pkg/liqoctl/install/utils"
)

const (
	providerPrefix = "eks"

	regionFlag          = "region"
	clusterNameFlag     = "cluster-name"
	userNameFlag        = "user-name"
	policyNameFlag      = "policy-name"
	accessKeyIDFlag     = "access-key-id"
	secretAccessKeyFlag = "secret-access-key"
)

type eksProvider struct {
	region      string
	clusterName string

	endpoint    string
	serviceCIDR string
	podCIDR     string

	iamLiqoUser   iamLiqoUser
	clusterLabels map[string]string
}

type iamLiqoUser struct {
	userName   string
	policyName string

	accessKeyID     string
	secretAccessKey string
}

// NewProvider initializes a new EKS provider struct.
func NewProvider() provider.InstallProviderInterface {
	return &eksProvider{
		clusterLabels: map[string]string{
			consts.ProviderClusterLabel: providerPrefix,
		},
	}
}

// ValidateCommandArguments validates specific arguments passed to the install command.
func (k *eksProvider) ValidateCommandArguments(flags *flag.FlagSet) (err error) {
	k.region, err = flags.GetString(regionFlag)
	if err != nil {
		return err
	}
	klog.V(3).Infof("EKS Region: %v", k.region)

	k.clusterName, err = flags.GetString(clusterNameFlag)
	if err != nil {
		return err
	}
	klog.V(3).Infof("EKS ClusterName: %v", k.clusterName)

	k.iamLiqoUser.userName, err = flags.GetString(userNameFlag)
	if err != nil {
		return err
	}
	klog.V(3).Infof("Liqo IAM username: %v", k.iamLiqoUser.userName)

	k.iamLiqoUser.policyName, err = flags.GetString(policyNameFlag)
	if err != nil {
		return err
	}
	klog.V(3).Infof("Liqo IAM policy name: %v", k.iamLiqoUser.policyName)

	// optional values

	k.iamLiqoUser.accessKeyID, err = flags.GetString(accessKeyIDFlag)
	if err != nil {
		return err
	}

	k.iamLiqoUser.secretAccessKey, err = flags.GetString(secretAccessKeyFlag)
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
func (k *eksProvider) ExtractChartParameters(ctx context.Context, config *rest.Config, commonArgs *provider.CommonArguments) error {
	sess, err := session.NewSession()
	if err != nil {
		return err
	}

	if err = k.getClusterInfo(sess); err != nil {
		return err
	}

	if !commonArgs.DisableEndpointCheck {
		if valid, err := installutils.CheckEndpoint(k.endpoint, config); err != nil {
			return err
		} else if !valid {
			return fmt.Errorf("the retrieved cluster information and the cluster selected in the kubeconfig do not match")
		}
	}

	if err = k.createIamIdentity(sess); err != nil {
		return err
	}

	return nil
}

// UpdateChartValues patches the values map with the values required for the selected cluster.
func (k *eksProvider) UpdateChartValues(values map[string]interface{}) {
	values["gateway"] = map[string]interface{}{
		"service": map[string]interface{}{
			"annotations": map[string]interface{}{
				"service.beta.kubernetes.io/aws-load-balancer-type": "nlb",
			},
		},
	}
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
	values["discovery"] = map[string]interface{}{
		"config": map[string]interface{}{
			"clusterLabels": installutils.GetInterfaceMap(k.clusterLabels),
		},
	}
	values["controllerManager"] = map[string]interface{}{
		"pod": map[string]interface{}{
			"extraArgs": []interface{}{"--disable-kubelet-certificate-generation=true"},
		},
	}
}

// GenerateFlags generates the set of specific subpath and flags are accepted for a specific provider.
func GenerateFlags(command *cobra.Command) {
	flags := command.Flags()

	flags.String(regionFlag, "", "The EKS region where your cluster is running")
	flags.String(clusterNameFlag, "", "The EKS clusterName of your cluster")

	flags.String(userNameFlag, "liqo-cluster-user", "The username to assign to the Liqo user. "+
		"This user will be created if no access keys have been provided, "+
		"otherwise liqoctl assumes that the provided keys are related to this user (optional)")
	flags.String(policyNameFlag, "liqo-cluster-policy", "The name of the policy to assign to the Liqo user (optional)")

	flags.String(accessKeyIDFlag, "", "The IAM accessKeyID for the Liqo user (optional)")
	flags.String(secretAccessKeyFlag, "", "The IAM secretAccessKey for the Liqo user (optional)")

	utilruntime.Must(command.MarkFlagRequired(regionFlag))
	utilruntime.Must(command.MarkFlagRequired(clusterNameFlag))
}
