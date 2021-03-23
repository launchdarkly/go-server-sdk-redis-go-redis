package ldredis

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-redis/redis/v8"

	"gopkg.in/launchdarkly/go-sdk-common.v2/ldvalue"
	"gopkg.in/launchdarkly/go-server-sdk.v5/interfaces"
)

const (
	// DefaultPrefix is a string that is prepended (along with a colon) to all Redis keys used
	// by the data store. You can change this value with the Prefix() option for
	// NewRedisDataStoreWithDefaults, or with the "prefix" parameter to the other constructors.
	//
	// See also DefaultClusterPrefix.
	DefaultPrefix = "launchdarkly"

	// DefaultClusterPrefix is an additional string of "{ld}." that is added before the
	// configured prefix (or DefaultPrefix) if you are connecting to a Redis cluster, if the
	// prefix does not already include curly braces.
	//
	// For instance, if you set the prefix to "app1", and you are using a single Redis node,
	// all keys will start with "app1:", but if you are using a cluster, they will start with
	// "{ld}.app1:". But if you set the prefix to "{xyz}app1", then the keys will start with
	// "{xyz}app1:" regardless of whether you are using a cluster or not.
	//
	// The reason for this is that in a Redis cluster, keys that begin with the same string in
	// curly braces are grouped together into one hash slot in the cluster. That allows
	// operations on those keys to be atomic, which the LaunchDarkly SDK Redis integration
	// relies on.
	//
	// When using a single Redis node rather than a cluster, there is no special meaning for
	// braces-- they are treated like any other characters in keys.
	DefaultClusterPrefix = "{ld}."
)

// DataStore returns a configurable builder for a Redis-backed data store.
func DataStore() *DataStoreBuilder {
	return &DataStoreBuilder{
		prefix:         DefaultPrefix,
		checkOnStartup: true,
	}
}

// DataStoreBuilder is a builder for configuring the Redis-based persistent data store.
//
// Obtain an instance of this type by calling DataStore(). After calling its methods to specify any
// desired custom settings, wrap it in a PersistentDataStoreBuilder by calling
// ldcomponents.PersistentDataStore(), and then store this in the SDK configuration's DataStore field.
//
// Builder calls can be chained, for example:
//
//     config.DataStore = ldredis.DataStore().URL("redis://hostname").Prefix("prefix")
//
// You do not need to call the builder's CreatePersistentDataStore() method yourself to build the
// actual data store; that will be done by the SDK.
type DataStoreBuilder struct {
	prefix         string
	redisOpts      redis.UniversalOptions
	url            string
	checkOnStartup bool
}

// CheckOnStartup sets whether the data store should check the availability of the Redis server when the
// SDK is initialized. If so, the SDK will refuse to start unless the server is available. This is true
// by default.
func (b *DataStoreBuilder) CheckOnStartup(value bool) *DataStoreBuilder {
	b.checkOnStartup = value
	return b
}

// HostAndPort is a shortcut for specifying the Redis host address as a hostname and port.
//
// To use multiple Redis hosts in cluster mode, use Addresses; or, use Options and set the Addrs field.
//
// Calling HostAndPort overwrites any addresses previously set with Addresses or Options.
func (b *DataStoreBuilder) HostAndPort(host string, port int) *DataStoreBuilder {
	return b.Addresses(fmt.Sprintf("%s:%d", host, port))
}

// Addresses specifies Redis host addresses. This is a shortcut for setting the Addrs field
// with Options.
//
// If multiple addresses are given, and a Master has been set, this is treated as a list of
// Redis Sentinel nodes.
//
// If multiple addresses are given, and no Master has been set, it is treated as a list of
// cluster nodes.
//
// If no addresses are given, the default address of localhost:6379 will be used.
//
// Calling Addresses overwrites any addresses previously set with HostAndPort or Options.
func (b *DataStoreBuilder) Addresses(addresses ...string) *DataStoreBuilder {
	if addresses == nil {
		b.redisOpts.Addrs = nil
	} else {
		copied := make([]string, len(addresses))
		copy(copied, addresses)
		b.redisOpts.Addrs = copied
	}
	return b
}

