package sinks

import (
	"fmt"

	"github.com/NorthlandPowerEurope/dashy/agent/sinks/dashboard"
	"github.com/NorthlandPowerEurope/dashy/agent/sinks/opcua"
	"github.com/NorthlandPowerEurope/dashy/agent/sinks/value"
)

type Sender interface {
	Send(value.Container) error
}

var sinkMap = map[string]func(params map[string]string, mapping map[string]string) (Sender, error){
	"dashboard": func(params map[string]string, mapping map[string]string) (Sender, error) {
		return dashboard.NewClient(params, mapping)
	},
	"opcua": func(params map[string]string, mapping map[string]string) (Sender, error) {
		return opcua.NewClient(params, mapping)
	},
}

func New(t string, params map[string]string, mapping map[string]string) (Sender, error) {
	if creator, exists := sinkMap[t]; exists {
		return creator(params, mapping)
	}
	return nil, fmt.Errorf("unknown sink type: %s", t)
}
