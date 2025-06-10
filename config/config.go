package config

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"github.com/mcuadros/go-defaults"
	"gopkg.in/yaml.v3"
	"os"
)

var (
	DefaultPath = "config.yml"
	_config     *Config
)

type Config struct {
	path   string
	Debug  bool   `yaml:"-" default:"false"`
	Token  string `yaml:"token"`
	Remote string `yaml:"remote"`

	Server ServerConfig `yaml:"server"`
	System SystemConfig `yaml:"system"`
	Docker DockerConfig `yaml:"docker"`
}

type ServerConfig struct {
	Bind string    `yaml:"bind" default:"127.0.0.1"`
	Port int       `yaml:"port" default:"8083"`
	TLS  TLSConfig `yaml:"tls"`
}

type TLSConfig struct {
	Enabled bool   `yaml:"enabled" default:"false"`
	Cert    string `yaml:"cert" default:""`
	Key     string `yaml:"key" default:""`
}

type SystemConfig struct {
	RootDirectory    string `yaml:"root_directory" default:"~/zephyr"`
	LogDirectory     string `yaml:"log_directory" default:"~/zephyr/logs"`
	VolumesDirectory string `yaml:"volumes_directory" default:"~/zephyr/volumes"`
	DataDirectory    string `yaml:"data_directory" default:"~/zephyr/data"`
	BackupDirectory  string `yaml:"backup_directory" default:"~/zephyr/backups"`
	TempDirectory    string `yaml:"temp_directory" default:"~/zephyr/tmp"`
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
	ccopy := *c
	if ccopy.path == "" {
		return errors.New("config path is not set")
	}

	b, err := yaml.Marshal(&ccopy)
	if err != nil {
		return err
	}

	return os.WriteFile(c.path, b, 0o600)
}

func DefaultConfig(path string) *Config {
	token := make([]byte, 32)
	_, _ = rand.Read(token)
	tokenStr := base64.StdEncoding.EncodeToString(token)

	// generate default config
	c := &Config{}
	defaults.SetDefaults(c)

	c.path = path
	c.Debug = false
	c.Token = tokenStr
	c.Remote = "http://127.0.0.1:8792"
	return c
}
