module github.com/mahaswarna/gateway

go 1.23

require (
	github.com/go-chi/chi/v5 v5.1.0
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/mahaswarna/contracts v0.0.0
	github.com/mahaswarna/infrastructure v0.0.0
	github.com/mahaswarna/observability v0.0.0
	github.com/mahaswarna/shared v0.0.0
	github.com/redis/go-redis/v9 v9.7.0
	github.com/segmentio/ksuid v1.0.4
	github.com/sony/gobreaker v1.0.0
)

require (
	github.com/alicebob/miniredis/v2 v2.33.0 // indirect; test only
)

replace (
	github.com/mahaswarna/contracts      => ../contracts
	github.com/mahaswarna/infrastructure => ../infrastructure
	github.com/mahaswarna/observability  => ../observability
	github.com/mahaswarna/shared         => ../shared
)
