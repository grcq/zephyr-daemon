package config

type networkInterfaces struct {
	V4 struct {
		Subnet  string `default:"172.17.0.0/16"`
		Gateway string `default:"172.17.0.1"`
	}
	V6 struct {
		Subnet  string `default:"fd00:17f2:8ca3::/64"`
		Gateway string `default:"fd00:17f2:8ca3::1"`
	}
}

type DockerNetworkConfig struct {
	Interface string   `default:"0.0.0.0" yml:"interface"`
	Dns       []string `default:"[1.1.1.1,1.0.0.1]"`

	Name       string            `default:"zephyr"`
	ISPN       bool              `default:"false" yaml:"ispn"`
	IPv6       bool              `default:"true" yaml:"ipv6"`
	Driver     string            `default:"bridge" yaml:"driver"`
	Mode       string            `default:"zephyr" yaml:"network_mode"`
	IsInternal bool              `default:"false" yaml:"internal"`
	EnableICC  bool              `default:"true" yaml:"icc"`
	NetworkMTU int64             `default:"1500" yaml:"network_mtu"`
	Interfaces networkInterfaces `yaml:"interfaces"`
}

type DockerConfig struct {
	Network DockerNetworkConfig `yaml:"network"`

	DomainName string `default:"" yaml:"domain_name"`

	Registries map[string]RegistryConfig `yaml:"registries"`

	TmpfsSize  uint   `default:"100" yaml:"tmpfs_size"` // 100MB
	UsernsMode string `default:"" yaml:"userns_mode"`
}

type RegistryConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}
