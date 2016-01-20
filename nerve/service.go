package nerve

type Config struct {
	InstanceId string `json:"instance_id" yaml"instance_id"`
	Services []Service `json:"services"`
}

type Service struct {
	Host string `json:"host"`
	Port int `json:"port"`
	ReporterType string `json:"reporter_type" yaml:"reporter_type"`
	ZkHosts []string `json:"zk_hosts" yaml:"zk_hosts"`
	ZkPath string `json:"zk_path" yaml:"zk_path"`
	CheckInterval int `json:"check_interval" yaml:"check_interval"`
	Checks []map[string]interface{} `json:"checks"`
}
