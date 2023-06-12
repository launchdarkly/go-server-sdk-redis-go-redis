# LaunchDarkly Server-side SDK for Go - Redis integration with go-redis

[![Circle CI](https://circleci.com/gh/launchdarkly/go-server-sdk-redis-go-redis.svg?style=shield)](https://circleci.com/gh/launchdarkly/go-server-sdk-redis-go-redis) [![Documentation](https://img.shields.io/static/v1?label=go.dev&message=reference&color=00add8)](https://pkg.go.dev/github.com/launchdarkly/go-server-sdk-redis-go-redis)

_This version of the SDK is a beta version and should not be considered ready for production use while this message is visible._

This library provides a [Redis](https://redis.io/)-backed persistence mechanism (data store) for the [LaunchDarkly Go SDK](https://github.com/launchdarkly/go-server-sdk), replacing the default in-memory data store.

The Redis client implementation it uses is [`go-redis`](https://github.com/redis/go-redis). This distinguishes it from the other Go SDK Redis integration, [`go-server-sdk-redis-redigo`](https://github.com/launchdarkly/go-server-sdk-redis-redigo), which uses the [`redigo`](https://github.com/gomodule/redigo) client (therefore the two projects have somewhat long and repetitive names). The main difference between the two is that `go-redis` supports cluster mode and `redigo` does not.

This version of the library requires at least version 6.0.0 of the LaunchDarkly Go SDK.

The minimum Go version is 1.19.

For more information, see also: [Using a persistent feature store](https://docs.launchdarkly.com/sdk/concepts/feature-store).

## Quick setup

This assumes that you have already installed the LaunchDarkly Go SDK.

1. Import the LaunchDarkly SDK packages and the package for this library:

```go
import (
    ld "github.com/launchdarkly/go-server-sdk/v6"
    "github.com/launchdarkly/go-server-sdk/v6/ldcomponents"
    ldredis "github.com/launchdarkly/go-server-sdk-redis-go-redis"
)
```

2. When configuring your SDK client, add the Redis data store as a `PersistentDataStore`. You may specify any custom Redis options using the methods of `RedisDataStoreBuilder`. For instance, to customize the Redis URL:

```go
    var config ld.Config{}
    config.DataStore = ldcomponents.PersistentDataStore(
        ldredis.DataStore().URL("redis://my-redis-host"),
    )
```

By default, the store will try to connect to a local Redis instance on port 6379.

To use cluster mode or other advanced `go-redis` features, use the `Options` method to pass a complete Redis [client configuration](https://pkg.go.dev/github.com/go-redis/redis/v8?tab=doc#UniversalOptions):

```go
import (
    goredis "github.com/go-redis/redis/v7"
)

    redisOpts := goredis.UniversalOptions{
        Addrs: []string{ "cluster-host-1:6379", "cluster-host-2:6379" },
    }
    config.DataStore = ldcomponents.PersistentDataStore(
        ldredis.DataStore().Options(redisOpts),
    )
```

## Caching behavior

The LaunchDarkly SDK has a standard caching mechanism for any persistent data store, to reduce database traffic. This is configured through the SDK's `PersistentDataStoreBuilder` class as described the SDK documentation. For instance, to specify a cache TTL of 5 minutes:

```go
    var config ld.Config{}
    config.DataStore = ldcomponents.PersistentDataStore(
        ldredis.DataStore(),
    ).CacheMinutes(5)
```

## LaunchDarkly overview

[LaunchDarkly](https://www.launchdarkly.com) is a feature management platform that serves trillions of feature flags daily to help teams build better software, faster. [Get started](https://docs.launchdarkly.com/docs/getting-started) using LaunchDarkly today!

## About LaunchDarkly

* LaunchDarkly is a continuous delivery platform that provides feature flags as a service and allows developers to iterate quickly and safely. We allow you to easily flag your features and manage them from the LaunchDarkly dashboard.  With LaunchDarkly, you can:
    * Roll out a new feature to a subset of your users (like a group of users who opt-in to a beta tester group), gathering feedback and bug reports from real-world use cases.
    * Gradually roll out a feature to an increasing percentage of users, and track the effect that the feature has on key metrics (for instance, how likely is a user to complete a purchase if they have feature A versus feature B?).
    * Turn off a feature that you realize is causing performance problems in production, without needing to re-deploy, or even restart the application with a changed configuration file.
    * Grant access to certain features based on user attributes, like payment plan (eg: users on the ‘gold’ plan get access to more features than users in the ‘silver’ plan). Disable parts of your application to facilitate maintenance, without taking everything offline.
* LaunchDarkly provides feature flag SDKs for a wide variety of languages and technologies. Check out [our documentation](https://docs.launchdarkly.com/docs) for a complete list.
* Explore LaunchDarkly
    * [launchdarkly.com](https://www.launchdarkly.com/ "LaunchDarkly Main Website") for more information
    * [docs.launchdarkly.com](https://docs.launchdarkly.com/  "LaunchDarkly Documentation") for our documentation and SDK reference guides
    * [apidocs.launchdarkly.com](https://apidocs.launchdarkly.com/  "LaunchDarkly API Documentation") for our API documentation
    * [blog.launchdarkly.com](https://blog.launchdarkly.com/  "LaunchDarkly Blog Documentation") for the latest product updates
