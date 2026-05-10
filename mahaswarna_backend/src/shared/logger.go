package shared

import "log/slog"

// Logger is the shared structured logger used across all services.
// Services call shared.Logger.Info / Error / Warn directly.
var Logger = slog.Default()
