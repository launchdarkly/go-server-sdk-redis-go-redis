package ldredis

import (
	"fmt"
	"testing"

	"gopkg.in/launchdarkly/go-server-sdk.v5/interfaces"
	"gopkg.in/launchdarkly/go-server-sdk.v5/testhelpers/storetest"

	"github.com/go-redis/redis/v8"
)

func TestBigSegmentStore(t *testing.T) {
	redisOpts := redis.UniversalOptions{
		Addrs: getTestAddresses(),
	}
	client := redis.NewUniversalClient(&redisOpts)
	defer client.Close()

	setTestMetadata := func(prefix string, metadata interfaces.BigSegmentStoreMetadata) error {
		if prefix == "" {
			prefix = DefaultPrefix
		}
		return client.Set(defaultContext(), bigSegmentsSyncTimeKey(prefix),
			fmt.Sprintf("%d", metadata.LastUpToDate), 0).Err()
	}

	setTestSegments := func(prefix string, userHashKey string, included []string, excluded []string) error {
		if prefix == "" {
			prefix = DefaultPrefix
		}
		for _, inc := range included {
			err := client.SAdd(defaultContext(), bigSegmentsIncludeKey(prefix, userHashKey), inc).Err()
			if err != nil {
				return err
			}
		}
		for _, exc := range excluded {
			err := client.SAdd(defaultContext(), bigSegmentsExcludeKey(prefix, userHashKey), exc).Err()
			if err != nil {
				return err
			}
		}
		return nil
	}

	storetest.NewBigSegmentStoreTestSuite(
		func(prefix string) interfaces.BigSegmentStoreFactory {
			return DataStore().Addresses(getTestAddresses()...).Prefix(prefix)
		},
		clearTestData,
		setTestMetadata,
		setTestSegments,
	).Run(t)
}
