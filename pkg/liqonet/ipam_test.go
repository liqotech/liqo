package liqonet_test

import (
	"github.com/liqotech/liqo/pkg/liqonet"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
)

var ipam *liqonet.IPAM

var _ = Describe("IpamNew", func() {
	Describe("After reserving a network", func() {
		Context("That belongs to a pool", func() {
			BeforeEach(func() {
				reserved := []string{
					"10.244.0.0/24",
				}
				clusterSubnet := make(map[string]string)
				ipam = liqonet.NewIPAM()
				err := ipam.Init(reserved, liqonet.Pools, clusterSubnet)
				gomega.Expect(err).To(gomega.BeNil())
				err = ipam.AcquireReservedSubnet("10.0.2.0/24")
				gomega.Expect(err).To(gomega.BeNil())
			})
			It("Should not be possible to acquire the same network for a cluster", func() {
				p, err := ipam.GetSubnetPerCluster("10.0.2.0/24", "cluster1")
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(p).ToNot(gomega.Equal("10.0.2.0/24"))
			})
			It("Should not be possible to acquire a larger network that contains it for a cluster", func() {
				p, err := ipam.GetSubnetPerCluster("10.0.0.0/16", "cluster1")
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(p).ToNot(gomega.Equal("10.0.0.0/16"))
			})
			It("Should not be possible to acquire a smaller network contained by it for a cluster", func() {
				p, err := ipam.GetSubnetPerCluster("10.0.2.0/25", "cluster1")
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(p).ToNot(gomega.Equal("10.0.2.0/25"))
			})
		})
	})
	Describe("Allocating a new network for a cluster", func() {
		Context("When the remote cluster asks for a subnet not belonging to any pool", func() {
			Context("and the subnet has not already been assigned to any other cluster", func() {
				BeforeEach(func() {
					pool := []string{
						"10.0.0.0/8",
					}
					reserved := []string{
						"192.168.1.0/24",
					}
					clusterSubnet := make(map[string]string)
					ipam = liqonet.NewIPAM()
					err := ipam.Init(reserved, pool, clusterSubnet)
					gomega.Expect(err).To(gomega.BeNil())
				})
				It("Should allocate the subnet itself, without mapping", func() {
					_, err := ipam.GetSubnetPerCluster("11.0.0.0/16", "cluster1")
					gomega.Expect(err).To(gomega.BeNil())
					gomega.Expect(ipam.SubnetPerCluster["cluster1"]).To(gomega.Equal("11.0.0.0/16"))
				})
			})
			Context("and the subnet has already been assigned to another cluster", func() {
				Context("and there is an available network with the same mask length in one pool", func() {
					BeforeEach(func() {
						pool := []string{
							"10.0.0.0/8",
						}
						reserved := []string{
							"192.168.1.0/24",
						}
						clusterSubnet := make(map[string]string)
						ipam = liqonet.NewIPAM()
						err := ipam.Init(reserved, pool, clusterSubnet)
						gomega.Expect(err).To(gomega.BeNil())
						_, err = ipam.GetSubnetPerCluster("11.0.0.0/16", "cluster1")
						gomega.Expect(err).To(gomega.BeNil())
						gomega.Expect(ipam.SubnetPerCluster["cluster1"]).To(gomega.Equal("11.0.0.0/16"))
					})
					It("should map the requested network to another network taken by the pool", func() {
						_, err := ipam.GetSubnetPerCluster("11.0.0.0/16", "cluster2")
						gomega.Expect(err).To(gomega.BeNil())
						gomega.Expect(ipam.SubnetPerCluster["cluster2"]).To(gomega.HavePrefix("10."))
						gomega.Expect(ipam.SubnetPerCluster["cluster2"]).To(gomega.HaveSuffix("/16"))
					})
				})
				Context("and there is not an available network with the same mask length in any pool", func() {
					BeforeEach(func() {
						pool := []string{
							"10.0.0.0/8",
						}
						reserved := []string{
							"192.168.1.0/24",
						}
						clusterSubnet := make(map[string]string)
						ipam = liqonet.NewIPAM()
						err := ipam.Init(reserved, pool, clusterSubnet)
						gomega.Expect(err).To(gomega.BeNil())
					})
					It("should fail to allocate a network", func() {
						_, err := ipam.GetSubnetPerCluster("11.0.0.0/16", "cluster1")
						gomega.Expect(err).To(gomega.BeNil())
						gomega.Expect(ipam.SubnetPerCluster["cluster1"]).To(gomega.Equal("11.0.0.0/16"))
						_, err = ipam.GetSubnetPerCluster("10.0.0.0/9", "cluster2")
						gomega.Expect(err).To(gomega.BeNil())
						gomega.Expect(ipam.SubnetPerCluster["cluster2"]).To(gomega.Equal("10.0.0.0/9"))
						_, err = ipam.GetSubnetPerCluster("10.128.0.0/9", "cluster3")
						gomega.Expect(err).To(gomega.BeNil())
						gomega.Expect(ipam.SubnetPerCluster["cluster3"]).To(gomega.Equal("10.128.0.0/9"))
						_, err = ipam.GetSubnetPerCluster("11.0.0.0/16", "cluster4")
						gomega.Expect(err).ToNot(gomega.BeNil())
					})
				})
			})
		})
		Context("When the remote clusters asks for a subnet which is equal to a pool", func() {
			BeforeEach(func() {
				var ipam *liqonet.IPAM
				pool := []string{
					"10.0.0.0/8",
					"172.16.0.0/12",
				}
				reserved := []string{
					"192.168.1.0/24",
				}
				clusterSubnet := make(map[string]string)
				ipam = liqonet.NewIPAM()
				err := ipam.Init(reserved, pool, clusterSubnet)
				gomega.Expect(err).To(gomega.BeNil())
			})
			It("should map it to another network", func() {
				network, err := ipam.GetSubnetPerCluster("172.16.0.0/12", "cluster1")
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(network).ToNot(gomega.Equal("172.16.0.0/12"))
			})
		})
		Context("When the remote cluster asks for a subnet belonging to a network in the pool", func() {
			Context("and all pools are full", func() {
				BeforeEach(func() {
					pool := []string{
						"10.0.0.0/8",
					}
					reserved := []string{
						"192.168.1.0/24",
					}
					clusterSubnet := make(map[string]string)
					ipam = liqonet.NewIPAM()
					err := ipam.Init(reserved, pool, clusterSubnet)
					gomega.Expect(err).To(gomega.BeNil())
				})
				It("should not allocate the network", func() {
					_, err := ipam.GetSubnetPerCluster("10.0.0.0/9", "cluster1")
					gomega.Expect(err).To(gomega.BeNil())
					gomega.Expect(ipam.SubnetPerCluster["cluster1"]).To(gomega.Equal("10.0.0.0/9"))
					_, err = ipam.GetSubnetPerCluster("10.128.0.0/9", "cluster2")
					gomega.Expect(err).To(gomega.BeNil())
					gomega.Expect(ipam.SubnetPerCluster["cluster2"]).To(gomega.Equal("10.128.0.0/9"))
					_, err = ipam.GetSubnetPerCluster("10.1.0.0/16", "cluster3")
					gomega.Expect(err).ToNot(gomega.BeNil())
				})
			})
			Context("and the subnet has not already been assigned to any other cluster", func() {
				BeforeEach(func() {
					pool := []string{
						"10.0.0.0/8",
					}
					reserved := []string{
						"192.168.1.0/24",
					}
					clusterSubnet := make(map[string]string)
					ipam = liqonet.NewIPAM()
					err := ipam.Init(reserved, pool, clusterSubnet)
					gomega.Expect(err).To(gomega.BeNil())
				})
				It("Should allocate the subnet itself, without mapping", func() {
					_, err := ipam.GetSubnetPerCluster("10.0.0.0/16", "cluster1")
					gomega.Expect(err).To(gomega.BeNil())
					gomega.Expect(ipam.SubnetPerCluster["cluster1"]).To(gomega.Equal("10.0.0.0/16"))
				})
			})
			Context("and the subnet has already been assigned to another cluster", func() {
				Context("and there is an available network with the same mask length in one pool", func() {
					BeforeEach(func() {
						pool := []string{
							"10.0.0.0/8",
						}
						reserved := []string{
							"192.168.1.0/24",
						}
						clusterSubnet := make(map[string]string)
						ipam = liqonet.NewIPAM()
						err := ipam.Init(reserved, pool, clusterSubnet)
						gomega.Expect(err).To(gomega.BeNil())
					})
					It("should map the requested network to another network taken by the pool", func() {
						_, err := ipam.GetSubnetPerCluster("10.0.0.0/16", "cluster1")
						gomega.Expect(err).To(gomega.BeNil())
						gomega.Expect(ipam.SubnetPerCluster["cluster1"]).To(gomega.Equal("10.0.0.0/16"))
						_, err = ipam.GetSubnetPerCluster("10.0.0.0/16", "cluster2")
						gomega.Expect(err).To(gomega.BeNil())
						gomega.Expect(ipam.SubnetPerCluster["cluster2"]).To(gomega.HavePrefix("10."))
						gomega.Expect(ipam.SubnetPerCluster["cluster2"]).To(gomega.HaveSuffix("/16"))
					})
				})
				Context("and there is not an available network with the same mask length in any pool", func() {
					BeforeEach(func() {
						pool := []string{
							"10.0.0.0/8",
						}
						reserved := []string{
							"192.168.1.0/24",
						}
						clusterSubnet := make(map[string]string)
						ipam = liqonet.NewIPAM()
						err := ipam.Init(reserved, pool, clusterSubnet)
						gomega.Expect(err).To(gomega.BeNil())
					})
					It("should allocate it as a new prefix", func() {
						_, err := ipam.GetSubnetPerCluster("10.0.0.0/16", "cluster1")
						gomega.Expect(err).To(gomega.BeNil())
						gomega.Expect(ipam.SubnetPerCluster["cluster1"]).To(gomega.Equal("10.0.0.0/16"))
						_, err = ipam.GetSubnetPerCluster("10.0.0.0/16", "cluster2")
						gomega.Expect(err).To(gomega.BeNil())
					})
				})
			})
		})
	})
	Describe("Reserving a subnet", func() {
		Context("which is not already reserved", func() {
			Context("and does not belong to any pool", func() {
				BeforeEach(func() {
					pool := []string{
						"10.0.0.0/8",
					}
					reserved := []string{
						"192.168.1.0/24",
					}
					clusterSubnet := make(map[string]string)
					ipam = liqonet.NewIPAM()
					err := ipam.Init(reserved, pool, clusterSubnet)
					gomega.Expect(err).To(gomega.BeNil())
				})
				It("should successfully reserve it", func() {
					err := ipam.AcquireReservedSubnet("172.16.0.0/24")
					gomega.Expect(err).To(gomega.BeNil())
				})
			})
			Context("and belongs to a pool", func() {
				BeforeEach(func() {
					pool := []string{
						"10.0.0.0/8",
					}
					reserved := []string{
						"192.168.1.0/24",
					}
					clusterSubnet := make(map[string]string)
					ipam = liqonet.NewIPAM()
					err := ipam.Init(reserved, pool, clusterSubnet)
					gomega.Expect(err).To(gomega.BeNil())
				})
				It("should successfully reserve it", func() {
					err := ipam.AcquireReservedSubnet("10.0.3.0/24")
					gomega.Expect(err).To(gomega.BeNil())
				})
			})
			Context("and is equal to a pool", func() {
				BeforeEach(func() {
					pool := []string{
						"10.0.0.0/8",
					}
					reserved := []string{
						"192.168.1.0/24",
					}
					clusterSubnet := make(map[string]string)
					ipam = liqonet.NewIPAM()
					err := ipam.Init(reserved, pool, clusterSubnet)
					gomega.Expect(err).To(gomega.BeNil())
				})
				It("should not reserve it", func() {
					err := ipam.AcquireReservedSubnet("10.0.0.0/8")
					gomega.Expect(err).ToNot(gomega.BeNil())
				})
			})
		})
		Context("which is already reserved", func() {
			BeforeEach(func() {
				pool := []string{
					"10.0.0.0/8",
				}
				reserved := []string{
					"192.168.1.0/24",
				}
				clusterSubnet := make(map[string]string)
				ipam = liqonet.NewIPAM()
				err := ipam.Init(reserved, pool, clusterSubnet)
				gomega.Expect(err).To(gomega.BeNil())
			})
			It("should not reserve it and return no errors", func() {
				err := ipam.AcquireReservedSubnet("10.0.0.0/16")
				gomega.Expect(err).To(gomega.BeNil())
				err = ipam.AcquireReservedSubnet("10.0.0.0/16")
				gomega.Expect(err).To(gomega.BeNil())
			})
		})
	})
	Describe("Removing a subnet", func() {
		BeforeEach(func() {
			pool := []string{
				"10.0.0.0/8",
			}
			reserved := []string{
				"192.168.1.0/16",
			}
			clusterSubnet := make(map[string]string)
			ipam = liqonet.NewIPAM()
			err := ipam.Init(reserved, pool, clusterSubnet)
			gomega.Expect(err).To(gomega.BeNil())
		})
		It("Should successfully free the subnet", func() {
			network, err := ipam.GetSubnetPerCluster("10.0.1.0/24", "cluster1")
			gomega.Expect(err).To(gomega.BeNil())
			gomega.Expect(network).To(gomega.Equal("10.0.1.0/24"))
			err = ipam.FreeSubnetPerCluster("cluster1")
			gomega.Expect(err).To(gomega.BeNil())
			network, err = ipam.GetSubnetPerCluster("10.0.1.0/24", "cluster2")
			gomega.Expect(err).To(gomega.BeNil())
			gomega.Expect(network).To(gomega.Equal("10.0.1.0/24"))
		})
	})
})
