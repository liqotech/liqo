package natmappinginflater

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	liqoneterrors "github.com/liqotech/liqo/pkg/liqonet/errors"
)

var (
	inflater       *NatMappingInflater
	dynClient      dynamic.Interface
	backedMappings = map[string]string{
		"10.0.1.0": "10.0.0.0",
		"10.0.1.4": "10.0.0.2",
	}
)

const (
	invalidValue = "invalid value"
	clusterID1   = "cluster-test"
	clusterID2   = "cluster-test-2"
	clusterID3   = "cluster-test-3"
	podCIDR      = "10.0.0.0/24"
	externalCIDR = "10.0.1.0/24"
	oldIP        = "20.0.0.1"
	oldIP2       = "20.0.0.3"
	newIP        = "10.0.3.3"
	newIP2       = "10.0.3.4"
)

func setDynClient() error {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{
		Group:   "net.liqo.io",
		Version: "v1alpha1",
		Kind:    "natmappings",
	}, &netv1alpha1.NatMapping{})

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
	nm1, err := ForgeNatMapping(clusterID1, podCIDR, externalCIDR, make(map[string]string))
	if err != nil {
		return err
	}
	nm2, err := ForgeNatMapping(clusterID2, podCIDR, externalCIDR, map[string]string{})
	if err != nil {
		return err
	}

	// The following loop guarrantees resource have different names.
	for nm2.GetName() == nm1.GetName() {
		nm2, err = ForgeNatMapping(clusterID2, podCIDR, externalCIDR, make(map[string]string))
		if err != nil {
			return err
		}
	}

	dynClient = fake.NewSimpleDynamicClientWithCustomListKinds(scheme, m, nm1, nm2)
	return nil
}

