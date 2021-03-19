package liqonet

import (
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
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

func InsertPolicyRoutingRule(tableID int, fromSubnet string, toSubnet string) error {
	var src, dst *net.IPNet
	var err error
	if fromSubnet != "" {
		_, src, err = net.ParseCIDR(fromSubnet)
		if err != nil {
			klog.Errorf("an error occurred parsing CIDR %s while inserting policy routing rule: %v", fromSubnet, err)
			return err
		}
	}
	if toSubnet != "" {
		_, dst, err = net.ParseCIDR(toSubnet)
		if err != nil {
			klog.Errorf("an error occurred parsing CIDR %s while inserting policy routing rule: %v", fromSubnet, err)
			return err
		}
	}
	//get existing rules
	rules, err := netlink.RuleList(netlink.FAMILY_V4)
	if err != nil {
		klog.Errorf("an error occurred while listing the policy routing rules: %v", err)
		return err
	}
	//check if the rule already exists
	for _, r := range rules {
		if reflect.DeepEqual(dst, r.Dst) && reflect.DeepEqual(src, r.Src) && r.Table == tableID {
			return nil
		}
	}
	rule := netlink.NewRule()
	rule.Table = tableID
	rule.Src = src
	rule.Dst = dst
	klog.Infof("inserting policy routing rule {%s}", rule.String())
	if err := netlink.RuleAdd(rule); err != nil && err != unix.EEXIST {
		klog.Errorf("an error occurred while inserting policy routing rule {%s}: %v", rule.String(), err)
		return err
	}
	return nil
}

func RemovePolicyRoutingRule(tableID int, fromSubnet, toSubnet string) error {
	var src, dst *net.IPNet
	var err error
	if fromSubnet != "" {
		_, src, err = net.ParseCIDR(fromSubnet)
		if err != nil {
			klog.Errorf("an error occurred parsing CIDR %s while inserting policy routing rule: %v", fromSubnet, err)
			return err
		}
	}
	if toSubnet != "" {
		_, dst, err = net.ParseCIDR(toSubnet)
		if err != nil {
			klog.Errorf("an error occurred parsing CIDR %s while inserting policy routing rule: %v", fromSubnet, err)
			return err
		}
	}
	//get existing rules
	rules, err := netlink.RuleList(netlink.FAMILY_V4)
	if err != nil {
		klog.Errorf("an error occurred while listing the policy routing rules: %v", err)
		return err
	}
	//check if the rule already exists
	for _, r := range rules {
		if reflect.DeepEqual(dst, r.Dst) && reflect.DeepEqual(src, r.Src) && r.Table == tableID {
			klog.Infof("removing policy routing rule {%s}", r.String())
			if err := netlink.RuleDel(&r); err != nil && err != unix.ESRCH {
				klog.Errorf("an error occurred while removing policy routing rule {%s}: %v", r.String(), err)
				return err
			}
		}
	}
	return nil
}
