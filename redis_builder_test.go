package ldredis

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDataSourceBuilder(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		b := DataStore()
		assert.Equal(t, DefaultPrefix, b.prefix)
		assert.Equal(t, "", b.url)
	})

	t.Run("HostAndPort", func(t *testing.T) {
		b := DataStore().HostAndPort("mine", 4000)
		assert.Equal(t, []string{"mine:4000"}, b.redisOpts.Addrs)
	})

	t.Run("Prefix", func(t *testing.T) {
		b := DataStore().Prefix("p")
		assert.Equal(t, "p", b.prefix)

		b.Prefix("")
		assert.Equal(t, DefaultPrefix, b.prefix)
	})

	t.Run("URL", func(t *testing.T) {
		b := DataStore().URL("redis://mine")
		assert.Equal(t, "redis://mine", b.url)
	})
}
