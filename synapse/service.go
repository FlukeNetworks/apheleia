package synapse

type Config map[string]interface{}

type Service struct {
	DefaultServers []Server `json:"default_servers" yaml:"default_servers"`
	Discovery Discovery `json:"discovery"`
	HAProxy HAProxyInfo `json:"haproxy" yaml:"haproxy"`
}

type Server struct {
	Name string `json:"name"`
	Host string `json:"host"`
	Port int `json:"port"`
}

type Discovery struct {
	Method string `json:"method"`
	Path string `json:"path"`
	Hosts []string `json:"hosts"`
}

type HAProxyInfo struct {
	Port int `json:"port"`
	ServerOptions string `json:"server_options" yaml:"server_options"`
	Listen []string `json:"listen"`
}
