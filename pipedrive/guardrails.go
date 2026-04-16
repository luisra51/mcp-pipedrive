package pipedrive

// IsToolAllowed returns true when no allowlist is configured or the tool name
// is present in the allowlist.
func IsToolAllowed(cfg Config, toolName string) bool {
	if len(cfg.AllowedTools) == 0 {
		return true
	}
	_, ok := cfg.AllowedTools[toolName]
	return ok
}
