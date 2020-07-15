package client

import (
	"fmt"
	advtypes "github.com/liqoTech/liqo/api/advertisement-operator/v1"
	"strings"
)

//DescribeAdvertisement provides a textual representation of an Advertisement CR that can be displayed in a MenuNode.
func DescribeAdvertisement(adv *advtypes.Advertisement) string {
	str := strings.Builder{}
	prices := adv.Spec.Prices
	str.WriteString(fmt.Sprintf("• ClusterID: %v\n", adv.Spec.ClusterId))
	str.WriteString(fmt.Sprintf("\t• STATUS: %v\n", adv.Status.AdvertisementStatus))
	str.WriteString("\t• Available Resources:\n")
	str.WriteString(fmt.Sprintf("\t\t- shared cpu = %v ", adv.Spec.ResourceQuota.Hard.Cpu()))
	if CpuPrice, cFound := prices["cpu"]; cFound {
		str.WriteString(fmt.Sprintf("[price %v]", CpuPrice.String()))
	}
	str.WriteString("\n")
	str.WriteString(fmt.Sprintf("\t\t-shared memory = %v ", adv.Spec.ResourceQuota.Hard.Memory()))
	if MemPrice, mFound := prices["memory"]; mFound {
		str.WriteString(fmt.Sprintf("[price %v]", MemPrice.String()))
	}
	str.WriteString("\n")
	str.WriteString(fmt.Sprintf("\t\t-available pods = %v ", adv.Spec.ResourceQuota.Hard.Pods()))
	return str.String()
}
