package discovery

import (
	"errors"
	"strings"
)

type TxtData struct {
	ID        string
	Namespace string
	ApiUrl    string
}

func (txtData TxtData) Encode() ([]string, error) {
	res := []string{
		"id=" + txtData.ID,
		"namespace=" + txtData.Namespace,
	}
	return res, nil
}

func Decode(ip string, port string, data []string) (*TxtData, error) {
	var res = TxtData{}
	for _, d := range data {
		if strings.HasPrefix(d, "id=") {
			res.ID = d[len("id="):]
		} else if strings.HasPrefix(d, "namespace=") {
			res.Namespace = d[len("namespace="):]
		}
	}
	res.ApiUrl = "https://" + ip + ":" + port
	if res.ID == "" || res.Namespace == "" || res.ApiUrl == "" {
		return nil, errors.New("TxtData missing required field")
	}
	return &res, nil
}

func (discovery *DiscoveryCtrl) GetTxtData() TxtData {
	return TxtData{
		ID:        discovery.ClusterId.GetClusterID(),
		Namespace: discovery.Namespace,
	}
}
