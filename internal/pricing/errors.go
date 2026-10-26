package pricing

// Code is a stable machine-readable degraded-state classification.
type Code string

const (
	ModelUnavailable        Code = "PRICING_MODEL_UNAVAILABLE"
	ModelUnvalidated        Code = "PRICING_MODEL_UNVALIDATED"
	ForecastStale           Code = "PRICING_FORECAST_STALE"
	TelemetryIncomplete     Code = "PRICING_TELEMETRY_INCOMPLETE"
	NodeOffline             Code = "PRICING_NODE_OFFLINE"
	NodeOverloaded          Code = "PRICING_NODE_OVERLOADED"
	CapacityUnsafe          Code = "PRICING_MACHINE_CAPACITY_UNSAFE"
	WorkerResourceLimit     Code = "PRICING_WORKER_RESOURCE_LIMIT"
	VersionConflict         Code = "PRICING_PRICE_VERSION_CONFLICT"
	RenewalQuoteUnavailable Code = "PRICING_RENEWAL_QUOTE_UNAVAILABLE"
	RenewalHealthDeferred   Code = "PRICING_RENEWAL_HEALTH_DEFERRED"
	OverrideActive          Code = "PRICING_ADMIN_OVERRIDE_ACTIVE"
	ExportResourceLimit     Code = "PRICING_EXPORT_RESOURCE_LIMIT"
	StoragePressure         Code = "PRICING_STORAGE_PRESSURE"
)

func (c Code) Error() string { return string(c) }

// Impact distinguishes optional analytics failures from authoritative billing.
type Impact struct {
	Prediction   bool `json:"prediction"`
	Publication  bool `json:"publication"`
	Admission    bool `json:"admission"`
	Billing      bool `json:"billing"`
	Entitlements bool `json:"entitlements"`
	Admin        bool `json:"admin"`
}

// Effects never treats an optimizer failure as revocation of a purchased entitlement.
func Effects(code Code) Impact {
	i := Impact{Prediction: true, Publication: true, Admin: true}
	switch code {
	case NodeOffline, NodeOverloaded, CapacityUnsafe:
		i.Admission = true
	case RenewalQuoteUnavailable, RenewalHealthDeferred:
		i.Billing = true
	case ExportResourceLimit:
		i.Prediction = false
		i.Publication = false
	}
	return i
}
