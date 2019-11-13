package messaging

import (
	"time"
)

type ResourceMessage struct {
	Sender        string        `json:"sender"`
	NodeResources NodeResources `json:"node_resources"`
	Timestamp     int64         `json:"timestamp"`
}

type NodeResources struct {
	Memory float64 `json:"memory"`
	Cpu    float64 `json:"cpu"`
}

func NewResourceMessage(sender string, memory float64, cpu float64) *ResourceMessage {
	resources := NodeResources{memory, cpu}
	message := ResourceMessage{sender, resources, time.Now().Unix()}

	return &message
}
