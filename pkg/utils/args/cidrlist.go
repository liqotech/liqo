package args

import (
	"net"
)

// CIDRList implements the flag.Value interface and allows to parse stringified lists
// in the form: "val1,val2".
type CIDRList struct {
	StringList StringList
	CIDRList   []net.IPNet
}

// String returns the stringified list.
func (cl *CIDRList) String() string {
	return cl.StringList.String()
}

// Set parses the provided string into the []string list.
func (cl *CIDRList) Set(str string) error {
	if cl.CIDRList == nil {
		cl.CIDRList = []net.IPNet{}
	}
	if err := cl.StringList.Set(str); err != nil {
		return err
	}

	for _, cidr := range cl.StringList.StringList {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return err
		}
		cl.CIDRList = append(cl.CIDRList, *ipNet)
	}
	return nil
}

// Type returns the cidrList type.
func (cl CIDRList) Type() string {
	return "cidrList"
}
