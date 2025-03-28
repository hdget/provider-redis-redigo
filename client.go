package redis_redigo

import (
	"fmt"
	"github.com/gomodule/redigo/redis"
	"github.com/hdget/common/intf"
	"github.com/hdget/common/protobuf"
	"github.com/hdget/utils/convert"
	"github.com/hdget/utils/paginator"
	"strconv"
	"time"
)

type redisClient struct {
	pool *redis.Pool
}

func newRedisClient(conf *redisClientConfig) (intf.RedisClient, error) {
	// 建立连接池
	p := &redis.Pool{
		// 最大空闲连接数，有这么多个连接提前等待着，但过了超时时间也会关闭
		MaxIdle: 256,
		// 最大连接数，即最多的tcp连接数
		MaxActive: 0,
		// 空闲连接超时时间，但应该设置比redis服务器超时时间短。否则服务端超时了，客户端保持着连接也没用
		IdleTimeout: time.Duration(120),
		// 超过最大连接，是报错，还是等待
		Wait: true,
		Dial: func() (redis.Conn, error) {
			conn, err := redis.Dial("tcp", fmt.Sprintf("%s:%d", conf.Host, conf.Port),
				redis.DialPassword(conf.Password),
				redis.DialDatabase(conf.Db),
				redis.DialConnectTimeout(1*time.Minute),
				redis.DialReadTimeout(3*time.Second),
				redis.DialWriteTimeout(30*time.Second),
			)
			if err != nil {
				return nil, err
			}
			return conn, nil
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			if time.Since(t) < time.Minute {
				return nil
			}
			_, err := c.Do("PING")
			return err
		},
	}

	// ping test
	client := &redisClient{pool: p}
	err := client.Ping()
	if err != nil {
		return nil, err
	}

	return client, nil
}

///////////////////////////////////////////////////////////////////////
// general purpose
///////////////////////////////////////////////////////////////////////

// Del 删除某个key
func (r *redisClient) Del(key string) error {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)

	_, err := conn.Do("DEL", key)
	if err != nil {
		return err
	}
	return nil
}

// Dels 删除多个key
func (r *redisClient) Dels(keys []string) error {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)

	for _, k := range keys {
		err := conn.Send("DEL", k)
		if err != nil {
			return err
		}
	}

	return conn.Flush()
}

// Exists 检查某个key是否存在
func (r *redisClient) Exists(key string) (bool, error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	return redis.Bool(conn.Do("EXISTS", key))
}

// Expire 使某个key过期
func (r *redisClient) Expire(key string, expire int) error {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)

	_, err := conn.Do("EXPIRE", key, expire)
	return err
}

// Ttl 获取某个key的过期时间
func (r *redisClient) Ttl(key string) (int64, error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)

	return redis.Int64(conn.Do("TTL", key))
}

// Incr 将某个key中的值加1
func (r *redisClient) Incr(key string) error {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)

	_, err := conn.Do("INCR", key)
	return err
}

func (r *redisClient) IncrBy(key string, number int) error {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)

	_, err := conn.Do("INCRBY", key, number)
	return err
}

func (r *redisClient) DecrBy(key string, number int) error {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)

	_, err := conn.Do("DECRBY", key, number)
	return err
}

// Ping 检查redis是否存活
func (r *redisClient) Ping() error {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	_, err := conn.Do("PING")
	return err
}

// Pipeline 批量提交命令
func (r *redisClient) Pipeline(commands []*intf.RedisCommand) (reply interface{}, err error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)

	// 批量往本地缓存发送命令
	for _, cmd := range commands {
		err := conn.Send(cmd.Name, cmd.Args...)
		if err != nil {
			return nil, err
		}
	}

	// 批量提交命令到redis
	err = conn.Flush()
	if err != nil {
		return nil, err
	}

	// 获取批量命令的执行结果, 注意这里只会获取到最后那条命令执行的结果
	reply, err = conn.Receive()
	if err != nil {
		return nil, err
	}

	return reply, nil
}

// Shutdown 关闭redis client
func (r *redisClient) Shutdown() {
	_ = r.pool.Close()
}

// ////////////////////////////////////////////////////////////////////
// hash map operations
// ////////////////////////////////////////////////////////////////////

// HDel 删除某个field
func (r *redisClient) HDel(key string, field interface{}) (int, error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)

	return redis.Int(conn.Do("HDEL", key, field))
}

// HDels 删除多个field
func (r *redisClient) HDels(key string, fields []interface{}) (int, error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)

	return redis.Int(conn.Do("HDEL", redis.Args{}.Add(key).AddFlat(fields)...))
}

// HMGet 一次获取多个field的值,返回为二维[]byte
func (r *redisClient) HMGet(key string, fields []string) ([][]byte, error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	return redis.ByteSlices(conn.Do("HMGET", redis.Args{}.Add(key).AddFlat(fields)...))
}

// HMSet 一次设置多个field的值
func (r *redisClient) HMSet(key string, fieldvalues map[string]interface{}) error {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	_, err := conn.Do("HMSET", redis.Args{}.Add(key).AddFlat(fieldvalues)...)
	return err
}

