package gke

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	flag "github.com/spf13/pflag"
	"google.golang.org/api/container/v1"

	"github.com/liqotech/liqo/pkg/consts"
)

func TestFetchingParameters(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test GKE provider")
}

const (
	endpoint    = "https://example.com"
	podCIDR     = "10.0.0.0/16"
	serviceCIDR = "10.80.0.0/16"

	credentialsPath = "path"
	projectID       = "id"
	zone            = "zone"
	clusterID       = "cluster-id"
)

var _ = Describe("Extract elements from GKE", func() {

	It("test flags", func() {

		p := NewProvider().(*gkeProvider)

		flags := flag.NewFlagSet("test", flag.PanicOnError)

		GenerateFlags(flags)

		Expect(flags.Set("gke.credentials-path", credentialsPath)).To(Succeed())
		Expect(flags.Set("gke.project-id", projectID)).To(Succeed())
		Expect(flags.Set("gke.zone", zone)).To(Succeed())
		Expect(flags.Set("gke.cluster-id", clusterID)).To(Succeed())

		Expect(p.ValidateCommandArguments(flags)).To(Succeed())

		Expect(p.credentialsPath).To(Equal(credentialsPath))
		Expect(p.projectID).To(Equal(projectID))
		Expect(p.zone).To(Equal(zone))
		Expect(p.clusterID).To(Equal(clusterID))

	})

	It("test parse values", func() {

		clusterOutput := &container.Cluster{
			Endpoint:         endpoint,
			ServicesIpv4Cidr: serviceCIDR,
			ClusterIpv4Cidr:  podCIDR,
			Location:         zone,
		}

		p := NewProvider().(*gkeProvider)

		p.parseClusterOutput(clusterOutput)

		Expect(p.endpoint).To(Equal(endpoint))
		Expect(p.serviceCIDR).To(Equal(serviceCIDR))
		Expect(p.podCIDR).To(Equal(podCIDR))

		Expect(p.clusterLabels).ToNot(BeEmpty())
		Expect(p.clusterLabels[consts.ProviderClusterLabel]).To(Equal(providerPrefix))
		Expect(p.clusterLabels[consts.TopologyRegionClusterLabel]).To(Equal(zone))

	})

})
