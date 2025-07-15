package webconfig

import (
	"fmt"
	"io/ioutil"
	"log"

	"gopkg.in/yaml.v3"
)

// TLSConfig holds the TLS configuration parameters.
type TLSConfig struct {
	CertFile     string `yaml:"cert_file"`
	KeyFile      string `yaml:"key_file"`
	ClientCAFile string `yaml:"client_ca_file"`
}

// BasicAuthConfig holds the Basic Authentication configuration parameters.
type BasicAuthConfig struct {
	Username     string `yaml:"username"`
	PasswordFile string `yaml:"password_file"`
}

// WebConfig is the top-level configuration structure.
type WebConfig struct {
	TLS      *TLSConfig       `yaml:"tls_server_config,omitempty"`
	BasicAuth *BasicAuthConfig `yaml:"basic_auth,omitempty"`
}

// LoadConfig reads and parses the web configuration file from the given path.
func LoadConfig(path string) (*WebConfig, error) {
	log.Printf("Loading web configuration from: %s", path)
	yamlFile, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read web config file: %w", err)
	}

	var config WebConfig
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse web config YAML: %w", err)
	}

	return &config, nil
}
