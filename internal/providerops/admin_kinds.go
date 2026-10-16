package providerops

const (

	// KindAdminEntitlementEdit synchronizes one audited full-field edit.
	KindAdminEntitlementEdit = "admin_entitlement_edit"
	// KindAdminEntitlementRefund synchronizes one credited cancellation.
	KindAdminEntitlementRefund = "admin_entitlement_refund"
	// KindAdminComboReplacement synchronizes one no-charge configuration change.
	KindAdminComboReplacement = "admin_combo_replacement"
	// KindAdminBulkExtension synchronizes deduplicated active-user extensions.
	KindAdminBulkExtension = "admin_bulk_extension"
	// KindNodeCompensation synchronizes one reviewed outage compensation.
	KindNodeCompensation = "node_compensation"
	// KindAdminTemporaryBan changes a profile's remote connection state.
	KindAdminTemporaryBan = "admin_temporary_ban"
	// KindAdminTemporaryUnban restores a profile after a manual temporary ban.
	KindAdminTemporaryUnban = "admin_temporary_unban"

	// KindAdminMaintenance runs the backup-gated retention maintenance flow.
	KindAdminMaintenance = "admin_maintenance"
)
