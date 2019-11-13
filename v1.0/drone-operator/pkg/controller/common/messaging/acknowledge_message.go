package messaging

import (
	"time"
)

const (
	ADD_ACK      string = "ADD_ACK"
	DELETE_ACK   string = "DEL_ACK"
	MODIFIED_ACK string = "MOD_ACK"
)

type AcknowledgeMessage struct {
	Sender    string       `json:"sender"`
	BaseNode  string       `json:"base_node"`
	TypeAck   string       `json:"type_ack"`
	Component ComponentAck `json:"component"`
	Timestamp int64        `json:"timestamp"`
}

type ComponentAck struct {
	Name     string      `json:"name"`
	AppName  string      `json:"app_name"`
	Function FunctionAck `json:"function"`
}

type FunctionAck struct {
	Image     string       `json:"image"`
	Resources ResourcesAck `json:"resources"`
}

type ResourcesAck struct {
	Memory float64 `json:"memory"`
	Cpu    float64 `json:"cpu"`
}

func NewAcknowledgeMessage(sender string, baseNode string, typeAck string, component ComponentAck) *AcknowledgeMessage {
	message := AcknowledgeMessage{Sender: sender, BaseNode: baseNode, TypeAck: typeAck, Component: component, Timestamp: time.Now().Unix()}

	return &message
}

func NewComponentAck(name string, appName string, image string, memory float64, cpu float64) *ComponentAck {
	resources := ResourcesAck{memory, cpu}
	functionAck := FunctionAck{Image: image, Resources: resources}
	component := ComponentAck{Name: name, AppName: appName, Function: functionAck}

	return &component
}
