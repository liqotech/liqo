package discovery

import (
	"context"
	"errors"
	"github.com/grandcat/zeroconf"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"os"
	"strings"
)

type TxtData struct {
	ID        string
	Name      string
	Namespace string
	ApiUrl    string
	Ttl       uint32
}

func (txtData TxtData) Encode() ([]string, error) {
	res := []string{
		"id=" + txtData.ID,
		"namespace=" + txtData.Namespace,
		"url=" + txtData.ApiUrl,
	}
	if txtData.Name != "" {
		res = append(res, "name="+txtData.Name)
	}
	return res, nil
}

func (txtData *TxtData) Decode(address string, port string, data []string) error {
	for _, d := range data {
		if strings.HasPrefix(d, "id=") {
			txtData.ID = d[len("id="):]
		} else if strings.HasPrefix(d, "namespace=") {
			txtData.Namespace = d[len("namespace="):]
		} else if strings.HasPrefix(d, "name=") {
			txtData.Name = d[len("name="):]
		} else if strings.HasPrefix(d, "url=") {
			// used in LAN discovery
			txtData.ApiUrl = d[len("url="):]
		}
	}

	// used in WAN discovery
	if address != "" && port != "" {
		txtData.ApiUrl = "https://" + address + ":" + port
	}
	if txtData.ID == "" || txtData.Namespace == "" || txtData.ApiUrl == "" {
		return errors.New("TxtData missing required field")
	}
	return nil
}

func (discovery *DiscoveryCtrl) GetTxtData() (*TxtData, error) {
	apiUrl, err := discovery.GetAPIUrl()
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	txtData := &TxtData{
		ID:        discovery.ClusterId.GetClusterID(),
		Namespace: discovery.Namespace,
		ApiUrl:    apiUrl,
	}
	if discovery.Config.ClusterName != "" {
		txtData.Name = discovery.Config.ClusterName
	}
	return txtData, nil
}

// get API Server Url for this cluster
// if APISERVER env variable is set we read it's ip form this variable
//     (this can be useful on managed k8s services where we have no master node)
// else get the IP of first master
// if APISERVER_PORT env variable is set we use it has port
// else we fallback to default port
func (discovery *DiscoveryCtrl) GetAPIUrl() (string, error) {
	address, ok := os.LookupEnv("APISERVER")
	if !ok || address == "" {
		nodes, err := discovery.crdClient.Client().CoreV1().Nodes().List(context.TODO(), v1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/master",
		})
		if err != nil {
			return "", err
		}
		if len(nodes.Items) == 0 || len(nodes.Items[0].Status.Addresses) == 0 {
			err = errors.New("no APISERVER env variable found and no master node found, one of the two values must be present")
			klog.Error(err)
			return "", err
		}
		address = nodes.Items[0].Status.Addresses[0].Address
	}

	port, ok := os.LookupEnv("APISERVER_PORT")
	if !ok {
		port = "6443"
	}

	return "https://" + address + ":" + port, nil
}

func (txtData *TxtData) Get(discovery *DiscoveryCtrl, entry *zeroconf.ServiceEntry) error {
	if discovery.isForeign(entry.AddrIPv4) {
		err := txtData.Decode("", "", entry.Text)
		if err != nil {
			klog.Error(err)
			return err
		}
		klog.V(4).Infof("Remote cluster found at %s", txtData.ApiUrl)
		txtData.Ttl = entry.TTL
		return nil
	}
	return nil
}

func (txtData *TxtData) IsComplete() bool {
	return txtData.ApiUrl != "" && txtData.Namespace != "" && txtData.Name != "" && txtData.ID != ""
}
