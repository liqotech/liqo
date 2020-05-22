package discovery

import (
	b64 "encoding/base64"
	"encoding/json"
	"github.com/netgroup-polito/dronev2/internal/discovery/clients"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"path"
	"strconv"
)

type TxtData struct {
	Url string `json:"url"`
}

func (txtData TxtData) Encode() (string, error) {
	bytes, err := json.Marshal(txtData)
	if err != nil {
		return "", err
	}
	return b64.StdEncoding.EncodeToString(bytes), nil
}

func Decode(data string) (map[string]interface{}, error) {
	bytes, err := b64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, err
	}
	var res = map[string]interface{}{}
	err = json.Unmarshal(bytes, &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func GetTxtData() TxtData {
	localClient, _ := clients.NewK8sClient()

	service, err := localClient.CoreV1().Services(Namespace).Get("credentials-provider", v1.GetOptions{})
	if err != nil {
		Log.Error(err, err.Error())
		os.Exit(1)
	}

	nl, err := localClient.CoreV1().Nodes().List(v1.ListOptions{})
	if err != nil {
		Log.Error(err, err.Error())
		os.Exit(1)
	}
	node := nl.Items[0].Status.Addresses[0].Address

	port := service.Spec.Ports[0].NodePort
	// TODO: add support for https
	url := "http://" + path.Join(node+":"+strconv.Itoa(int(port)), "config.yaml")

	return TxtData{
		Url: url,
	}
}
