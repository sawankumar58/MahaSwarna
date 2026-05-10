package middleware

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/mahaswarna/shared"
	"github.com/mahaswarna/shared/types"
)

const headerAppVersion = "X-App-Version"

// APIVersionValidator enforces the API version contract using the Accept-Version header.
//
// Supported versions:   {"v1"}
// Deprecated versions:  {} (empty until the 90-day /v1/ sunset window closes)
//
// Behaviour:
//   - Missing header → treated as "v1" (backwards-compatible).
//   - Deprecated version → 410 Gone with error "api_version_deprecated".
//   - Unrecognised version (e.g. client bug sending "v3") → 400 Bad Request.
//   - Supported version → passes through.
//
// MUST be registered before JWTPreValidator so a deprecated client is blocked
// before any token processing occurs. See ARCHITECTURE.md §Gateway.
//
// Client (Android) ApiErrorMapper:
//   HTTP 410 (any body)                          → ApiError.VersionDeprecated
//   HTTP 400 + error=="unsupported_api_version"  → ApiError.VersionDeprecated
//   Both → non-dismissible UpdateRequiredScreen (deep-link Play Store).
func APIVersionValidator(next http.Handler) http.Handler {
	supported  := map[string]struct{}{"v1": {}}
	deprecated := map[string]struct{}{} // populated when /v2/ ships and /v1/ enters sunset

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		version := r.Header.Get("Accept-Version")
		if version == "" {
			version = "v1" // missing header → assume v1 (legacy clients)
		}

		if _, ok := supported[version]; ok {
			next.ServeHTTP(w, r)
			return
		}

		if _, ok := deprecated[version]; ok {
			writeError(w, http.StatusGone, "api_version_deprecated",
				"this API version is no longer supported; please update the app")
			return
		}

		writeError(w, http.StatusBadRequest, "unsupported_api_version",
			"unrecognised API version; please update the app")
	})
}

// VersionValidator rejects requests whose X-App-Version is below MIN_APP_VERSION.
//
// Version format: MAJOR.MINOR.PATCH (semver-like). Comparison is purely numeric
// left-to-right; pre-release labels are ignored.
//
// If MIN_APP_VERSION is unset the middleware is a no-op (allows all versions).
// If the header is absent the middleware lets the request through — old clients
// that pre-date the version header should be handled by a force-update screen
// driven by the /v1/flags endpoint, not rejected at the gateway layer.
func VersionValidator(next http.Handler) http.Handler {
	minRaw := os.Getenv("MIN_APP_VERSION")
	if minRaw == "" {
		return next // no minimum configured
	}

	minParts, err := parseSemver(minRaw)
	if err != nil {
		shared.Logger.Warn("VersionValidator: invalid MIN_APP_VERSION, skipping enforcement",
			"value", minRaw, "err", err)
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := r.Header.Get(headerAppVersion)
		if raw == "" {
			// Missing header → pass through; force-update handled by flags.
			next.ServeHTTP(w, r)
			return
		}

		clientParts, err := parseSemver(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_version",
				fmt.Sprintf("X-App-Version %q is not a valid version", raw))
			return
		}

		if semverLess(clientParts, minParts) {
			writeError(w, http.StatusUpgradeRequired, "app_update_required",
				fmt.Sprintf("minimum supported version is %s; please update the app", minRaw))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// parseSemver splits "1.2.3" into [1, 2, 3]. Trailing labels (e.g. "-beta") stripped.
func parseSemver(v string) ([3]int, error) {
	// Strip build/pre-release labels.
	v = strings.Split(v, "-")[0]
	v = strings.Split(v, "+")[0]

	parts := strings.SplitN(v, ".", 3)
	for len(parts) < 3 {
		parts = append(parts, "0")
	}

	var out [3]int
	for i, p := range parts[:3] {
		n, err := strconv.Atoi(p)
		if err != nil {
			return out, fmt.Errorf("component %d %q: %w", i, p, err)
		}
		out[i] = n
	}
	return out, nil
}

func semverLess(a, b [3]int) bool {
	for i := 0; i < 3; i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false // equal
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	resp := types.Fail[struct{}](code, msg)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = encodeJSON(w, resp)
}
