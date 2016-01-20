package main

type ApheleiaNode struct {
	Services []Service `json:"services"`
}

type Service struct {
	Name string `json:"name"`
	Patterns ServicePatterns `json:"patterns"`
	PortIndex int `json:"port_index"`
	ServicePort int `json:"service_port"`
	CheckInterval int `json:"check_interval"`
	Checks []map[string]interface{} `json:"checks"`
}

type ServicePatterns struct {
	Executor string `json:"executor"`
	Task string `json:"task"`
}
