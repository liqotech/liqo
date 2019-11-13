package messaging

import (
	"time"
)

const (
	ADD      string = "ADD"
	DELETE   string = "DEL"
	MODIFIED string = "MOD"
)

type AdvertisementMessage struct {
	AppName    string    `json:"app_name"`
	BaseNode   string    `json:"base_node"`
	Type       string    `json:"type"`
	Components []Component `json:"components"`
	Timestamp  float64     `json:"timestamp"`
}

type Component struct {
	Name             string     `json:"name"`
	Function         Function   `json:"function"`
	Parameters       interface{} `json:"parameters"`
	BootDependencies []string   `json:"boot_dependencies"`
	NodesBlacklist   []string   `json:"nodes-blacklist"`
	NodesWhitelist   []string   `json:"nodes-whitelist"`
}

type Function struct {
	Image     string    `json:"image"`
	Resources Resources `json:"resources"`
}

type Resources struct {
	Memory float64 `json:"memory"`
	Cpu    float64 `json:"cpu"`
}

func NewAdvertisementMessage(appName string, baseNode string, typeMes string, components []Component) *AdvertisementMessage {
	message := AdvertisementMessage{appName, baseNode, typeMes,components,float64(time.Now().Unix())}
	return &message
}

func NewComponent(name string, function Function, parameters interface{}, bootDependencies []string, NodesBlacklist []string, NodesWhitelist []string) *Component {
	components:=Component{name,function,parameters,bootDependencies,NodesBlacklist,NodesWhitelist}
	return &components
}

func NewFunction(image string, resources Resources) *Function{
	function:= Function{image,resources}
	return &function
}

func NewResources(memory float64, cpu float64) *Resources{
	resources :=  Resources{memory, cpu}
	return &resources
}

// Check if two message are equals
func (m *AdvertisementMessage) Equal(message AdvertisementMessage) bool{
	if m.AppName==message.AppName && m.BaseNode==message.BaseNode {
		return true
	}
	return false
}