package ldredis

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/redis/go-redis/v9"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/launchdarkly/go-server-sdk/v7/subsystems"
	"github.com/launchdarkly/go-server-sdk/v7/subsystems/ldstoreimpl"
	"github.com/launchdarkly/go-server-sdk/v7/subsystems/ldstoretypes"
	"github.com/launchdarkly/go-server-sdk/v7/testhelpers/storetest"
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
	assert.Contains(t, err.Error(), "lookup")
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
		defer clusterClient.Close() //nolint:errcheck // test cleanup
		return clusterClient.ForEachMaster(defaultContext(), func(ctx context.Context, client *redis.Client) error {
			return deleteAllKeys(client)
		})
	} else {
		client := redis.NewUniversalClient(makeClientOptions())
		defer client.Close() //nolint:errcheck // test cleanup
		return deleteAllKeys(client)
	}
}

func setConcurrentModificationHook(store subsystems.PersistentDataStore, hook func()) {
	store.(*redisDataStoreImpl).testTxHook = hook
}

func TestUpsertGivesUpAfterMaxRetries(t *testing.T) {
	prefix := "test-upsert-gives-up"
	require.NoError(t, clearTestData(prefix))

	store, err := makeTestStore(prefix).Build(subsystems.BasicClientContext{})
	require.NoError(t, err)
	defer store.Close() //nolint:errcheck // test cleanup

	impl := store.(*redisDataStoreImpl)
	kind := ldstoreimpl.Features()
	hashKey := impl.keyForKind(kind)

	// Touch the watched hash from a separate client on every attempt, so each transaction sees a
	// concurrent modification and the key looks perpetually contended.
	otherClient := redis.NewUniversalClient(makeClientOptions())
	defer otherClient.Close() //nolint:errcheck // test cleanup
	attempts := 0
	impl.testTxHook = func() {
		attempts++
		require.NoError(t, otherClient.HSet(defaultContext(), hashKey, "flag-key", `{"key":"flag-key","version":1}`).Err())
	}

	updated, err := store.Upsert(kind, "flag-key", ldstoretypes.SerializedItemDescriptor{
		Version:        2,
		SerializedItem: []byte(`{"key":"flag-key","version":2}`),
	})

	require.False(t, updated, "an abandoned update must not be reported as an update")
	require.EqualError(t, err, `failed to update key "flag-key" in "features" after 10 attempts`)
	require.Equal(t, maxRetries, attempts, "Upsert should make exactly maxRetries attempts")
}
