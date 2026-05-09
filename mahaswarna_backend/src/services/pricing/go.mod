module github.com/mahaswarna/pricing

go 1.23

require (
	github.com/go-chi/chi/v5 v5.1.0
	github.com/mahaswarna/contracts v0.0.0
	github.com/mahaswarna/infrastructure v0.0.0
	github.com/mahaswarna/observability v0.0.0
	github.com/mahaswarna/shared v0.0.0
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/gorilla/websocket v1.5.3
	github.com/jackc/pgx/v5 v5.7.1
	github.com/redis/go-redis/v9 v9.7.0
	github.com/robfig/cron/v3 v3.0.1
	google.golang.org/api v0.210.0
)

replace (
	github.com/mahaswarna/contracts      => ../../contracts
	github.com/mahaswarna/infrastructure => ../../infrastructure
	github.com/mahaswarna/observability  => ../../observability
	github.com/mahaswarna/shared         => ../../shared
)

