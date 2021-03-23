package ldredis

import (
	"fmt"
	"strconv"

	"github.com/go-redis/redis/v8"

	"gopkg.in/launchdarkly/go-sdk-common.v2/ldlog"
	"gopkg.in/launchdarkly/go-sdk-common.v2/ldtime"
	"gopkg.in/launchdarkly/go-server-sdk.v5/interfaces"
	"gopkg.in/launchdarkly/go-server-sdk.v5/ldcomponents/ldstoreimpl"
)

// Internal implementation of the BigSegmentStore interface for Redis.
type redisBigSegmentStoreImpl struct {
	client    redis.UniversalClient
	redisOpts redis.UniversalOptions
	prefix    string
	loggers   ldlog.Loggers
}

func (store *redisBigSegmentStoreImpl) GetMetadata() (interfaces.BigSegmentStoreMetadata, error) {
	valueStr, err := store.client.Get(defaultContext(), bigSegmentsSyncTimeKey(store.prefix)).Result()
	if err != nil {
		return interfaces.BigSegmentStoreMetadata{}, err
	}

	value, err := strconv.ParseUint(valueStr, 10, 64)
	if err != nil {
		return interfaces.BigSegmentStoreMetadata{}, err
	}

	return interfaces.BigSegmentStoreMetadata{
		LastUpToDate: ldtime.UnixMillisecondTime(value),
	}, nil
}

func (store *redisBigSegmentStoreImpl) GetUserMembership(
	userHashKey string,
) (interfaces.BigSegmentMembership, error) {
	includedRefs, err := store.client.SMembers(defaultContext(), 
		bigSegmentsIncludeKey(store.prefix, userHashKey)).Result()
	if err != nil {
		return nil, err
	}
	excludedRefs, err := store.client.SMembers(defaultContext(),
		bigSegmentsExcludeKey(store.prefix, userHashKey)).Result()
	if err != nil {
		return nil, err
	}

	return ldstoreimpl.NewBigSegmentMembershipFromSegmentRefs(includedRefs, excludedRefs), nil
}

func (store *redisBigSegmentStoreImpl) Close() error {
	return store.client.Close()
}

func bigSegmentsSyncTimeKey(prefix string) string {
	return fmt.Sprintf("%s:big_segments_synchronized_on", prefix)
}

func bigSegmentsIncludeKey(prefix, userHashKey string) string {
	return fmt.Sprintf("%s:big_segment_include:%s", prefix, userHashKey)
}

func bigSegmentsExcludeKey(prefix, userHashKey string) string {
	return fmt.Sprintf("%s:big_segment_exclude:%s", prefix, userHashKey)
}
