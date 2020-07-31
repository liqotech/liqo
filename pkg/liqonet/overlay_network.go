package liqonet

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/vishvananda/netlink"
	"io/ioutil"
	"k8s.io/client-go/kubernetes"
	"net"
	"os"
	"strconv"
	"strings"
)

const (
	vxlanOverhead int = 50
)

type VxlanNetConfig struct {
	Network    string `json:"Network"`
	DeviceName string `json:"DeviceName"`
	Port       string `json:"Port"`
	Vni        string `json:"Vni"`
}

func CreateVxLANInterface(clientset *kubernetes.Clientset, vxlanConfig VxlanNetConfig) error {
	podIPAddr, err := getPodIP()
	if err != nil {
		return err
	}
	token := strings.Split(vxlanConfig.Network, "/")
	vxlanNet := token[0]

	//get the mtu of the default interface
	mtu, err := getDefaultIfaceMTU()
	if err != nil {
		return err
	}

	//derive IP for the vxlan device
	//take the last octet of the podIP
	//TODO: use & and | operators with masks
	temp := strings.Split(podIPAddr.String(), ".")
	temp1 := strings.Split(vxlanNet, ".")
	vxlanIPString := temp1[0] + "." + temp1[1] + "." + temp1[2] + "." + temp[3]
	vxlanIP := net.ParseIP(vxlanIPString)

	vxlanMTU := mtu - vxlanOverhead
	vni, err := strconv.Atoi(vxlanConfig.Vni)
	if err != nil {
		return fmt.Errorf("unable to convert vxlan vni \"%s\" from string to int: %v", vxlanConfig.Vni, err)
	}
	port, err := strconv.Atoi(vxlanConfig.Port)
	if err != nil {
		return fmt.Errorf("unable to convert vxlan port \"%s\" from string to int: %v", vxlanConfig.Port, err)
	}
	attr := &VxlanDeviceAttrs{
		Vni:      uint32(vni),
		Name:     vxlanConfig.DeviceName,
		VtepPort: port,
		VtepAddr: podIPAddr,
		Mtu:      vxlanMTU,
	}
	vxlanDev, err := NewVXLANDevice(attr)
	if err != nil {
		return fmt.Errorf("failed to create vxlan interface on node with ip -> %s: %v", podIPAddr.String(), err)
	}
	err = vxlanDev.ConfigureIPAddress(vxlanIP, net.IPv4Mask(255, 255, 255, 0))
	if err != nil {
		return fmt.Errorf("failed to configure ip in vxlan interface on node with ip -> %s: %v", podIPAddr.String(), err)
	}

	remoteVETPs, err := getRemoteVTEPS(clientset)
	if err != nil {
		return err
	}

	for _, vtep := range remoteVETPs {
		macAddr, err := net.ParseMAC("00:00:00:00:00:00")
		if err != nil {
			return fmt.Errorf("unable to parse mac address. %v", err)
		}
		fdbEntry := Neighbor{
			MAC: macAddr,
			IP:  net.ParseIP(vtep),
		}
		err = vxlanDev.AddFDB(fdbEntry)
		if err != nil {
			return fmt.Errorf("an error occured while adding an fdb entry : %v", err)
		}
	}
	return nil
}

//this function enables the rp_filter on each vxlan interface on the node
func Enable_rp_filter() error {
	//list all the network interfaces on the host
	ifaces_list, err := netlink.LinkList()
	if err != nil {
		return fmt.Errorf("unable to retrieve the all the network interfaces: %v", err)
	}
	for index := range ifaces_list {
		if ifaces_list[index].Type() == "vxlan" {
			// Enable loose mode reverse path filtering on the vxlan interface.
			err = ioutil.WriteFile("/proc/sys/net/ipv4/conf/"+ifaces_list[index].Attrs().Name+"/rp_filter", []byte("2"), 0600)
			if err != nil {
				return fmt.Errorf("unable to update vxlan rp_filter proc entry for interface %s, err: %s", ifaces_list[index].Attrs().Name, err)
			}
		}
	}
	return nil
}

func getDefaultIfaceMTU() (int, error) {
	//search for the default route and return the link associated to the route
	//we consider only the ipv4 routes
	mtu := 0
	routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		return mtu, fmt.Errorf("unable to list routes while trying to identify default interface for the host: %v", err)
	}
	var route netlink.Route
	for _, route = range routes {
		if route.Dst == nil {
			break
		}
	}
	//get default link
	defualtIface, err := netlink.LinkByIndex(route.LinkIndex)
	if err != nil {
		return mtu, fmt.Errorf("unable to retrieve link with index %d :%v", route.LinkIndex, err)
	}
	return defualtIface.Attrs().MTU, nil
}

//the config file is expected to reside in /etc/kube-liqo/liqonet/vxlan-net-conf.json
func ReadVxlanNetConfig(defaultConfig VxlanNetConfig) (VxlanNetConfig, error) {
	pathToConfigFile := "/etc/kube-liqonet/liqonet/vxlan-net-conf.json" //path where we expect the configuration file

	var config VxlanNetConfig
	//check if the file exists
	if _, err := os.Stat(pathToConfigFile); err == nil {
		data, err := ioutil.ReadFile(pathToConfigFile)
		//TODO: add debugging info
		if err != nil {
			return config, fmt.Errorf("an erro occured while reading \"%s\" configuration file: %v", pathToConfigFile, err)
		}
		err = json.Unmarshal(data, &config)
		if err != nil {
			return config, fmt.Errorf("an error occured while unmarshalling \"%s\" configuration file: %v", pathToConfigFile, err)
		}
		if config.Network == "" || config.Port == "" || config.DeviceName == "" || config.Vni == "" {
			return config, errors.New("some configuration fields are missing in \"" + pathToConfigFile + "\", please check your configuration.")
		}
		return config, nil
	} else {
		return defaultConfig, nil
	}
}
