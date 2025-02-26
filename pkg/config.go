package pkg

import (
	"github.com/hdget/common/intf"
	"github.com/pkg/errors"
)

type redisProviderConfig struct {
	Default *redisClientConfig   `mapstructure:"default"`
	Items   []*redisClientConfig `mapstructure:"items"`
}

type redisClientConfig struct {
	Name     string `mapstructure:"name"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Db       int    `mapstructure:"db"`
	Password string `mapstructure:"password"`
}

const (
	configSection = "sdk.redis"
)

var (
	errInvalidConfig = errors.New("invalid redis config")
	errEmptyConfig   = errors.New("empty redis config")
)

func newConfig(configProvider intf.ConfigProvider) (*redisProviderConfig, error) {
	if configProvider == nil {
		return nil, errInvalidConfig
	}

	var c *redisProviderConfig
	err := configProvider.Unmarshal(&c, configSection)
	if err != nil {
		return nil, err
	}

	if c == nil {
		return nil, errEmptyConfig
	}

	err = c.validate()
	if err != nil {
		return nil, errors.Wrap(err, "validate redis provider config")
	}

	return c, nil
}

func (c *redisProviderConfig) validate() error {
	if c.Default != nil {
		err := c.validateInstanceConfig(c.Default)
		if err != nil {
			return err
		}
	}

	for _, item := range c.Items {
		err := c.validateExtraInstanceConfig(item)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *redisProviderConfig) validateInstanceConfig(conf *redisClientConfig) error {
	if conf.Host == "" {
		return errEmptyConfig
	}

	// setup default config value
	if conf.Port == 0 {
		conf.Port = 6379
	}

	return nil
}

func (c *redisProviderConfig) validateExtraInstanceConfig(conf *redisClientConfig) error {
	if conf.Name == "" || conf.Host == "" {
		return errInvalidConfig
	}

	// setup default config value
	if conf.Port == 0 {
		conf.Port = 6379
	}

	return nil
}
