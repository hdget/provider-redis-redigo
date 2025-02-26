package redigo

import (
	"github.com/hdget/common/types"
	"github.com/hdget/provider-redis-redigo/pkg"
	"go.uber.org/fx"
)

var Capability = &types.Capability{
	Category: types.ProviderCategoryRedis,
	Name:     types.ProviderNameRedisRedigo,
	Module: fx.Module(
		string(types.ProviderNameRedisRedigo),
		fx.Provide(pkg.New),
	),
}
