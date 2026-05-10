package gateway_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/mahaswarna/gateway/lib"
)

func TestFallbackCache_StoreAndServeStale(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	cache := lib.NewFallbackCache(rdb, 0) // 0 TTL → use default

	body := []byte(`{"ok":true,"data":{"gold":72000,"silver":85000}}`)
	cache.Store(ctx, "test:key", body)

	rr := httptest.NewRecorder()
	served := cache.ServeStale(ctx, "test:key", rr)
	if !served {
		t.Fatal("expected ServeStale to return true")
	}
	if rr.Body.String() != string(body) {
		t.Errorf("body mismatch\ngot:  %s\nwant: %s", rr.Body.String(), string(body))
	}
	if rr.Header().Get("X-Cache") != "STALE" {
		t.Error("expected X-Cache: STALE header")
	}
}

func TestFallbackCache_MissingKeyReturnsFalse(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	cache := lib.NewFallbackCache(rdb, 0)

	rr := httptest.NewRecorder()
	served := cache.ServeStale(ctx, "nonexistent:key", rr)
	if served {
		t.Error("expected ServeStale to return false for missing key")
	}
}
