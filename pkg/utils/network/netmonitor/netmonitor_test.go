// Copyright 2019-2026 The Liqo Authors
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

//go:build linux

package netmonitor

import (
	"context"
	"net"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestNetmonitor(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Netmonitor Suite")
}

var _ = Describe("Link monitoring", func() {
	var (
		eventCh chan event.GenericEvent
		errCh   chan error
		ctx     context.Context
		cancel  context.CancelFunc
	)

	BeforeEach(func() {
		eventCh = make(chan event.GenericEvent, 10)
		errCh = make(chan error, 1)
		ctx, cancel = context.WithCancel(context.Background())
	})

	AfterEach(func() {
		cancel()
	})

	It("should detect a real link creation and deletion", func() {
		go func() {
			errCh <- InterfacesMonitoring(ctx, eventCh, &Options{
				Link: &OptionsLink{Create: true, Delete: true},
			})
		}()
		time.Sleep(200 * time.Millisecond)

		link := &netlink.Veth{
			LinkAttrs: netlink.LinkAttrs{Name: "liqo-test-veth0"},
			PeerName:  "liqo-test-veth1",
		}
		DeferCleanup(func() {
			_ = netlink.LinkDel(link)
		})

		Expect(netlink.LinkAdd(link)).To(Succeed())
		Eventually(eventCh, "2s").Should(Receive())

		Expect(netlink.LinkDel(link)).To(Succeed())
		Eventually(eventCh, "2s").Should(Receive())

		cancel()
		Eventually(errCh, "2s").Should(Receive(BeNil()))
	})
})

var _ = Describe("Address monitoring", func() {
	var (
		eventCh chan event.GenericEvent
		errCh   chan error
		ctx     context.Context
		cancel  context.CancelFunc
		link    netlink.Link
	)

	BeforeEach(func() {
		eventCh = make(chan event.GenericEvent, 10)
		errCh = make(chan error, 1)
		ctx, cancel = context.WithCancel(context.Background())
	})

	AfterEach(func() {
		cancel()
		if link != nil {
			_ = netlink.LinkDel(link)
		}
	})

	It("should detect a real address addition and deletion", func() {
		go func() {
			errCh <- InterfacesMonitoring(ctx, eventCh, &Options{
				Addr: &OptionsAddr{Create: true, Delete: true},
			})
		}()
		time.Sleep(200 * time.Millisecond)

		link = &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "liqo-test-addr0"}}
		Expect(netlink.LinkAdd(link)).To(Succeed())
		link, err := netlink.LinkByName("liqo-test-addr0")
		Expect(err).ToNot(HaveOccurred())

		addr := &netlink.Addr{
			IPNet: &net.IPNet{IP: net.ParseIP("10.200.0.1"), Mask: net.CIDRMask(24, 32)},
		}
		Expect(netlink.AddrAdd(link, addr)).To(Succeed())
		Eventually(eventCh, "2s").Should(Receive())

		Expect(netlink.AddrDel(link, addr)).To(Succeed())
		Eventually(eventCh, "2s").Should(Receive())

		cancel()
		Eventually(errCh, "2s").Should(Receive(BeNil()))
	})
})

var _ = Describe("Route monitoring", func() {
	var (
		eventCh chan event.GenericEvent
		errCh   chan error
		ctx     context.Context
		cancel  context.CancelFunc
	)

	BeforeEach(func() {
		eventCh = make(chan event.GenericEvent, 10)
		errCh = make(chan error, 1)
		ctx, cancel = context.WithCancel(context.Background())
	})

	AfterEach(func() {
		cancel()
	})

	It("should detect a real route addition and deletion", func() {
		go func() {
			errCh <- InterfacesMonitoring(ctx, eventCh, &Options{
				Route: &OptionsRoute{Create: true, Delete: true},
			})
		}()
		time.Sleep(200 * time.Millisecond)

		route := &netlink.Route{
			Dst: &net.IPNet{IP: net.ParseIP("192.0.2.0"), Mask: net.CIDRMask(24, 32)},
			Gw:  net.ParseIP("127.0.0.1"),
		}
		DeferCleanup(func() {
			_ = netlink.RouteDel(route)
		})

		Expect(netlink.RouteAdd(route)).To(Succeed())
		Eventually(eventCh, "2s").Should(Receive())

		Expect(netlink.RouteDel(route)).To(Succeed())
		Eventually(eventCh, "2s").Should(Receive())

		cancel()
		Eventually(errCh, "2s").Should(Receive(BeNil()))
	})
})

var _ = Describe("Rule monitoring", func() {
	var (
		eventCh chan event.GenericEvent
		errCh   chan error
		ctx     context.Context
		cancel  context.CancelFunc
	)

	BeforeEach(func() {
		eventCh = make(chan event.GenericEvent, 10)
		errCh = make(chan error, 1)
		ctx, cancel = context.WithCancel(context.Background())
	})

	AfterEach(func() {
		cancel()
	})

	It("should detect a real IP rule addition and deletion", func() {
		go func() {
			errCh <- InterfacesMonitoring(ctx, eventCh, &Options{
				Rule: &OptionsRule{Create: true, Delete: true},
			})
		}()
		time.Sleep(200 * time.Millisecond)

		rule := netlink.NewRule()
		rule.Priority = 32765
		rule.Table = 254
		rule.Dst = &net.IPNet{IP: net.ParseIP("198.51.100.0"), Mask: net.CIDRMask(24, 32)}

		DeferCleanup(func() {
			_ = netlink.RuleDel(rule)
		})

		Expect(netlink.RuleAdd(rule)).To(Succeed())
		Eventually(eventCh, "2s").Should(Receive())

		Expect(netlink.RuleDel(rule)).To(Succeed())
		Eventually(eventCh, "2s").Should(Receive())

		cancel()
		Eventually(errCh, "2s").Should(Receive(BeNil()))
	})
})

var _ = Describe("InterfacesMonitoring", func() {
	It("should return an error when options are nil", func() {
		err := InterfacesMonitoring(context.Background(), make(chan event.GenericEvent), nil)
		Expect(err).To(HaveOccurred())
	})
})
