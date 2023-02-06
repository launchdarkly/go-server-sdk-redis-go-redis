package ldredis

import (
	"github.com/launchdarkly/go-server-sdk/v6/subsystems"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDataSourceBuilder(t *testing.T) {
	makeStore := func(b *DataStoreBuilder) *redisDataStoreImpl {
		b.CheckOnStartup(false)
		store, err := b.Build(subsystems.BasicClientContext{})
		require.NoError(t, err)
		return store.(*redisDataStoreImpl)
	}

	t.Run("defaults", func(t *testing.T) {
		store := makeStore(DataStore())
		assert.Equal(t, DefaultPrefix, store.prefix)
		assert.Equal(t, []string{defaultAddress}, store.redisOpts.Addrs)
	})

	t.Run("HostAndPort", func(t *testing.T) {
		b := DataStore().HostAndPort("mine", 4000)
		assert.Equal(t, []string{"mine:4000"}, makeStore(b).redisOpts.Addrs)
	})

	t.Run("Prefix", func(t *testing.T) {
		assert.Equal(t, "p", makeStore(DataStore().Prefix("p")).prefix)

		assert.Equal(t, DefaultPrefix, makeStore(DataStore().Prefix("")).prefix)

		// assert.Equal(t, DefaultClusterPrefix + "p", makeStore(DataStore().Prefix("p").))
	})

	t.Run("URL", func(t *testing.T) {
		b := DataStore().URL("redis://mine")
		assert.Equal(t, "redis://mine", b.url)
	})
}
