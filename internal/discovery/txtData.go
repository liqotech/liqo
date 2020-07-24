package discovery

import (
	"errors"
	"strings"
)

type TxtData struct {
	ID               string
	Namespace        string
	AllowUntrustedCA bool
	ApiUrl           string
}

func (txtData TxtData) Encode() ([]string, error) {
	res := []string{
		"id=" + txtData.ID,
		"namespace=" + txtData.Namespace,
		"untrusted-ca=" + txtData.GetAllowUntrustedCA(),
	}
	return res, nil
}

func (txtData *TxtData) GetAllowUntrustedCA() string {
	if txtData.AllowUntrustedCA {
		return "true"
	} else {
		return "false"
	}
}

func Decode(ip string, port string, data []string) (*TxtData, error) {
	var res = TxtData{}
	for _, d := range data {
		if strings.HasPrefix(d, "id=") {
			res.ID = d[len("id="):]
		} else if strings.HasPrefix(d, "namespace=") {
			res.Namespace = d[len("namespace="):]
		} else if strings.HasPrefix(d, "untrusted-ca=") {
			res.AllowUntrustedCA = d[len("untrusted-ca="):] == "true"
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
		ID:               discovery.ClusterId.GetClusterID(),
		Namespace:        discovery.Namespace,
		AllowUntrustedCA: discovery.Config.AllowUntrustedCA,
	}
}
