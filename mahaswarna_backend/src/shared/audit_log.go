package shared

// AuditEntry is the canonical audit log payload written by AuditLogRepository.Append.
// All six fields should be populated; EntityID and Metadata may be empty/nil.
type AuditEntry struct {
	Actor    string         // user ID, phone, or "system"
	Action   string         // e.g. "login", "account_deleted", "flag_updated"
	Entity   string         // table name: "users", "alerts", "feature_flags"
	EntityID string         // primary key of the affected row
	Metadata map[string]any // arbitrary extra context (JSON-serialised by repo)
}
