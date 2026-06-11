package ldredis

import (
	"context"
	"errors"
	"github.com/launchdarkly/go-sdk-common/v3/ldlog"
	"github.com/launchdarkly/go-server-sdk/v7/subsystems/ldstoretypes"
	"github.com/redis/go-redis/v9"
)

const (
	defaultAddress = "localhost:6379"
	maxRetries     = 10
)

// Internal implementation of the PersistentDataStore interface for Redis.
type redisDataStoreImpl struct {
	client     redis.UniversalClient
	redisOpts  redis.UniversalOptions
	prefix     string
	loggers    ldlog.Loggers
	testTxHook func()
}

const initedKey = "$inited"

// All go-redis operations take a Context parameter which allows the operation to be cancelled. For
// operations where we don't need to have a way to cancel them, we use defaultContext.
func defaultContext() context.Context {
	return context.Background()
}

func (store *redisDataStoreImpl) Init(allData []ldstoretypes.SerializedCollection) error {
	pipe := store.client.Pipeline()
	for _, coll := range allData {
		baseKey := store.keyForKind(coll.Kind)

		if err := pipe.Del(defaultContext(), baseKey).Err(); err != nil {
			return err
		}

		for _, keyedItem := range coll.Items {
			err := pipe.HSet(defaultContext(), baseKey, keyedItem.Key, keyedItem.Item.SerializedItem).Err()
			if err != nil {
				return err
			}
		}
	}

	if err := pipe.Set(defaultContext(), store.initedKey(), "", 0).Err(); err != nil {
		return err
	}
	_, err := pipe.Exec(defaultContext())
	return err
}

func (store *redisDataStoreImpl) Get(
	kind ldstoretypes.DataKind,
	key string,
) (ldstoretypes.SerializedItemDescriptor, error) {
	data, err := store.client.HGet(defaultContext(), store.keyForKind(kind), key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
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
	values, err := store.client.HGetAll(defaultContext(), store.keyForKind(kind)).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
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
	baseKey := store.keyForKind(kind)

	finished := false
	updated := false
	var retryErr error

	for availableRetries := maxRetries; availableRetries > 0; availableRetries-- {
		err := store.client.Watch(defaultContext(), func(tx *redis.Tx) error {
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

			result, err := tx.TxPipelined(defaultContext(), func(pipe redis.Pipeliner) error {
				err = pipe.HSet(defaultContext(), baseKey, key, newItem.SerializedItem).Err()
				if err == nil {
					result, err := pipe.Exec(defaultContext())
					// if exec returned nothing, it means the watch was triggered and we should retry
					if (err == nil && len(result) == 0) || errors.Is(err, redis.TxFailedErr) {
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
	inited, _ := store.client.Exists(defaultContext(), store.initedKey()).Result()
	return inited == 1
}

func (store *redisDataStoreImpl) IsStoreAvailable() bool {
	_, err := store.client.Exists(defaultContext(), store.initedKey()).Result()
	return err == nil
}

func (store *redisDataStoreImpl) Close() error {
	return store.client.Close()
}

// Computes the key that is used for all items of the specified kind. The value of this key in
// Redis is a hash where each field name is the item key and the field value is the serialized
// item.
func (store *redisDataStoreImpl) keyForKind(kind ldstoretypes.DataKind) string {
	return store.prefix + ":" + kind.GetName()
}

// Computes the special key that is used to indicate that the data store contains data.
func (store *redisDataStoreImpl) initedKey() string {
	return store.prefix + ":" + initedKey
}
