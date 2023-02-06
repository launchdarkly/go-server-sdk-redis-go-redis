package ldredis

import (
	"context"
	"github.com/go-redis/redis/v8"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/launchdarkly/go-server-sdk/v6/subsystems"
	"github.com/launchdarkly/go-server-sdk/v6/testhelpers/storetest"
)

func TestRedisDataStore(t *testing.T) {
	storetest.NewPersistentDataStoreTestSuite(makeTestStore, clearTestData).
		ErrorStoreFactory(makeFailedStore(), verifyFailedStoreError).
		ConcurrentModificationHook(setConcurrentModificationHook).
		Run(t)
}

func getTestAddresses() []string {
	if s := os.Getenv("LD_TEST_REDIS_ADDRESSES"); s != "" {
		return strings.Split(s, " ")
	}
	return []string{defaultAddress}
}

func isClusterMode() bool {
	return len(getTestAddresses()) > 1
}

func makeClientOptions() *redis.UniversalOptions {
	return &redis.UniversalOptions{Addrs: getTestAddresses()}
}

func makeTestStore(prefix string) subsystems.ComponentConfigurer[subsystems.PersistentDataStore] {
	return DataStore().Prefix(prefix).Options(*makeClientOptions())
}

func makeFailedStore() subsystems.ComponentConfigurer[subsystems.PersistentDataStore] {
	// Here we ensure that all Redis operations will fail by using an invalid hostname.
	return DataStore().URL("redis://not-a-real-host").CheckOnStartup(false)
}

func verifyFailedStoreError(t assert.TestingT, err error) {
	assert.Contains(t, err.Error(), "no such host")
}

func clearTestData(prefix string) error {
	if prefix == "" {
		prefix = DefaultPrefix
	}

	// The SCAN command (which we only use in this test code, not in the actual integration) needs
	// to be handled differently depending on whether we're using a cluster or not.

	deleteAllKeys := func(client redis.Cmdable) error {
		var allKeys []string
		iter := client.Scan(defaultContext(), 0, prefix+":*", 0).Iterator()
		for iter.Next(defaultContext()) {
			allKeys = append(allKeys, iter.Val())
		}
		if iter.Err() != nil {
			return iter.Err()
		}
		if len(allKeys) == 0 {
			return nil
		}
		return client.Del(defaultContext(), allKeys...).Err()
	}

	if isClusterMode() {
		prefix = DefaultClusterPrefix + prefix
		clusterClient := redis.NewClusterClient(&redis.ClusterOptions{Addrs: getTestAddresses()})
		defer clusterClient.Close()
		return clusterClient.ForEachMaster(defaultContext(), func(ctx context.Context, client *redis.Client) error {
			return deleteAllKeys(client)
		})
	} else {
		client := redis.NewUniversalClient(makeClientOptions())
		defer client.Close()
		return deleteAllKeys(client)
	}
}

func setConcurrentModificationHook(store subsystems.PersistentDataStore, hook func()) {
	store.(*redisDataStoreImpl).testTxHook = hook
}