var _ = Describe("NatMappingInflater", func() {
	BeforeEach(func() {
		err := setDynClient()
		Expect(err).To(BeNil())
		inflater = NewInflater(dynClient)
	})
	Describe("Re-scheduling of the inflater", func() {
		Context("If there are existing resources", func() {
			It("the inflater should recover from these resources", func() {
				By("Creating the resouces for different clusters")
				err := inflater.InitNatMappingsPerCluster(podCIDR, externalCIDR, clusterID1)
				Expect(err).To(BeNil())
				err = inflater.InitNatMappingsPerCluster(podCIDR, externalCIDR, clusterID2)
				Expect(err).To(BeNil())
				By("Populating resources with mappings")
				err = inflater.AddMapping(oldIP, newIP, clusterID1)
				Expect(err).To(BeNil())
				err = inflater.AddMapping(oldIP2, newIP2, clusterID1)
				Expect(err).To(BeNil())
				err = inflater.AddMapping(oldIP, newIP, clusterID2)
				Expect(err).To(BeNil())
				By("Simulate re-scheduling of inflater")
				inflater = NewInflater(dynClient)
				Expect(inflater.GetNatMappings(clusterID1)).To(HaveKeyWithValue(oldIP, newIP))
				Expect(inflater.GetNatMappings(clusterID1)).To(HaveKeyWithValue(oldIP2, newIP2))
				Expect(inflater.GetNatMappings(clusterID2)).To(HaveKeyWithValue(oldIP, newIP))
			})
		})
	})
	Describe("InitNatMappingsPerCluster", func() {
		Context("Passing an invalid PodCIDR", func() {
			It("should return a WrongParameter error", func() {
				err := inflater.InitNatMappingsPerCluster(invalidValue, "10.0.1.0/24", "cluster1")
				Expect(err).To(MatchError(fmt.Sprintf("%s must be %s", invalidValue, liqoneterrors.ValidCIDR)))
			})
		})
		Context("Passing an invalid ExternalCIDR", func() {
			It("should return a WrongParameter error", func() {
				err := inflater.InitNatMappingsPerCluster("10.0.1.0/24", invalidValue, "cluster1")
				Expect(err).To(MatchError(fmt.Sprintf("%s must be %s", invalidValue, liqoneterrors.ValidCIDR)))
			})
		})
		Context("Passing an empty Cluster ID", func() {
			It("should return a WrongParameter error", func() {
				err := inflater.InitNatMappingsPerCluster("10.0.1.0/24", "10.0.0.0/24", "")
				Expect(err).To(MatchError(fmt.Sprintf("ClusterID must be %s", liqoneterrors.StringNotEmpty)))
			})
		})
		Context("Passing an empty PodCIDR", func() {
			It("should return a WrongParameter error", func() {
				err := inflater.InitNatMappingsPerCluster("", "10.0.0.0/24", clusterID1)
				Expect(err).To(MatchError(fmt.Sprintf("PodCIDR must be %s", liqoneterrors.StringNotEmpty)))
			})
		})
		Context("Passing an empty ExternalCIDR", func() {
			It("should return a WrongParameter error", func() {
				err := inflater.InitNatMappingsPerCluster("10.0.1.0/24", "", clusterID1)
				Expect(err).To(MatchError(fmt.Sprintf("ExternalCIDR must be %s", liqoneterrors.StringNotEmpty)))
			})
		})
		Context("Initializing mappings", func() {
			It("should create a new resource", func() {
				// Create new fake client with resource
				scheme := runtime.NewScheme()
				scheme.AddKnownTypeWithName(schema.GroupVersionKind{
					Group:   "net.liqo.io",
					Version: "v1alpha1",
					Kind:    "natmappings",
				}, &netv1alpha1.NatMapping{})
				var m = make(map[schema.GroupVersionResource]string)
				m[schema.GroupVersionResource{
					Group:    "net.liqo.io",
					Version:  "v1alpha1",
					Resource: "natmappings",
				}] = "natmappingsList"
				dynClient = fake.NewSimpleDynamicClientWithCustomListKinds(scheme, m)

				// Create new Inflater and inject client
				inflater = NewInflater(dynClient)

				// Call func
				err := inflater.InitNatMappingsPerCluster(podCIDR, externalCIDR, clusterID1)
				Expect(err).To(BeNil())

				// Check resource has been created
				nm, err := inflater.getNatMappingResource(clusterID1)
				Expect(err).To(BeNil())
				Expect(nm.Spec.ClusterID).To(Equal(clusterID1))
				Expect(nm.Spec.PodCIDR).To(Equal(podCIDR))
				Expect(nm.Spec.ExternalCIDR).To(Equal(externalCIDR))
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
	Describe("GetNatMappings", func() {
		Context("If the cluster has not been initialized yet", func() {
			It("should return a WrongParameterError", func() {
				_, err := inflater.GetNatMappings(clusterID3)
				Expect(err).To(MatchError(fmt.Sprintf("%s for cluster %s must be %s",
					consts.NatMappingKind, clusterID3, liqoneterrors.Initialization)))
			})
		})
		Context("If the cluster has not any mapping", func() {
			It("should return an empty map", func() {
				err := inflater.InitNatMappingsPerCluster(podCIDR, externalCIDR, clusterID1)
				Expect(err).To(BeNil())
				mappings, err := inflater.GetNatMappings(clusterID1)
				Expect(err).To(BeNil())
				Expect(mappings).To(HaveLen(0))
			})
		})
		Context("If the cluster has some mappings", func() {
			It("should return the set of mappings of the cluster", func() {
				err := inflater.InitNatMappingsPerCluster(podCIDR, externalCIDR, clusterID1)
				Expect(err).To(BeNil())

				inflater.natMappingsPerCluster[clusterID1] = backedMappings

				mappings, err := inflater.GetNatMappings(clusterID1)
				Expect(err).To(BeNil())
				Expect(mappings).To(Equal(backedMappings))
			})
		})
	})
	Describe("getNatMappingResource", func() {
		Context("If resource for cluster does not exist", func() {
			It("should return a NotFoundError", func() {
				// BeforeEach creates resouce for clusterID1 and clusterID2
				_, err := inflater.getNatMappingResource(clusterID3)
				Expect(errors.IsNotFound(err)).To(BeTrue())
			})
		})
		Context("If multiple resources exist for the same cluster", func() {
			It("should delete all resource except one", func() {
				// Create new fake client with resource
				scheme := runtime.NewScheme()
				scheme.AddKnownTypeWithName(schema.GroupVersionKind{
					Group:   "net.liqo.io",
					Version: "v1alpha1",
					Kind:    "natmappings",
				}, &netv1alpha1.NatMapping{})
				var m = make(map[schema.GroupVersionResource]string)
				m[schema.GroupVersionResource{
					Group:    "net.liqo.io",
					Version:  "v1alpha1",
					Resource: "natmappings",
				}] = "natmappingsList"

				nm1, err := ForgeNatMapping(clusterID1, podCIDR, externalCIDR, backedMappings)
				Expect(err).To(BeNil())
				nm2, err := ForgeNatMapping(clusterID1, podCIDR, externalCIDR, backedMappings)
				Expect(err).To(BeNil())
				nm3, err := ForgeNatMapping(clusterID1, podCIDR, externalCIDR, backedMappings)
				Expect(err).To(BeNil())
				nm4, err := ForgeNatMapping(clusterID1, podCIDR, externalCIDR, backedMappings)
				Expect(err).To(BeNil())
				dynClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, m, nm1, nm2, nm3, nm4)

				inflater.dynClient = dynClient

				_, err = inflater.getNatMappingResource(clusterID1)
				Expect(err).To(BeNil())

				// Check if there is only one resouce
				list, err := inflater.dynClient.
					Resource(netv1alpha1.NatMappingGroupResource).
					List(context.Background(), metav1.ListOptions{
						LabelSelector: fmt.Sprintf("%s=%s,%s=%s",
							consts.NatMappingResourceLabelKey,
							consts.NatMappingResourceLabelValue,
							consts.ClusterIDLabelName, clusterID1),
					})
				Expect(err).To(BeNil())
				Expect(list.Items).To(HaveLen(1))
			})
		})
		Context("If a resource for the remote cluster exists", func() {
			It("should return that resource", func() {
				// BeforeEach creates resources for clusterID1 and clusterID2
				nm, err := inflater.getNatMappingResource(clusterID1)
				Expect(err).To(BeNil())

				Expect(nm.Spec.ClusterID).To(Equal(clusterID1))
				Expect(nm.Spec.PodCIDR).To(Equal(podCIDR))
				Expect(nm.Spec.ExternalCIDR).To(Equal(externalCIDR))
			})
		})
	})
	Describe("AddMapping", func() {
		Context("Call func without initializing NAT mappings", func() {
			It("should return a MissingInit error", func() {
				err := inflater.AddMapping(oldIP, newIP, clusterID3)
				Expect(err).To(MatchError(fmt.Sprintf("%s for cluster %s must be %s", consts.NatMappingKind, clusterID3, liqoneterrors.Initialization)))
			})
		})
		Context("Call func after correct initialization", func() {
			It("should successfully add the mapping", func() {
				// Init
				err := inflater.InitNatMappingsPerCluster(podCIDR, externalCIDR, clusterID1)
				Expect(err).To(BeNil())

				err = inflater.AddMapping(oldIP, newIP, clusterID1)
				Expect(err).To(BeNil())
				mappings, err := inflater.GetNatMappings(clusterID1)
				Expect(mappings).To(HaveKeyWithValue(oldIP, newIP))
			})
		})
		Context("Call func twice with same parameters", func() {
			It("second call should be a nop", func() {
				// Init
				err := inflater.InitNatMappingsPerCluster(podCIDR, externalCIDR, clusterID1)
				Expect(err).To(BeNil())

				err = inflater.AddMapping(oldIP, newIP, clusterID1)
				Expect(err).To(BeNil())

				// Check config before second call
				nm, err := inflater.getNatMappingResource(clusterID1)
				Expect(err).To(BeNil())
				Expect(nm.Spec.ClusterMappings).To(HaveKeyWithValue(oldIP, newIP))

				err = inflater.AddMapping(oldIP, newIP, clusterID1)
				Expect(err).To(BeNil())

				// Check config after
				newNm, err := inflater.getNatMappingResource(clusterID1)
				Expect(err).To(BeNil())
				Expect(newNm).To(Equal(nm))
			})
		})
		Context("Call func twice with different new IP", func() {
			It("should return no errors and update the mapping", func() {
				// Init
				err := inflater.InitNatMappingsPerCluster(podCIDR, externalCIDR, clusterID1)
				Expect(err).To(BeNil())

				err = inflater.AddMapping(oldIP, newIP, clusterID1)
				Expect(err).To(BeNil())

				err = inflater.AddMapping(oldIP, newIP2, clusterID1)
				Expect(err).To(BeNil())

				// Check if inflater has been updated successfully
				mappings, err := inflater.GetNatMappings(clusterID1)
				Expect(mappings).To(HaveKeyWithValue(oldIP, newIP2))

				// Check resource
				nm, err := inflater.getNatMappingResource(clusterID1)
				Expect(err).To(BeNil())
				Expect(nm.Spec.ClusterMappings).To(HaveKeyWithValue(oldIP, newIP2))
			})
		})
	})
	Describe("RemoveMapping", func() {
		Context("Call func without initializing NAT mappings", func() {
			It("should return a MissingInit error", func() {
				err := inflater.RemoveMapping(oldIP, clusterID3)
				Expect(err).To(MatchError(fmt.Sprintf("%s for cluster %s must be %s", consts.NatMappingKind, clusterID3, liqoneterrors.Initialization)))
			})
		})
		Context("Call func after correct initialization", func() {
			It("should successfully remove the mapping", func() {
				// Init
				err := inflater.InitNatMappingsPerCluster(podCIDR, externalCIDR, clusterID1)
				Expect(err).To(BeNil())

				// Add mapping
				err = inflater.AddMapping(oldIP, newIP, clusterID1)
				Expect(err).To(BeNil())

				// Remove mapping
				err = inflater.RemoveMapping(oldIP, clusterID1)
				Expect(err).To(BeNil())

				// Check if removed successfully
				nm, err := inflater.getNatMappingResource(clusterID1)
				Expect(err).To(BeNil())
				Expect(nm.Spec.ClusterMappings).ToNot(HaveKeyWithValue(oldIP, newIP))
			})
		})
		Context("Call func twice", func() {
			It("second call should be a nop", func() {
				// Init
				err := inflater.InitNatMappingsPerCluster(podCIDR, externalCIDR, clusterID1)
				Expect(err).To(BeNil())

				// Add mapping
				err = inflater.AddMapping(oldIP, newIP, clusterID1)
				Expect(err).To(BeNil())

				// Remove mapping
				err = inflater.RemoveMapping(oldIP, clusterID1)
				Expect(err).To(BeNil())

				// Check config before second call
				nm, err := inflater.getNatMappingResource(clusterID1)
				Expect(err).To(BeNil())

				// Remove mapping for the second time
				err = inflater.RemoveMapping(oldIP, clusterID1)
				Expect(err).To(BeNil())

				// Check config after
				newNm, err := inflater.getNatMappingResource(clusterID1)
				Expect(err).To(BeNil())
				Expect(newNm).To(Equal(nm))
			})
		})
	})
})
