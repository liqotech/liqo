package natmappinginflater_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"

	liqonetapi "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/pkg/liqonet/errors"
	"github.com/liqotech/liqo/pkg/liqonet/natmappinginflater"
)

var inflater *natmappinginflater.NatMappingInflater
var dynClient dynamic.Interface

const (
	invalidValue = "invalid value"
	clusterID1   = "cluster-test"
	clusterID2   = "cluster-2-test"
)

func setDynClient() error {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{
		Group:   "net.liqo.io",
		Version: "v1alpha1",
		Kind:    "natmappings",
	}, &liqonetapi.NatMapping{})

	var m = make(map[schema.GroupVersionResource]string)

	m[schema.GroupVersionResource{
		Group:    "net.liqo.io",
		Version:  "v1alpha1",
		Resource: "ipamstorages",
	}] = "ipamstoragesList"

	m[schema.GroupVersionResource{
		Group:    "net.liqo.io",
		Version:  "v1alpha1",
		Resource: "natmappings",
	}] = "natmappingsList"

	// Init fake dynamic client with objects in order to avoid errors in InitNatMappings func
	// due to the lack of support of fake.dynamicClient for creation of more than 2 resources of the same Kind.
	nm1, err := natmappinginflater.ForgeNatMapping(clusterID1, "10.0.0.0/24", "10.0.1.0/24", make(map[string]string))
	if err != nil {
		return err
	}
	nm2, err := natmappinginflater.ForgeNatMapping(clusterID2, "10.0.0.0/24", "10.0.1.0/24", map[string]string{})
	if err != nil {
		return err
	}

	dynClient = fake.NewSimpleDynamicClientWithCustomListKinds(scheme, m, nm1, nm2)
	return nil
}

var _ = Describe("NatMappingInflater", func() {
	BeforeEach(func() {
		err := setDynClient()
		Expect(err).To(BeNil())
		inflater = natmappinginflater.NewInflater(dynClient)
	})
	Describe("InitNatMappingsPerCluster", func() {
		Context("Passing an invalid PodCIDR", func() {
			It("should return a WrongParameter error", func() {
				err := inflater.InitNatMappingsPerCluster(invalidValue, "10.0.1.0/24", "cluster1")
				Expect(err).To(MatchError(fmt.Sprintf("%s must be %s", invalidValue, errors.ValidCIDR)))
			})
		})
		Context("Passing an invalid ExternalCIDR", func() {
			It("should return a WrongParameter error", func() {
				err := inflater.InitNatMappingsPerCluster("10.0.1.0/24", invalidValue, "cluster1")
				Expect(err).To(MatchError(fmt.Sprintf("%s must be %s", invalidValue, errors.ValidCIDR)))
			})
		})
		Context("Passing an empty Cluster ID", func() {
			It("should return a WrongParameter error", func() {
				err := inflater.InitNatMappingsPerCluster("10.0.1.0/24", "10.0.0.0/24", "")
				Expect(err).To(MatchError(fmt.Sprintf("ClusterID must be %s", errors.StringNotEmpty)))
			})
		})
		Context("Passing an empty PodCIDR", func() {
			It("should return a WrongParameter error", func() {
				err := inflater.InitNatMappingsPerCluster("", "10.0.0.0/24", clusterID1)
				Expect(err).To(MatchError(fmt.Sprintf("PodCIDR must be %s", errors.StringNotEmpty)))
			})
		})
		Context("Passing an empty ExternalCIDR", func() {
			It("should return a WrongParameter error", func() {
				err := inflater.InitNatMappingsPerCluster("10.0.1.0/24", "", clusterID1)
				Expect(err).To(MatchError(fmt.Sprintf("ExternalCIDR must be %s", errors.StringNotEmpty)))
			})
		})
		Context("Initializing mappings if resource already exists", func() {
			It("should retrieve configuration from resource", func() {
				podCIDR := "10.0.0.0/24"
				externalCIDR := "10.0.1.0/24"
				backedMappings := map[string]string{
					"10.0.1.0": "10.0.0.0",
				}
				// Forge resource
				nm, err := natmappinginflater.ForgeNatMapping(clusterID1, podCIDR, externalCIDR, backedMappings)
				Expect(err).To(BeNil())

				// Create new fake client with resource
				scheme := runtime.NewScheme()
				scheme.AddKnownTypeWithName(schema.GroupVersionKind{
					Group:   "net.liqo.io",
					Version: "v1alpha1",
					Kind:    "natmappings",
				}, &liqonetapi.NatMapping{})
				var m = make(map[schema.GroupVersionResource]string)
				m[schema.GroupVersionResource{
					Group:    "net.liqo.io",
					Version:  "v1alpha1",
					Resource: "natmappings",
				}] = "natmappingsList"
				dynClient = fake.NewSimpleDynamicClientWithCustomListKinds(scheme, m, nm)

				// Create new Inflater and inject client
				inflater = natmappinginflater.NewInflater(dynClient)

				// Call func
				err = inflater.InitNatMappingsPerCluster(podCIDR, externalCIDR, clusterID1)
				Expect(err).To(BeNil())

				// Check if mappings in memory are equal to those in the resource.
				mappings, err := inflater.GetNatMappings(clusterID1)
				Expect(err).To(BeNil())
				Expect(mappings).To(Equal(backedMappings))
			})
		})
		Context("Initializing mappings more than once", func() {
			It("should return no errors", func() {
				err := inflater.InitNatMappingsPerCluster("10.0.0.0/24", "10.0.1.0/24", clusterID1)
				Expect(err).To(BeNil())
				err = inflater.InitNatMappingsPerCluster("10.0.0.0/24", "10.0.1.0/24", clusterID1)
				Expect(err).To(BeNil())
			})
		})
	})
})
