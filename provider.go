package redis_redigo

import (
	"github.com/hdget/common/types"
)

type redigoProvider struct {
	defaultClient types.RedisClient            // 缺省redis
	extraClients  map[string]types.RedisClient // 额外的redis
}

func New(configProvider types.ConfigProvider, logger types.LoggerProvider) (types.RedisProvider, error) {
	config, err := newConfig(configProvider)
	if err != nil {
		return nil, err
	}

	p := &redigoProvider{
		extraClients: make(map[string]types.RedisClient),
	}

	if config.Default != nil {
		p.defaultClient, err = newRedisClient(config.Default)
		if err != nil {
			logger.Fatal("init redis default client", "err", err)
		}
		logger.Debug("init redis default client", "host", config.Default.Host)
	}

	for _, itemConf := range config.Items {
		p.extraClients[itemConf.Name], err = newRedisClient(itemConf)
		if err != nil {
			logger.Fatal("new redis extra client", "name", itemConf.Name, "err", err)
		}
		logger.Debug("init redis extra client", "name", itemConf.Name, "host", itemConf.Host)
	}

	return p, nil
}

func (p *redigoProvider) GetCapability() types.Capability {
	return Capability
}

func (p *redigoProvider) My() types.RedisClient {
	return p.defaultClient
}

func (p *redigoProvider) By(name string) types.RedisClient {
	return p.extraClients[name]
}
