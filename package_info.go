// Package ldredis provides a Redis-backed persistent data store for the LaunchDarkly Go SDK.
//
// For more details about how and why you can use a persistent data store, see:
// https://docs.launchdarkly.com/sdk/concepts/data-stores
//
// To use the Redis data store with the LaunchDarkly client:
//
//	import ldredis "github.com/launchdarkly/go-server-sdk-redis-go-redis"
//
//	config := ld.Config{
//	    DataStore: ldcomponents.PersistentDataStore(ldredis.DataStore()),
//	}
//	client, err := ld.MakeCustomClient("sdk-key", config, 5*time.Second)
//
// The default configuration uses an address of localhost:6379. You may customize the configuration
// by using the methods of the ldredis.DataStoreBuilder returned by ldredis.DataStore():
//
//	config := ld.Config{
//	    DataStore: ldcomponents.PersistentDataStore(
//	        ldredis.DataStore().URL(myRedisURL),
//	    ).CacheSeconds(30),
//	}
//
// Note that CacheSeconds() is not a method of ldredis.DataStoreBuilder, but rather a method of
// ldcomponents.PersistentDataStore(), because the caching behavior is provided by the SDK for
// all database integrations.
//
// For advanced customization of the underlying go-redis client, use the Options method with
// ldredis.DataStore(). Note that some Redis client features can also be specified as part of the
// URL.
//
// If you are also using Redis for other purposes, the data store can coexist with
// other data as long as you are not using the same keys. By default, the keys used by the
// data store will always start with "launchdarkly:"; you can change this to another
// prefix if desired using the ldredis.DataStoreBuilder's Prefix method.
package ldredis
