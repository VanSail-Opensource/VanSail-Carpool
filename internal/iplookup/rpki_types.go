package iplookup

// RPKIResult freezes origin validation independently of reputation provider coverage.
type RPKIResult struct {
	LiveMeta
	Prefix string      `json:"prefix"`
	Routes []RPKIRoute `json:"routes"`
}

// RPKIRoute validates one announced origin, including source failures separately.
type RPKIRoute struct {
	LiveMeta
	ASN      uint32    `json:"asn"`
	Validity string    `json:"validity"`
	ROAs     []RPKIROA `json:"roas"`
}

// RPKIROA is the published authorization relevant to an origin/prefix pair.
type RPKIROA struct {
	ASN       uint32 `json:"asn"`
	Prefix    string `json:"prefix"`
	MaxLength int    `json:"maxLength"`
}

func applyRPKIRefusals(r *Report) {
	kept := make([]Refusal, 0, len(r.Refusals))
	for _, item := range r.Refusals {
		if item.Kind != "rpki_invalid_asn" && item.Kind != "rpki_invalid_length" {
			kept = append(kept, item)
		}
	}
	r.Refusals = kept
	if r.RPKI == nil {
		return
	}
	for _, route := range r.RPKI.Routes {
		if route.Status == "success" && (route.Validity == "invalid_asn" || route.Validity == "invalid_length") {
			r.Refusals = append(r.Refusals, Refusal{Kind: "rpki_" + route.Validity, Source: "ripe_rpki", Value: "AS" + asnText(route.ASN) + " " + r.RPKI.Prefix})
		}
	}
}
