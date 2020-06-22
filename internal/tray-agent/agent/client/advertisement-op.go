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
	str.WriteString(fmt.Sprintf("\t• ClusterID: %v\n\t• Available Resources:\n", adv.Spec.ClusterId))
	str.WriteString(fmt.Sprintf("\t• STATUS: %v", adv.Status.AdvertisementStatus))
	str.WriteString(fmt.Sprintf("\t\t-shared cpu = %v ", adv.Spec.ResourceQuota.Hard.Cpu()))
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

// ListAdvertisements returns a slice containing the human-readable description
// of each Advertisement currently inside the cluster associated to the client
/*func ListAdvertisements(c *client.Client) (advDescriptionList []string, err error) {
	advClient := *c
	var advList v1.AdvertisementList
	err = advClient.List(context.Background(), &advList)
	if err != nil {
		return nil, err
	}
	//temporary workaround to show advertisements notifications
	if len(advList.Items) > 0 {
		app_indicator.GetIndicator().SetIcon(app_indicator.IconLiqoAdvNew)
		app_indicator.GetIndicator().SetLabel(string(len(advList.Items)))
	}
	for i, adv := range advList.Items {

		str := strings.Builder{}
		str.WriteString(fmt.Sprintf("❨%02d❩ ⟹\t%s", i+1, DescribeAdvertisement(&adv)))
		advDescriptionList = append(advDescriptionList, str.String())
	}
	return advDescriptionList, nil
}*/
