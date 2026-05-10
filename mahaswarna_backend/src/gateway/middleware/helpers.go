package middleware

import (
	"encoding/json"
	"io"
)

// encodeJSON writes v as JSON into w. Errors are silently discarded because
// at the point we call this the response header has already been written.
func encodeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}
