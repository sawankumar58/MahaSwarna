package types

// PageRequest carries cursor-based pagination parameters.
// Use Page + Size for offset-based endpoints; Cursor for stream-style endpoints.
type PageRequest struct {
	Page   int    `json:"page,omitempty"`   // 1-indexed; 0 means "not set"
	Size   int    `json:"size,omitempty"`   // default enforced by handler (e.g. 20)
	Cursor string `json:"cursor,omitempty"` // opaque base64 continuation token
}

// PageMeta is embedded in list responses to communicate pagination state.
type PageMeta struct {
	Page        int    `json:"page"`
	Size        int    `json:"size"`
	TotalCount  int    `json:"totalCount"`
	TotalPages  int    `json:"totalPages"`
	NextCursor  string `json:"nextCursor,omitempty"` // set when more pages exist
}