// HGet 获取某个field的值
func (r *redisClient) HGet(key string, field any) ([]byte, error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	return redis.Bytes(conn.Do("HGET", key, field))
}

// HGetInt 获取某个field的int值
func (r *redisClient) HGetInt(key string, field string) (int, error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	return redis.Int(conn.Do("HGET", key, field))
}

// HGetInt64 获取某个field的int64值
func (r *redisClient) HGetInt64(key string, field string) (int64, error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	return redis.Int64(conn.Do("HGET", key, field))
}

// HGetFloat64 获取某个field的float64值
func (r *redisClient) HGetFloat64(key string, field string) (float64, error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	return redis.Float64(conn.Do("HGET", key, field))
}

// HGetString 获取某个field的float64值
func (r *redisClient) HGetString(key string, field string) (string, error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	return redis.String(conn.Do("HGET", key, field))
}

// HGetAll 获取所有fields的值
func (r *redisClient) HGetAll(key string) (map[string]string, error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	return redis.StringMap(conn.Do("HGETALL", key))
}

// HSet 设置某个field的值
func (r *redisClient) HSet(key string, field interface{}, value interface{}) (int, error) {
	s, err := convert.ToString(value)
	if err != nil {
		return 0, err
	}

	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)

	return redis.Int(conn.Do("HSET", key, field, s))
}

// HLen 设置某个field的值
func (r *redisClient) HLen(key string) (int, error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)

	return redis.Int(conn.Do("HLEN", key))
}

// /////////////////////////////////////////////////////////////////////////
// set
// /////////////////////////////////////////////////////////////////////////

// Get 获取某个key的值，返回为[]byte
func (r *redisClient) Get(key string) ([]byte, error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	return redis.Bytes(conn.Do("GET", key))
}

func (r *redisClient) GetInt(key string) (int, error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	return redis.Int(conn.Do("GET", key))
}

func (r *redisClient) GetInt64(key string) (int64, error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	return redis.Int64(conn.Do("GET", key))
}

func (r *redisClient) GetFloat64(key string) (float64, error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	return redis.Float64(conn.Do("GET", key))
}

func (r *redisClient) GetString(key string) (string, error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	return redis.String(conn.Do("GET", key))
}

// Set 设置某个key为value
func (r *redisClient) Set(key string, value interface{}) error {
	strValue, err := convert.ToString(value)
	if err != nil {
		return err
	}

	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)

	_, err = conn.Do("SET", key, strValue)
	return err
}

// SetEx 设置某个key为value,并设置过期时间(单位为秒)
func (r *redisClient) SetEx(key string, value interface{}, expire int) error {
	strValue, err := convert.ToString(value)
	if err != nil {
		return err
	}
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	_, err = conn.Do("SET", key, strValue, "EX", expire)
	return err
}

// /////////////////////////////////////////////////////////////////////////////
// set
// /////////////////////////////////////////////////////////////////////////////

// SIsMember 检查中成员是否出现在key中
func (r *redisClient) SIsMember(key string, member interface{}) (bool, error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	return redis.Bool(conn.Do("SISMEMBER", key, member))
}

// SAdd 集合中添加一个成员
func (r *redisClient) SAdd(key string, members interface{}) error {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)

	_, err := conn.Do("SADD", redis.Args{}.Add(key).AddFlat(members)...)
	return err
}

// SRem 集合中删除一个成员
func (r *redisClient) SRem(key string, members interface{}) error {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)

	_, err := conn.Do("SREM", redis.Args{}.Add(key).AddFlat(members)...)
	return err
}

// SInter 取不同keys中集合的交集
func (r *redisClient) SInter(keys []string) ([]string, error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	return redis.Strings(conn.Do("SINTER", redis.Args{}.AddFlat(keys)...))
}

// SUnion 取不同keys中集合的并集
func (r *redisClient) SUnion(keys []string) ([]string, error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	return redis.Strings(conn.Do("SUNION", redis.Args{}.AddFlat(keys)...))
}

// SDiff 比较不同集合中的不同元素
func (r *redisClient) SDiff(keys []string) ([]string, error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	return redis.Strings(conn.Do("SDIFF", redis.Args{}.AddFlat(keys)...))
}

// SMembers 取集合中的成员
func (r *redisClient) SMembers(key string) ([]string, error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	return redis.Strings(conn.Do("SMEMBERS", key))
}

// /////////////////////////////////////////////////////////////////////////////////////
// sorted set
// /////////////////////////////////////////////////////////////////////////////////////

// ZRemRangeByScore delete members by score
func (r *redisClient) ZRemRangeByScore(key string, min, max interface{}) error {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	_, err := conn.Do("ZREMRANGEBYSCORE", key, min, max)
	return err
}

// ZRangeByScore get members by score
func (r *redisClient) ZRangeByScore(key string, min, max interface{}, withScores bool, list *protobuf.ListParam) ([]string, error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)

	args := []interface{}{key, min, max}
	if withScores {
		args = append(args, "WITHSCORES")
	}

	if list != nil {
		p := paginator.New(list.Page, list.PageSize)
		args = append(args, "LIMIT", p.Offset, p.PageSize)
	}

	return redis.Strings(conn.Do("ZRANGEBYSCORE", args...))
}

