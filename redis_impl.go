package ldredis

import (
	"errors"

	"github.com/go-redis/redis/v7"

	"gopkg.in/launchdarkly/go-sdk-common.v2/ldlog"
	"gopkg.in/launchdarkly/go-server-sdk.v5/interfaces/ldstoretypes"
)

const (
	defaultAddress = "localhost:6379"
	hashTag        = "{ld}."
	maxRetries     = 10
)

// Internal implementation of the PersistentDataStore interface for Redis.
type redisDataStoreImpl struct {
	client     redis.UniversalClient
	prefix     string
	cluster    bool
	loggers    ldlog.Loggers
	testTxHook func()
}

const initedKey = "$inited"

func newRedisDataStoreImpl(
	builder *DataStoreBuilder,
	loggers ldlog.Loggers,
) (*redisDataStoreImpl, error) {
	redisOpts := builder.redisOpts

	if builder.url != "" {
		if len(redisOpts.Addrs) > 0 {
			return nil, errors.New("Redis data store must be configured with either Options.Address or URL, but not both")
		}
		parsed, err := redis.ParseURL(builder.url)
		if err != nil {
			return nil, err
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

	if builder.checkOnStartup {
		// Test connection and immediately fail initialization if it fails
		err := client.Ping().Err()
		if err != nil {
			return nil, err
		}
	}

	impl := &redisDataStoreImpl{
		client:  client,
		prefix:  builder.prefix,
		loggers: loggers,
	}

	impl.loggers.SetPrefix("RedisDataStore:")

	if len(redisOpts.Addrs) > 1 {
		impl.cluster = true
	}

	return impl, nil
}

func (store *redisDataStoreImpl) Init(allData []ldstoretypes.SerializedCollection) error {
	pipe := store.client.Pipeline()
	for _, coll := range allData {
		baseKey := store.featuresKey(coll.Kind)

		if err := pipe.Del(baseKey).Err(); err != nil {
			return err
		}

		for _, keyedItem := range coll.Items {
			err := pipe.HSet(baseKey, store.hashTagKey(keyedItem.Key), keyedItem.Item.SerializedItem).Err()
			if err != nil {
				return err
			}
		}
	}

	if err := pipe.Set(store.initedKey(), "", 0).Err(); err != nil {
		return err
	}
	_, err := pipe.Exec()
	return err
}

func (store *redisDataStoreImpl) Get(
	kind ldstoretypes.DataKind,
	key string,
) (ldstoretypes.SerializedItemDescriptor, error) {
	data, err := store.client.HGet(store.featuresKey(kind), store.hashTagKey(key)).Result()
	if err != nil {
		if err == redis.Nil {
			store.loggers.Debugf("Key: %s not found in \"%s\"", key, kind.GetName())
			return ldstoretypes.SerializedItemDescriptor{}.NotFound(), nil
		}
		return ldstoretypes.SerializedItemDescriptor{}.NotFound(), err
	}

	return ldstoretypes.SerializedItemDescriptor{Version: 0, SerializedItem: []byte(data)}, nil
}

func (store *redisDataStoreImpl) GetAll(
	kind ldstoretypes.DataKind,
) ([]ldstoretypes.KeyedSerializedItemDescriptor, error) {
	values, err := store.client.HGetAll(store.featuresKey(kind)).Result()
	if err != nil && err != redis.Nil {
		return nil, err
	}

	results := make([]ldstoretypes.KeyedSerializedItemDescriptor, 0, len(values))
	for k, v := range values {
		results = append(results, ldstoretypes.KeyedSerializedItemDescriptor{
			Key:  k,
			Item: ldstoretypes.SerializedItemDescriptor{Version: 0, SerializedItem: []byte(v)},
		})
	}
	return results, nil
}

func (store *redisDataStoreImpl) Upsert(
	kind ldstoretypes.DataKind,
	key string,
	newItem ldstoretypes.SerializedItemDescriptor,
) (bool, error) {
	baseKey := store.featuresKey(kind)

	finished := false
	updated := false
	var retryErr error

	for availableRetries := maxRetries; availableRetries > 0; availableRetries-- {
		err := store.client.Watch(func(tx *redis.Tx) error {
			oldItem, err := store.Get(kind, key)
			if err != nil {
				return err
			}

			if store.testTxHook != nil { // instrumentation for unit tests
				store.testTxHook()
			}

			// In this implementation, we have to parse the existing item in order to determine its version.
			oldVersion := oldItem.Version
			if oldItem.SerializedItem != nil {
				parsed, _ := kind.Deserialize(oldItem.SerializedItem)
				oldVersion = parsed.Version
			}

			if oldVersion >= newItem.Version {
				updateOrDelete := "update"
				if newItem.Deleted {
					updateOrDelete = "delete"
				}
				store.loggers.Debugf(`Attempted to %s key: %s version: %d in "%s" with a version that is the same or older: %d`,
					updateOrDelete, key, oldItem.Version, kind.GetName(), newItem.Version)
				finished = true
				return nil
			}

			result, err := tx.TxPipelined(func(pipe redis.Pipeliner) error {
				err = pipe.HSet(baseKey, store.hashTagKey(key), newItem.SerializedItem).Err()
				if err == nil {
					result, err := pipe.Exec()
					// if exec returned nothing, it means the watch was triggered and we should retry
					if (err == nil && len(result) == 0) || err == redis.TxFailedErr {
						store.loggers.Debug("Concurrent modification detected, retrying")
						return nil
					}
					if err != nil {
						return err
					}
					finished = true
					updated = true
				} else {
					return err
				}
				return nil // end Pipeline
			})
			if err != nil {
				return err // Pipeline error
			}
			if len(result) > 0 {
				return result[0].Err() // Pipeline failed
			}
			return nil //end WATCH
		}, baseKey)
		if err != nil {
			return false, err
		}
		if finished {
			return updated, nil
		}
	}
	return false, retryErr
}

func (store *redisDataStoreImpl) IsInitialized() bool {
	inited, _ := store.client.Exists(store.initedKey()).Result()
	return inited == 1
}

func (store *redisDataStoreImpl) IsStoreAvailable() bool {
	_, err := store.client.Exists(store.initedKey()).Result()
	return err == nil
}

func (store *redisDataStoreImpl) Close() error {
	return store.client.Close()
}

func (store *redisDataStoreImpl) featuresKey(kind ldstoretypes.DataKind) string {
	return store.prefix + ":" + kind.GetName()
}

func (store *redisDataStoreImpl) initedKey() string {
	return store.prefix + ":" + initedKey
}

// We use a hashtag in order to keep all keys in the same node (and hash slot) so we can perform
// and use watch ... exec without issues. Only in ClusterMode
func (store *redisDataStoreImpl) hashTagKey(key string) string {
	if store.cluster {
		return hashTag + key
	}
	return key
}
