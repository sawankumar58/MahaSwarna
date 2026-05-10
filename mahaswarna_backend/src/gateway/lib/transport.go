package lib

import (
	"net/http"
	"time"
)

// sharedTransport is the single HTTP transport used by all upstream callers
// (ResilientProxy and BFF HomeAggregator). One shared transport means one
// connection pool per host, maximising connection reuse and preventing
// duplicate pools from exhausting upstream connection limits.
var sharedTransport = &http.Transport{
	MaxIdleConns:          200,
	MaxIdleConnsPerHost:   50,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   5 * time.Second,
	ResponseHeaderTimeout: 10 * time.Second,
}

// SharedTransport returns the package-level shared HTTP transport.
// All upstream HTTP clients should use this to ensure a single connection pool.
func SharedTransport() *http.Transport {
	return sharedTransport
}
