// Copyright 2019-2022 The Liqo Authors
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

package searchdomainoperator

import (
	"errors"
	"net"
	"time"

	"github.com/miekg/dns"
	"k8s.io/klog/v2"

	discovery "github.com/liqotech/liqo/pkg/discoverymanager"
)

// loadAuthDataFromDNS loads a list of foreign AuthServices given a DNS domain name.
// These foreign services have to be added in a PTR record for that domain name,
// for example:
// liqo.mycompany.com		myliqo1.mycompany.com, myliqo2.mycompany.com
// can be 2 different clusters registered on a company domain.
// For each cluster than we have to have a SRV record that specify the port where to contact that cluster.
func loadAuthDataFromDNS(dnsAddr, name string) ([]*discovery.AuthData, error) {
	authData := []*discovery.AuthData{}

	if dnsAddr == "" {
		clientConfig, err := dns.ClientConfigFromFile("/etc/resolv.conf")
		if err != nil {
			klog.Error(err)
			return nil, err
		}
		if len(clientConfig.Servers) == 0 {
			err = errors.New("no DNS server config found")
			klog.Error(err)
			return nil, err
		}
		dnsAddr = net.JoinHostPort(clientConfig.Servers[0], "53")
	}

	c := new(dns.Client)
	c.DialTimeout = 30 * time.Second

	// PTR query
	msg := getDNSMsg(name, dns.TypePTR)
	in, _, err := c.Exchange(msg, dnsAddr)
	if err != nil {
		klog.Error(err, err.Error())
		return nil, err
	}

	for _, ans := range in.Answer {
		ptr, ok := ans.(*dns.PTR)
		if !ok {
			klog.Warning("Not PTR record: ", ans)
			continue
		}
		aData, err := resolveWan(c, dnsAddr, ptr)
		if err != nil {
			klog.Error(err, err.Error())
			return nil, err
		}
		authData = append(authData, aData)
	}
	return authData, nil
}

func resolveWan(c *dns.Client, dnsAddr string, ptr *dns.PTR) (*discovery.AuthData, error) {
	// SRV query
	msg := getDNSMsg(ptr.Ptr, dns.TypeSRV)
	in, _, err := c.Exchange(msg, dnsAddr)
	if err != nil {
		klog.Error(err, err.Error())
		return nil, err
	}
	if len(in.Answer) == 0 {
		klog.Error("SRV record is not set for " + ptr.Ptr)
		return nil, errors.New("SRV record is not set for " + ptr.Ptr)
	}
	srv := in.Answer[0].(*dns.SRV)

	return discovery.NewAuthData(srv.Target, int(srv.Port), srv.Hdr.Ttl), nil
}

func getDNSMsg(name string, qType uint16) *dns.Msg {
	msg := new(dns.Msg)
	msg.Id = dns.Id()
	msg.RecursionDesired = true
	msg.Question = make([]dns.Question, 1)
	msg.Question[0] = dns.Question{
		Name:   dns.Fqdn(name),
		Qtype:  qType,
		Qclass: dns.ClassINET,
	}
	return msg
}
