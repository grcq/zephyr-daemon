package config

import (
	"crypto/rand"
	"encoding/base64"
	"gopkg.in/yaml.v3"
	"os"
)

var (
	DefaultPath = "config.yml"
	_config     *Config
)

type Config struct {
	path        string
	Debug       bool   `json:"-" default:"false"`
	Token       string `json:"token"`
	VolumesPath string `json:"volume"`
	DataPath    string `json:"data"`
	Remote      string `json:"remote"`

	Server ServerConfig `json:"server"`
}

type ServerConfig struct {
	Bind string    `json:"bind" default:"127.0.0.1"`
	Port int       `json:"port" default:"8083"`
	TLS  TLSConfig `json:"tls"`
}

type TLSConfig struct {
	Enabled bool   `json:"enabled" default:"false"`
	Cert    string `json:"cert"`
	Key     string `json:"key"`
}

func Load(path string) (*Config, error) {
	if _config != nil && _config.path == path {
		return _config, nil
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	config.path = path
	if err := yaml.Unmarshal(b, &config); err != nil {
		return nil, err
	}

	return Set(&config)
}

func Set(config *Config) (*Config, error) {
	_config = config
	return _config, nil
}

func Get() *Config {
	return _config
}

func (c *Config) Save() error {
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(c.path, b, 0644)
}

func DefaultConfig(path string) *Config {
	token := make([]byte, 32)
	_, _ = rand.Read(token)
	tokenStr := base64.StdEncoding.EncodeToString(token)

	c := &Config{
		path:        path,
		Debug:       false,
		Token:       tokenStr,
		VolumesPath: "~/zephyr/volumes",
		DataPath:    "~/zephyr/data",
		Remote:      "http://127.0.0.1:8792",
		Server: ServerConfig{
			Bind: "127.0.0.1",
			Port: 8083,
			TLS: TLSConfig{
				Enabled: false,
				Cert:    "",
				Key:     "",
			},
		},
	}

	return c
}
