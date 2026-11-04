package connections

// Stable authenticated context for administrator unblock and retired block cleanup.
func ipBlockSecretContext(userID, nodeUUID, digest string) string {
	return "connection-ip-block:" + userID + ":" + nodeUUID + ":" + digest
}
