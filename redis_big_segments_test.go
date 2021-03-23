package ldredis

import (
	"fmt"
	"strings"
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
		prefix = realTestPrefix(prefix)
		return client.Set(defaultContext(), bigSegmentsSyncTimeKey(prefix),
			fmt.Sprintf("%d", metadata.LastUpToDate), 0).Err()
	}

	setTestSegments := func(prefix string, userHashKey string, included []string, excluded []string) error {
		prefix = realTestPrefix(prefix)
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

func realTestPrefix(prefix string) string {
	if prefix == "" {
		prefix = DefaultPrefix
	}
	if len(getTestAddresses()) > 1 {
		if !strings.Contains(prefix, "{") {
			prefix = DefaultClusterPrefix + prefix
		}
	}
	return prefix
}
