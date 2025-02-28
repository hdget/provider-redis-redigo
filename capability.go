package redis_redigo

import (
	"github.com/hdget/common/types"
	"github.com/hdget/provider-redis-redigo/pkg"
	"go.uber.org/fx"
)

const (
	providerName = "redis-redigo"
)

var Capability = &types.Capability{
	Category: types.ProviderCategoryRedis,
	Name:     providerName,
	Module: fx.Module(
		providerName,
		fx.Provide(pkg.New),
	),
}
