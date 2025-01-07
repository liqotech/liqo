// Copyright 2019-2025 The Liqo Authors
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

package testutil

import (
	"net"
	"os"

	"github.com/miekg/dns"
	"k8s.io/klog/v2"
)

type DnsServer struct {
	dnsServer      dns.Server
	registryDomain string
	ptrQueries     map[string][]string
	hasCname       bool
}

func (s *DnsServer) Serve() {
	s.registryDomain = "test.liqo.io."
	s.ptrQueries = map[string][]string{
		s.registryDomain: {
			"myliqo1." + s.registryDomain,
			"myliqo2." + s.registryDomain,
		},
	}

	s.dnsServer = dns.Server{
		Addr: "127.0.0.1:8053",
		Net:  "udp",
	}
	s.dnsServer.Handler = s

	c := make(chan struct{})
	s.dnsServer.NotifyStartedFunc = func() {
		close(c)
	}

	go func() {
		if err := s.dnsServer.ListenAndServe(); err != nil {
			klog.Fatal("Failed to set udp listener ", err.Error())
		}
	}()

	<-c
}

func (s *DnsServer) Shutdown() {
	err := s.dnsServer.Shutdown()
	if err != nil {
		klog.Fatal(err)
	}
}

func (s *DnsServer) GetAddr() string {
	return s.dnsServer.Addr
}

func (s *DnsServer) GetName() string {
	return s.registryDomain
}

func (s *DnsServer) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	msg := dns.Msg{}
	msg.SetReply(r)
	msg.Authoritative = true
	domain := msg.Question[0].Name
	switch r.Question[0].Qtype {
	case dns.TypePTR:
		addresses, ok := s.ptrQueries[domain]
		if ok {
			for _, address := range addresses {
				msg.Answer = append(msg.Answer, &dns.PTR{
					Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypePTR, Class: dns.ClassINET, Ttl: 60},
					Ptr: address,
				})
			}
		}
	case dns.TypeSRV:
		var port int
		var host string
		if domain == s.ptrQueries[s.registryDomain][0] {
			port = 1234
			host = "h1." + s.registryDomain
		} else if domain == s.ptrQueries[s.registryDomain][1] {
			port = 4321
			host = "h2." + s.registryDomain
		}
		msg.Answer = append(msg.Answer, &dns.SRV{
			Hdr:      dns.RR_Header{Name: domain, Rrtype: dns.TypeSRV, Class: dns.ClassINET, Ttl: 60},
			Priority: 0,
			Weight:   0,
			Port:     uint16(port),
			Target:   host,
		})
	case dns.TypeA:
		var host string
		if domain == "h1."+s.registryDomain {
			host = "1.2.3.4"
		} else if domain == "h2."+s.registryDomain {
			host = "4.3.2.1"
		}
		if !s.hasCname {
			msg.Answer = append(msg.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
				A:   net.ParseIP(host),
			})
		}
	case dns.TypeCNAME:
		if s.hasCname {
			msg.Answer = append(msg.Answer, &dns.CNAME{
				Hdr:    dns.RR_Header{Name: domain, Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 60},
				Target: "cname.test.liqo.io.",
			})
		}
	}
	err := w.WriteMsg(&msg)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}
}