// ZRange get members
func (r *redisClient) ZRange(key string, min, max int64) ([]string, error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	return redis.Strings(conn.Do("ZRANGE", key, min, max))
}

// ZAdd add a member
func (r *redisClient) ZAdd(key string, score int64, member interface{}) error {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	_, err := conn.Do("ZADD", key, score, member)
	return err
}

// ZIncrBy add increment to member's score
func (r *redisClient) ZIncrBy(key string, increment int64, member interface{}) error {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	_, err := conn.Do("ZINCRBY", key, increment, member)
	return err
}

// ZCard get members total
func (r *redisClient) ZCard(key string) (int, error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	return redis.Int(conn.Do("ZCARD", key))
}

// ZScore get score of member
func (r *redisClient) ZScore(key string, member interface{}) (int64, error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	return redis.Int64(conn.Do("ZSCORE", key, member))
}

// ZInterstore get intersect of set
func (r *redisClient) ZInterstore(destKey string, keys ...interface{}) (int64, error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	return redis.Int64(conn.Do("ZINTERSTORE", redis.Args{}.Add(destKey).AddFlat(keys)...))
}

// ZRem delete members
func (r *redisClient) ZRem(destKey string, members ...interface{}) (int64, error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	return redis.Int64(conn.Do("ZREM", redis.Args{}.Add(destKey).AddFlat(members)...))
}

// ///////////////////////////////////////////////////////////
// list
// ///////////////////////////////////////////////////////////

func (r *redisClient) LPush(key string, values ...any) error {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	_, err := conn.Do("LPUSH", redis.Args{}.Add(key).AddFlat(values)...)
	return err
}

func (r *redisClient) RPush(key string, values ...any) error {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	_, err := conn.Do("RPUSH", redis.Args{}.Add(key).AddFlat(values)...)
	return err
}

// RPop 移除列表的最后一个元素，返回值为移除的元素
func (r *redisClient) RPop(key string) ([]byte, error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	return redis.Bytes(conn.Do("RPOP", key))
}

func (r *redisClient) LRangeInt64(key string, start, end int64) ([]int64, error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	return redis.Int64s(conn.Do("LRANGE", key, start, end))
}

func (r *redisClient) LRangeString(key string, start, end int64) ([]string, error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	return redis.Strings(conn.Do("LRANGE", key, start, end))
}

func (r *redisClient) LLen(key string) (int64, error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	return redis.Int64(conn.Do("LLEN", key))
}

func (r *redisClient) Eval(scriptContent string, keys []interface{}, args []interface{}) (interface{}, error) {
	script := redis.NewScript(len(keys), scriptContent)

	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)

	keyArgs := make([]interface{}, 0)
	keyArgs = append(keyArgs, keys...)
	keyArgs = append(keyArgs, args...)

	reply, err := script.Do(conn, keyArgs...)
	if err != nil {
		return nil, err
	}

	return reply, nil
}

/////////////////////////////////////////////////////////////
// Redis Bloom
/////////////////////////////////////////////////////////////

// BfExists - Determines whether an item may exist in the Bloom Filter or not.
// args:
// key - the name of the filter
// item - the item to check for
func (r *redisClient) BfExists(key string, item string) (exists bool, err error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	return redis.Bool(conn.Do("BF.EXISTS", key, item))
}

// BfAdd - Add (or create and add) a new value to the filter
// args:
// key - the name of the filter
// item - the item to add
func (r *redisClient) BfAdd(key string, item string) (exists bool, err error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	return redis.Bool(conn.Do("BF.ADD", key, item))
}

// BfReserve - Creates an empty Bloom Filter with a given desired error ratio and initial capacity.
// args:
// key - the name of the filter
// error_rate - the desired probability for false positives
// capacity - the number of entries you intend to add to the filter
func (r *redisClient) BfReserve(key string, errorRate float64, capacity uint64) (err error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	_, err = conn.Do("BF.RESERVE", key, strconv.FormatFloat(errorRate, 'g', 16, 64), capacity)
	return err
}

// BfAddMulti - Adds one or more items to the Bloom Filter, creating the filter if it does not yet exist.
// args:
// key - the name of the filter
// item - One or more items to add
func (r *redisClient) BfAddMulti(key string, items []interface{}) ([]int64, error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	args := redis.Args{key}.AddFlat(items)
	result, err := conn.Do("BF.MADD", args...)
	return redis.Int64s(result, err)
}

// BfExistsMulti - Determines if one or more items may exist in the filter or not.
// args:
// key - the name of the filter
// item - one or more items to check
func (r *redisClient) BfExistsMulti(key string, items []interface{}) ([]int64, error) {
	conn := r.pool.Get()
	defer func(conn redis.Conn) {
		_ = conn.Close()
	}(conn)
	args := redis.Args{key}.AddFlat(items)
	result, err := conn.Do("BF.MEXISTS", args...)
	return redis.Int64s(result, err)
}
