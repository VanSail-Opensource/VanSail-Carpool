package model

// PAYGAvailable never treats merchandising visibility as access revocation.
func (p SquadProduct) PAYGAvailable() bool {
	return p.BillingMode == "payg" && p.PAYGState != "unavailable"
}
