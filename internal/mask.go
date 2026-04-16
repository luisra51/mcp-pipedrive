package internal

import "strings"

var sensitiveKeys = []string{"token", "secret", "password", "apikey", "api_key", "api_token", "access_token", "refresh_token"}

func MaskSensitive(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, v2 := range t {
			if isSensitiveKey(k) {
				out[k] = "***"
				continue
			}
			out[k] = MaskSensitive(v2)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, v2 := range t {
			out[i] = MaskSensitive(v2)
		}
		return out
	default:
		return v
	}
}

func isSensitiveKey(key string) bool {
	k := strings.ToLower(key)
	for _, s := range sensitiveKeys {
		if strings.Contains(k, s) {
			return true
		}
	}
	return false
}
