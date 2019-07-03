package config

import (
	"io/ioutil"

	yaml "gopkg.in/yaml.v2"
)

var conf = &Config{}

// GetConfig returns the config instance of cassini
func GetConfig() *Config {
	return conf
}

// Config wraps all configure data of gaiabot
type Config struct {
	// ConfigFile is configure file path of cassini
	ConfigFile string

	// LogConfigFile is configure file path of log
	LogConfigFile string `yaml:"log,omitempty"`

	// Ticker for schedule job
	Ticker uint32 `yaml:"ticker,omitempty"`

	Node string `yaml:"node,omitempty"`

	ChainID string `yaml:"chain_id,omitempty"`

	ValidatorAddress string `yaml:"validator_address,omitempty"`

	DelegatorAddress string `yaml:"delegator_address,omitempty"`

	WalletAddress string `yaml:"secure_wallet_address,omitempty"`
}

// Load the configure file
func (c *Config) Load() error {
	bytes, err := ioutil.ReadFile(c.ConfigFile)
	if err != nil {
		return err
	}
	return c.Parse(bytes)
}

// Parse the configure file
func (c *Config) Parse(bytes []byte) error {
	return yaml.UnmarshalStrict(bytes, c)
}