// Prefix specifies a string that should be prepended to all Redis keys used by the data store.
// A colon will be added to this automatically. If this is unspecified or empty, DefaultPrefix will be used.
func (b *DataStoreBuilder) Prefix(prefix string) *DataStoreBuilder {
	if prefix == "" {
		prefix = DefaultPrefix
	}
	b.prefix = prefix
	return b
}

// Options sets all of the parameters supported by the go-redis UniversalOptions type.
//
// This overwrites any previous setting of HostAndPort or Addresses.
func (b *DataStoreBuilder) Options(options redis.UniversalOptions) *DataStoreBuilder {
	b.redisOpts = options
	return b
}

// URL specifies the Redis host URL. If not specified, the default value is DefaultURL.
//
// Note that some Redis client features can specified either as part of the URL or with Options. For instance,
// the Password and DB fields in Options can be part of a "redis://" URL
// (https://www.iana.org/assignments/uri-schemes/prov/redis), and TLS can be enabled either by setting the
// TLSConfig in Options or by using a "rediss://" URL (https://www.iana.org/assignments/uri-schemes/prov/rediss).
//
// To use multiple Redis hosts in cluster mode, use Options and set the Addrs field.
//
// Specifying an invalid URL will cause an error when the SDK is started.
func (b *DataStoreBuilder) URL(url string) *DataStoreBuilder {
	b.url = url
	return b
}

// Master sets the master hostname, when using Redis Sentinel.
func (b *DataStoreBuilder) Master(masterName string) *DataStoreBuilder {
	b.redisOpts.MasterName = masterName
	return b
}

// CreatePersistentDataStore is called internally by the SDK to create a data store implementation object.
func (b *DataStoreBuilder) CreatePersistentDataStore(
	context interfaces.ClientContext,
) (interfaces.PersistentDataStore, error) {
	client, redisOpts, prefix, err := b.validateAndCreateClient()
	if err != nil {
		return nil, err
	}
	loggers := context.GetLogging().GetLoggers()
	loggers.SetPrefix("RedisDataStore:")
	return &redisDataStoreImpl{
		client: client,
		redisOpts: redisOpts,
		prefix: prefix,
		loggers: loggers,
	}, nil
}

// CreateBigSegmentStore is called internally by the SDK to create a data store implementation object.
func (b *DataStoreBuilder) CreateBigSegmentStore(
	context interfaces.ClientContext,
) (interfaces.BigSegmentStore, error) {
	client, redisOpts, prefix, err := b.validateAndCreateClient()
	if err != nil {
		return nil, err
	}
	loggers := context.GetLogging().GetLoggers()
	loggers.SetPrefix("RedisBigSegmentStore:")
	return &redisBigSegmentStoreImpl{
		client: client,
		redisOpts: redisOpts,
		prefix: prefix,
		loggers: loggers,
	}, nil
}

func (b *DataStoreBuilder) validateAndCreateClient() (
	redis.UniversalClient, redis.UniversalOptions, string, error,
) {
	redisOpts := b.redisOpts
	
	if b.url != "" {
		if len(redisOpts.Addrs) > 0 {
			return nil, redisOpts, "",
				errors.New("Redis data store must be configured with either Options.Address or URL, but not both")
		}
		parsed, err := redis.ParseURL(b.url)
		if err != nil {
			return nil, redisOpts, "", err
		}
		redisOpts.DB = parsed.DB
		redisOpts.Addrs = []string{parsed.Addr}
		redisOpts.Username = parsed.Username
		redisOpts.Password = parsed.Password
	}

	if len(redisOpts.Addrs) == 0 {
		redisOpts.Addrs = []string{defaultAddress}
	}

	client := redis.NewUniversalClient(&redisOpts)

	if b.checkOnStartup {
		// Test connection and immediately fail initialization if it fails
		err := client.Ping(defaultContext()).Err()
		if err != nil {
			return nil, redisOpts, "", err
		}
	}

	prefix := b.prefix
	if len(redisOpts.Addrs) > 1 {
		if !strings.Contains(prefix, "{") {
			prefix = DefaultClusterPrefix + prefix
		}
	}

	return client, redisOpts, prefix, nil
}

// DescribeConfiguration is used internally by the SDK to inspect the configuration.
func (b *DataStoreBuilder) DescribeConfiguration() ldvalue.Value {
	return ldvalue.String("Redis")
}
