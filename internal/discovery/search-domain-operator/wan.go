package search_domain_operator

import (
	"errors"
	"github.com/liqotech/liqo/internal/discovery"
	"github.com/miekg/dns"
	"k8s.io/klog"
	"net"
	"strconv"
	"time"
)

func Wan(dnsAddr string, name string) ([]*discovery.TxtData, error) {
	txtData := []*discovery.TxtData{}

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
	msg := GetDnsMsg(name, dns.TypePTR)
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
		txt, err := ResolveWan(c, dnsAddr, ptr)
		if err != nil {
			klog.Error(err, err.Error())
			return nil, err
		}
		txtData = append(txtData, txt)
	}
	return txtData, nil
}

func ResolveWan(c *dns.Client, dnsAddr string, ptr *dns.PTR) (*discovery.TxtData, error) {
	// SRV query
	msg := GetDnsMsg(ptr.Ptr, dns.TypeSRV)
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

	// TXT query
	msg = GetDnsMsg(ptr.Ptr, dns.TypeTXT)
	in, _, err = c.Exchange(msg, dnsAddr)
	if err != nil {
		klog.Error(err, err.Error())
		return nil, err
	}
	txt, err := AnswerToTxt(in.Answer)
	if err != nil {
		klog.Error(err, err.Error())
		return nil, err
	}

	txtData := &discovery.TxtData{}
	if err = txtData.Decode(srv.Target, strconv.Itoa(int(srv.Port)), txt); err != nil {
		klog.Error(err, err.Error())
		return nil, err
	}
	txtData.Ttl = srv.Header().Ttl
	return txtData, nil
}

func GetDnsMsg(name string, qType uint16) *dns.Msg {
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

func AnswerToTxt(answers []dns.RR) ([]string, error) {
	res := []string{}
	for _, ans := range answers {
		txt, ok := ans.(*dns.TXT)
		if !ok {
			return nil, errors.New("Not TXT record: " + ans.String())
		}
		res = append(res, txt.Txt...)
	}
	return res, nil
}
