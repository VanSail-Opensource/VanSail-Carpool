package providerops

const (
	// KindSubscriptionRevoke rotates one member subscription credential.
	KindSubscriptionRevoke = "subscription_revoke"
	// KindEmbySetup provisions one priced Emby account.
	KindEmbySetup = "emby_setup"
	// KindEmbyPreferences replaces one linked Emby policy projection.
	KindEmbyPreferences = "emby_preferences"
	// KindEmbyPassword replaces one linked Emby password.
	KindEmbyPassword = "emby_password"
	// KindEmbyProvisionRetry resumes one explicitly reviewed provisioning attempt.
	KindEmbyProvisionRetry = "emby_provision_retry"
	// KindOutboxRetry makes one failed local job eligible again.
	KindOutboxRetry = "outbox_retry"
	// KindPaymentRefund reverses one provider-funded balance credit.
	KindPaymentRefund = "payment_refund"
)
