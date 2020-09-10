package search_domain_operator

import (
	"errors"
	"github.com/liqotech/liqo/internal/discovery"
	"github.com/miekg/dns"
	"k8s.io/klog"
	"strconv"
	"time"
)

func Wan(dnsAddr string, name string, test bool) ([]*discovery.TxtData, error) {
	txtData := []*discovery.TxtData{}

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
		txt, err := ResolveWan(c, dnsAddr, ptr, test)
		if err != nil {
			klog.Error(err, err.Error())
			return nil, err
		}
		txtData = append(txtData, txt)
	}
	return txtData, nil
}

func ResolveWan(c *dns.Client, dnsAddr string, ptr *dns.PTR, test bool) (*discovery.TxtData, error) {
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

	if test {
		return completeResolution(c, dnsAddr, srv, txt)
	} else {
		// let http client to do A/CNAME resolution
		return discovery.Decode(srv.Target, strconv.Itoa(int(srv.Port)), txt)
	}
}

func completeResolution(c *dns.Client, dnsAddr string, srv *dns.SRV, txt []string) (*discovery.TxtData, error) {
	// A query
	msg := GetDnsMsg(srv.Target, dns.TypeA)
	in, _, err := c.Exchange(msg, dnsAddr)
	if err != nil {
		klog.Error(err, err.Error())
		return nil, err
	}
	if len(in.Answer) == 0 {
		// no A record is set, let's try with CNAME
		msg = GetDnsMsg(srv.Target, dns.TypeCNAME)
		in, _, err = c.Exchange(msg, dnsAddr)
		if err != nil {
			klog.Error(err, err.Error())
			return nil, err
		}

		if len(in.Answer) == 0 {
			klog.Error("no A record or CNAME record is set for " + srv.Target)
			return nil, errors.New("no A record or CNAME record is set for " + srv.Target)
		}

		cname := in.Answer[0].(*dns.CNAME)
		return discovery.Decode(cname.Target, strconv.Itoa(int(srv.Port)), txt)
	} else {
		a := in.Answer[0].(*dns.A)
		return discovery.Decode(a.A.String(), strconv.Itoa(int(srv.Port)), txt)
	}
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
