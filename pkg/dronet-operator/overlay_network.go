package dronet_operator

import (
	"fmt"
	"k8s.io/client-go/kubernetes"
	"net"
	"strings"
)

var (
	mtu int = 1500
	VxLANOverhead int = 50
	vxlanDeviceName = "dronet"
	vni int = 200
	vxlanPort = 4789
)
func CreateVxLANInterface(clientset *kubernetes.Clientset) error {
	 mtu := 1500

	podIPAddr, err := getPodIP()
	if err != nil{
		return err
	}
	if err !=nil {
		return err
	}
/*	vxlanCIDR, err := getOverlayCIDR()
	if err != nil{
		return err
	}*/
	vxlanCIDR := "192.168.200.0"
	//derive IP for the vxlan device
	//take the last octet of the podIP
	//TODO: use & and | operators with masks
	temp := strings.Split(podIPAddr.String(), ".")
	temp1 := strings.Split(vxlanCIDR, ".")
	vxlanIPString := temp1[0] + "." + temp1[1] + "." + temp1[2] + "." + temp[3]
	vxlanIP := net.ParseIP(vxlanIPString)

	//TODO: Derive the MTU based on the default outgoing interface
	vxlanMTU := mtu -VxLANOverhead
	attr := &VxlanDeviceAttrs{
		Vni:       200,
		Name:      vxlanDeviceName,
		VtepPort:  vxlanPort,
		VtepAddr:  podIPAddr,
		Mtu: 	   vxlanMTU,
	}
	vxlanDev, err := NewVXLANDevice(attr)
	if err != nil {
		return fmt.Errorf("failed to create vxlan interface on node with ip -> %s: %v", podIPAddr.String(), err)
	}
	err = vxlanDev.ConfigureIPAddress(vxlanIP, net.IPv4Mask(255, 255, 255, 0))
	if err != nil {
		return fmt.Errorf("failed to configure ip in vxlan interface on node with ip -> %s: %v",podIPAddr.String(), err)
	}
	remoteVETPs, err := getRemoteVTEPS(clientset)
	if err != nil{
		return err
	}

		for _, vtep := range remoteVETPs{
			macAddr, err := net.ParseMAC("00:00:00:00:00:00")
			if err != nil {
				return fmt.Errorf("unable to parse mac address. %v", err)
			}
			fdbEntry := Neighbor{
				MAC: macAddr,
				IP:  net.ParseIP(vtep),
			}
			err = vxlanDev.AddFDB(fdbEntry)
			if err != nil{
				return fmt.Errorf("an error occured while adding an fdb entry : %v", err)
			}
		}

	return nil
}

