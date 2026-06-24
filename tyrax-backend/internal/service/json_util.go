package service

import "encoding/json"

// parseJSON is a thin wrapper so callers don't import encoding/json directly.
func parseJSON(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
