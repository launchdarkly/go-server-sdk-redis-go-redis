package ldredis

import (
	"testing"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"

	"gopkg.in/launchdarkly/go-server-sdk.v5/interfaces"
	"gopkg.in/launchdarkly/go-server-sdk.v5/testhelpers/storetest"
)

func TestRedisDataStore(t *testing.T) {
	storetest.NewPersistentDataStoreTestSuite(makeTestStore, clearTestData).
		ErrorStoreFactory(makeFailedStore(), verifyFailedStoreError).
		ConcurrentModificationHook(setConcurrentModificationHook).
		Run(t)
}

func makeClientOptions() *redis.UniversalOptions {
	return &redis.UniversalOptions{Addrs: []string{defaultAddress}}
}

func makeTestStore(prefix string) interfaces.PersistentDataStoreFactory {
	return DataStore().Prefix(prefix).Options(*makeClientOptions())
}

func makeFailedStore() interfaces.PersistentDataStoreFactory {
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

	client := redis.NewUniversalClient(makeClientOptions())
	defer client.Close()

	var allKeys []string

	cursor := uint64(0)
	for {
		cmd := client.Scan(defaultContext(), cursor, prefix+":*", 0)
		keys, nextCursor, err := cmd.Result()
		if err != nil {
			return err
		}
		allKeys = append(allKeys, keys...)
		if nextCursor == 0 { // SCAN returns 0 when the current result subset is the last one
			break
		}
		cursor = nextCursor
	}

	if len(allKeys) == 0 {
		return nil
	}
	return client.Del(defaultContext(), allKeys...).Err()
}

func setConcurrentModificationHook(store interfaces.PersistentDataStore, hook func()) {
	store.(*redisDataStoreImpl).testTxHook = hook
}
