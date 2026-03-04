package service

import "strings"

func normalizeStatusValue(raw string) string {
	return strings.ToUpper(strings.TrimSpace(raw))
}

func resolveEntityStatus(effectiveStatus, configuredStatus, legacyStatus string) string {
	if s := normalizeStatusValue(effectiveStatus); s != "" {
		return s
	}
	if s := normalizeStatusValue(configuredStatus); s != "" {
		return s
	}
	if s := normalizeStatusValue(legacyStatus); s != "" {
		return s
	}
	return ""
}
