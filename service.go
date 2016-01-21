package main

import (
	"github.com/FlukeNetworks/apheleia/nerve"
	"github.com/FlukeNetworks/apheleia/synapse"
)

type ApheleiaNode struct {
	Services []Service `json:"services"`
}

type Service struct {
	Name string `json:"name"`
	Public bool `json:"public"`
	Patterns ServicePatterns `json:"patterns"`
	PortIndex int `json:"port_index" yaml:"port_index"`
	ServicePort int `json:"service_port" yaml:"service_port"`
	Nerve nerve.Service `json:"nerve"`
	Synapse synapse.Service `json:"synapse"`
}

func (s *Service) GetNodePath() string {
	return "/" + s.Name
}

type ServicePatterns struct {
	Executor string `json:"executor"`
	Task string `json:"task"`
}
