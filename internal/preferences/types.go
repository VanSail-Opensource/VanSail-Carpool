// Package preferences defines account-wide presentation and delivery choices.
package preferences

import "context"

// Notifications groups automatic private messages by their originating event.
type Notifications struct {
	Combos   bool `json:"combos"`
	Traffic  bool `json:"traffic"`
	Money    bool `json:"money"`
	Activity bool `json:"activity"`
	Account  bool `json:"account"`
}

// Preferences contains only choices that cannot be derived from other records.
type Preferences struct {
	PAYGEnabled          bool          `json:"paygEnabled"`
	Notifications        Notifications `json:"notifications"`
	ShowReferralUsername bool          `json:"showReferralUsername"`
	ShowAroundTX         bool          `json:"showAroundTx"`
	ShowActivity         bool          `json:"showActivity"`
	IncludeNodePrices    bool          `json:"includeNodePrices"`
}

// Snapshot adds server-owned current eligibility to the stored choices.
type Snapshot struct {
	Preferences
	ActiveCombo bool            `json:"activeCombo"`
	PAYGSquads  map[string]bool `json:"paygSquads"`
}

// NotificationPatch distinguishes omitted values from an explicit false.
type NotificationPatch struct {
	Combos   *bool `json:"combos"`
	Traffic  *bool `json:"traffic"`
	Money    *bool `json:"money"`
	Activity *bool `json:"activity"`
	Account  *bool `json:"account"`
}

// Patch merges only supplied fields into the latest stored preferences.
type Patch struct {
	PAYGEnabled          *bool              `json:"paygEnabled"`
	PAYGSquads           map[string]bool    `json:"paygSquads"`
	Notifications        *NotificationPatch `json:"notifications"`
	ShowReferralUsername *bool              `json:"showReferralUsername"`
	ShowAroundTX         *bool              `json:"showAroundTx"`
	ShowActivity         *bool              `json:"showActivity"`
	IncludeNodePrices    *bool              `json:"includeNodePrices"`
}

// Reader retrieves the latest choices at the delivery boundary.
type Reader interface {
	UserPreferences(context.Context, string) (Preferences, error)
}

// Defaults preserves delivery and pricing while keeping optional entrances hidden.
func Defaults() Preferences {
	return Preferences{PAYGEnabled: true, Notifications: Notifications{true, true, true, true, true}, ShowReferralUsername: true, IncludeNodePrices: true}
}

// Apply merges a partial update without replacing unrelated fields.
func (p *Preferences) Apply(patch Patch) {
	set := func(target *bool, value *bool) {
		if value != nil {
			*target = *value
		}
	}
	set(&p.PAYGEnabled, patch.PAYGEnabled)
	set(&p.ShowReferralUsername, patch.ShowReferralUsername)
	set(&p.ShowAroundTX, patch.ShowAroundTX)
	set(&p.ShowActivity, patch.ShowActivity)
	set(&p.IncludeNodePrices, patch.IncludeNodePrices)
	if n := patch.Notifications; n != nil {
		set(&p.Notifications.Combos, n.Combos)
		set(&p.Notifications.Traffic, n.Traffic)
		set(&p.Notifications.Money, n.Money)
		set(&p.Notifications.Activity, n.Activity)
		set(&p.Notifications.Account, n.Account)
	}
}

// PAYG contains durable opt-outs only; an absent squad choice is enabled.
type PAYG struct {
	Enabled bool
	Squads  map[string]bool
}

// Allows applies the master switch without changing saved squad choices.
func (p PAYG) Allows(group string) bool {
	enabled, exists := p.Squads[group]
	return p.Enabled && (!exists || enabled)
}
