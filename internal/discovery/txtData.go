package discovery

import (
	b64 "encoding/base64"
	"encoding/json"
	"errors"
	"github.com/liqoTech/liqo/internal/discovery/clients"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"path"
	"strconv"
	"strings"
)

type TxtData struct {
	Config config
	ID     string
}

type config struct {
	Url       string `json:"url"`
	Namespace string `json:"namespace"`
}

func (txtData TxtData) Encode() ([]string, error) {
	bytes, err := json.Marshal(txtData.Config)
	if err != nil {
		return nil, err
	}
	res := []string{
		"config=" + b64.StdEncoding.EncodeToString(bytes),
		"id=" + txtData.ID,
	}
	return res, nil
}

func Decode(data []string) (*TxtData, error) {
	var res = TxtData{Config: config{}}
	for _, d := range data {
		if strings.HasPrefix(d, "config=") {
			bytes, err := b64.StdEncoding.DecodeString(d[len("config="):])
			if err != nil {
				return nil, err
			}
			err = json.Unmarshal(bytes, &res.Config)
			if err != nil {
				return nil, err
			}
		} else if strings.HasPrefix(d, "id=") {
			res.ID = d[len("id="):]
		}
	}
	if res.Config.Url == "" || res.ID == "" {
		return nil, errors.New("TxtData missing required field")
	}
	return &res, nil
}

func (discovery *DiscoveryCtrl) GetTxtData() TxtData {
	localClient, _ := clients.NewK8sClient()

	service, err := localClient.CoreV1().Services(discovery.Namespace).Get("credentials-provider", v1.GetOptions{})
	if err != nil {
		discovery.Log.Error(err, err.Error())
		os.Exit(1)
	}

	nl, err := localClient.CoreV1().Nodes().List(v1.ListOptions{})
	if err != nil {
		discovery.Log.Error(err, err.Error())
		os.Exit(1)
	}
	node := nl.Items[0].Status.Addresses[0].Address

	port := service.Spec.Ports[0].NodePort
	// TODO: add support for https
	url := "http://" + path.Join(node+":"+strconv.Itoa(int(port)), "config.yaml")

	return TxtData{
		Config: config{
			Url:       url,
			Namespace: discovery.Namespace,
		},
		ID: discovery.ClusterId.GetClusterID(),
	}
}
