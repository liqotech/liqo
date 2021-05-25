package liqonet_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/liqotech/liqo/pkg/liqonet"
)

var inflater *liqonet.NatMappingInflater

var _ = Describe("NatMappingInflater", func() {
	BeforeEach(func() {
		err := setDynClient()
		Expect(err).To(BeNil())
		inflater = liqonet.NewInflater(dynClient)
	})
	Describe("InitMapping", func() {
		Context("Initializing mappings more than once", func() {
			It("should return no errors", func() {
				err := inflater.InitNatMappings("10.0.0.0/24", "10.0.1.0/24", "cluster1")
				Expect(err).To(BeNil())
				err = inflater.InitNatMappings("10.0.0.0/24", "10.0.1.0/24", "cluster1")
				Expect(err).To(BeNil())
			})
		})
	})
	Describe("TerminateNatMapping", func() {
		Context("Terminate mappings more than once", func() {
			It("should return no errors", func() {
				// Init
				err := inflater.InitNatMappings("10.0.0.0/24", "10.0.1.0/24", "cluster1")
				Expect(err).To(BeNil())
				// Terminate twice
				err = inflater.TerminateNatMappings("cluster1")
				Expect(err).To(BeNil())
				err = inflater.TerminateNatMappings("cluster1")
				Expect(err).To(BeNil())
			})
		})
	})
	Describe("AddMapping", func() {
		Context("Call func without initializing NAT mappings", func() {
			It("should return an error", func() {
				err := inflater.AddMapping("10.0.0.1", "192.168.0.1", "cluster3")
				Expect(err).ToNot(BeNil())
			})
		})
		Context("Call func after correct initialization", func() {
			It("should successfully add the mapping", func() {
				// Init
				err := inflater.InitNatMappings("10.0.0.0/24", "10.0.1.0/24", "cluster1")
				Expect(err).To(BeNil())

				err = inflater.AddMapping("10.0.0.1", "192.168.0.1", "cluster1")
				Expect(err).To(BeNil())
				mappings, err := inflater.GetNatMappings("cluster1")
				Expect(mappings).To(HaveKeyWithValue("10.0.0.1", "192.168.0.1"))
			})
		})
		Context("Call func twice with same parameters", func() {
			It("should return no errors", func() {
				// Init
				err := inflater.InitNatMappings("10.0.0.0/24", "10.0.1.0/24", "cluster1")
				Expect(err).To(BeNil())

				err = inflater.AddMapping("10.0.0.1", "192.168.0.1", "cluster1")
				Expect(err).To(BeNil())
				err = inflater.AddMapping("10.0.0.1", "192.168.0.1", "cluster1")
				Expect(err).To(BeNil())
			})
		})
		Context("Call func twice with different new IP", func() {
			It("should return no errors and update the mapping", func() {
				// Init
				err := inflater.InitNatMappings("10.0.0.0/24", "10.0.1.0/24", "cluster1")
				Expect(err).To(BeNil())

				err = inflater.AddMapping("10.0.0.1", "192.168.0.1", "cluster1")
				Expect(err).To(BeNil())

				err = inflater.AddMapping("10.0.0.1", "192.168.0.2", "cluster1")
				Expect(err).To(BeNil())

				// Check if updated successfully
				mappings, err := inflater.GetNatMappings("cluster1")
				Expect(mappings).To(HaveKeyWithValue("10.0.0.1", "192.168.0.2"))
			})
		})
	})
	Describe("RemoveMapping", func() {
		Context("Call func without initializing NAT mappings", func() {
			It("should return an error", func() {
				err := inflater.RemoveMapping("10.0.0.1", "cluster3")
				Expect(err).ToNot(BeNil())
			})
		})
		Context("Call func after correct initialization", func() {
			It("should successfully remove the mapping", func() {
				// Init
				err := inflater.InitNatMappings("10.0.0.0/24", "10.0.1.0/24", "cluster1")
				Expect(err).To(BeNil())

				// Add mapping
				err = inflater.AddMapping("10.0.0.1", "192.168.0.1", "cluster1")
				Expect(err).To(BeNil())

				// Remove mapping
				err = inflater.RemoveMapping("10.0.0.1", "cluster1")
				Expect(err).To(BeNil())

				// Check if removed successfully
				mappings, err := inflater.GetNatMappings("cluster1")
				Expect(mappings).ToNot(HaveKeyWithValue("10.0.0.1", "192.168.0.1"))
			})
		})
		Context("Call func twice", func() {
			It("should return no errors", func() {
				// Init
				err := inflater.InitNatMappings("10.0.0.0/24", "10.0.1.0/24", "cluster1")
				Expect(err).To(BeNil())

				// Add mapping
				err = inflater.AddMapping("10.0.0.1", "192.168.0.1", "cluster1")
				Expect(err).To(BeNil())

				// Remove mapping
				err = inflater.RemoveMapping("10.0.0.1", "cluster1")
				Expect(err).To(BeNil())

				// Remove mapping for the second time
				err = inflater.RemoveMapping("10.0.0.1", "cluster1")
				Expect(err).To(BeNil())
			})
		})
	})
})
