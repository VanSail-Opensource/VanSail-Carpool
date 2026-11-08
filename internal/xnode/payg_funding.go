package xnode

import "time"

// PAYGFunding contains fresh authorization inputs, without historical counters.
// It is an ephemeral read projection, never a cached admission decision.
type PAYGFunding struct {
	ProxyUserID                               int64
	GroupID, NodeID                           string
	UnitPriceMinor, BalanceMinor, CreditBytes int64
	CreditStartedAt                           time.Time
	Disabled                                  bool
}

// Funding projects only the fields needed to renew volatile authorization.
func (a PAYGAccount) Funding() PAYGFunding {
	return PAYGFunding{ProxyUserID: a.ProxyUserID, GroupID: a.GroupID, NodeID: a.NodeID,
		UnitPriceMinor: a.UnitPriceMinor, BalanceMinor: a.BalanceMinor,
		CreditBytes: a.CreditBytes, CreditStartedAt: a.CreditStartedAt, Disabled: a.Disabled}
}

func (a PAYGFunding) account() PAYGAccount {
	return PAYGAccount{UnitPriceMinor: a.UnitPriceMinor, BalanceMinor: a.BalanceMinor,
		CreditBytes: a.CreditBytes, CreditStartedAt: a.CreditStartedAt}
}

// HasCredit retains the accounting model's exact prepaid expiry boundary.
func (a PAYGFunding) HasCredit(at time.Time) bool { return a.account().HasCredit(at) }

// Funded retains the accounting model's current wallet and prepaid rules.
func (a PAYGFunding) Funded(at time.Time) bool { return a.account().Funded(at) }
