package ldredis

import (
	"fmt"

	"github.com/go-redis/redis/v7"

	"gopkg.in/launchdarkly/go-sdk-common.v2/ldvalue"
	"gopkg.in/launchdarkly/go-server-sdk.v5/interfaces"
)

const (
	// DefaultPrefix is a string that is prepended (along with a colon) to all Redis keys used
	// by the data store. You can change this value with the Prefix() option for
	// NewRedisDataStoreWithDefaults, or with the "prefix" parameter to the other constructors.
	DefaultPrefix = "launchdarkly"
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
// To use multiple Redis hosts in cluster mode, use Options and set the Addrs field.
func (b *DataStoreBuilder) HostAndPort(host string, port int) *DataStoreBuilder {
	b.redisOpts.Addrs = []string{fmt.Sprintf("%s:%d", host, port)}
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

// CreatePersistentDataStore is called internally by the SDK to create the data store implementation object.
func (b *DataStoreBuilder) CreatePersistentDataStore(
	context interfaces.ClientContext,
) (interfaces.PersistentDataStore, error) {
	store, err := newRedisDataStoreImpl(b, context.GetLogging().GetLoggers())
	return store, err
}

// DescribeConfiguration is used internally by the SDK to inspect the configuration.
func (b *DataStoreBuilder) DescribeConfiguration() ldvalue.Value {
	return ldvalue.String("Redis")
}
