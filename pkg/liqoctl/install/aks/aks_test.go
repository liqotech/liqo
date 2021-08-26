package aks

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2021-07-01/containerservice"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	flag "github.com/spf13/pflag"
	"k8s.io/utils/pointer"
)

func TestFetchingParameters(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test AKS provider")
}

const (
	endpoint    = "https://example.com"
	podCIDR     = "10.0.0.0/16"
	serviceCIDR = "10.80.0.0/16"

	subscriptionID    = "subID"
	resourceGroupName = "test"
	resourceName      = "liqo"
)

var _ = Describe("Extract elements from AKS", func() {

	It("test flags", func() {

		p := NewProvider().(*aksProvider)

		flags := flag.NewFlagSet("test", flag.PanicOnError)

		GenerateFlags(flags)

		Expect(flags.Set("aks.subscription-id", subscriptionID)).To(Succeed())
		Expect(flags.Set("aks.resource-group-name", resourceGroupName)).To(Succeed())
		Expect(flags.Set("aks.resource-name", resourceName)).To(Succeed())

		Expect(p.ValidateCommandArguments(flags)).To(Succeed())

		Expect(p.subscriptionID).To(Equal(subscriptionID))
		Expect(p.resourceGroupName).To(Equal(resourceGroupName))
		Expect(p.resourceName).To(Equal(resourceName))

	})

	It("test parse values", func() {
		ctx := context.TODO()

		clusterOutput := &containerservice.ManagedCluster{
			ManagedClusterProperties: &containerservice.ManagedClusterProperties{
				Fqdn: pointer.StringPtr(endpoint),
				NetworkProfile: &containerservice.NetworkProfile{
					NetworkPlugin: containerservice.NetworkPluginKubenet,
					PodCidr:       pointer.StringPtr(podCIDR),
					ServiceCidr:   pointer.StringPtr(serviceCIDR),
				},
				AgentPoolProfiles: &[]containerservice.ManagedClusterAgentPoolProfile{
					{
						VnetSubnetID: nil,
					},
				},
			},
		}

		p := NewProvider().(*aksProvider)

		Expect(p.parseClusterOutput(ctx, clusterOutput)).To(Succeed())

		Expect(p.endpoint).To(Equal(endpoint))
		Expect(p.serviceCIDR).To(Equal(serviceCIDR))
		Expect(p.podCIDR).To(Equal(podCIDR))
		Expect(len(p.reservedSubnets)).To(BeNumerically("==", 1))
		Expect(p.reservedSubnets).To(ContainElement(defaultAksNodeCIDR))

	})

})
