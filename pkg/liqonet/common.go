package liqonet

import (
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"io/ioutil"
	"k8s.io/klog"
	"net"
	"os"
	"reflect"
	"strconv"
	"strings"
)

const (
	RoutingTableID   = 18952
	RoutingTableName = "liqo"
)

//create a new routing table with the given ID and name
func CreateRoutingTable(tableID int, tableName string) error {
	klog.Infof("creating routing table with ID %d and name %s", tableID, tableName)
	//file path
	rtTableFilePath := "/etc/iproute2/rt_tables"
	data := strings.Join([]string{strconv.Itoa(tableID), tableName}, "\t")
	//open the file
	file, err := os.OpenFile(rtTableFilePath, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		klog.Errorf("an error occurred while opening file %s: %v", rtTableFilePath, err)
		return err
	}
	defer file.Close()
	//write data to file
	if _, err := file.WriteString(data); err != nil {
		klog.Errorf("an error occurred while writing to file %s: %v", rtTableFilePath, err)
		return err
	}
	return nil
}

func InsertPolicyRoutingRule(tableID int, fromSubnet, toSubnet *net.IPNet) (*netlink.Rule, error) {
	//get existing rules
	rules, err := netlink.RuleList(netlink.FAMILY_V4)
	if err != nil {
		klog.Errorf("an error occurred while listing the policy routing rules: %v", err)
		return nil, err
	}
	//check if the rule already exists
	for _, r := range rules {
		if reflect.DeepEqual(toSubnet, r.Dst) && reflect.DeepEqual(fromSubnet, r.Src) && r.Table == tableID {
			return nil, unix.EEXIST
		}
	}
	rule := netlink.NewRule()
	rule.Table = tableID
	rule.Src = fromSubnet
	rule.Dst = toSubnet
	klog.Infof("inserting policy routing rule {%s}", rule.String())
	if err := netlink.RuleAdd(rule); err != nil {
		return nil, err
	}
	return rule, nil
}

func RemovePolicyRoutingRule(tableID int, fromSubnet, toSubnet *net.IPNet) error {
	//get existing rules
	rules, err := netlink.RuleList(netlink.FAMILY_V4)
	if err != nil {
		klog.Errorf("an error occurred while listing the policy routing rules: %v", err)
		return err
	}
	//check if the rule already exists
	for _, r := range rules {
		if reflect.DeepEqual(toSubnet, r.Dst) && reflect.DeepEqual(fromSubnet, r.Src) && r.Table == tableID {
			klog.Infof("removing policy routing rule {%s}", r.String())
			if err := netlink.RuleDel(&r); err != nil && err != unix.ESRCH {
				klog.Errorf("an error occurred while removing policy routing rule {%s}: %v", r.String(), err)
				return err
			}
		}
	}
	return nil
}

//this function enables the proxy arp for a given network interface
func EnableProxyArp(ifaceName string) error {
	klog.Infof("enabling proxy arp for interface %s", ifaceName)
	proxyArpFilePath := strings.Join([]string{"/proc/sys/net/ipv4/conf/", ifaceName, "/proxy_arp"}, "")
	err := ioutil.WriteFile(proxyArpFilePath, []byte("1"), 0600)
	if err != nil {
		klog.Errorf("an error occurred while writing to file %s: %v", proxyArpFilePath, err)
		return err
	}
	return nil
}
