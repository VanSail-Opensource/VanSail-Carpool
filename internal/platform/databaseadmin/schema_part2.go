package databaseadmin

import (
	"strings"
)

func isBooleanColumn(name, declared string) bool {
	if strings.Contains(strings.ToUpper(declared), "BOOL") {
		return true
	}
	name = strings.ToLower(name)
	if strings.HasPrefix(name, "is_") || strings.HasPrefix(name, "has_") || strings.HasPrefix(name, "enable_") || strings.HasPrefix(name, "allow_") {
		return true
	}
	for _, exact := range []string{"active", "enabled", "visible", "encrypted", "configured", "retryable", "replayed", "won", "create_attempted", "group_joined", "channel_joined", "upstream_present"} {
		if name == exact {
			return true
		}
	}
	return false
}

func isSensitiveColumn(table, column string) bool {
	column = strings.ToLower(column)
	if table == "settings" && column == "value" {
		return true
	}
	for _, fragment := range []string{"password", "secret", "token", "hash", "cipher", "subscription_url", "invite_link", "capability", "provider_payload", "raw_csv"} {
		if strings.Contains(column, fragment) {
			return true
		}
	}
	return false
}
