package iplookup

import (
	"context"
	"encoding/json"
	"net/mail"
	"sort"
	"strings"
	"sync"
)

// originDetails fetches at most eight origins and batches their public websites.
func (s *LiveService) originDetails(ctx context.Context, origins []uint32, c Config) []ASNDetails {
	origins = origins[:min(8, len(origins))]
	values := make([]ASNDetails, len(origins))
	var work sync.WaitGroup
	for i, id := range origins {
		work.Add(1)
		go func() { defer work.Done(); a := s.asn(ctx, id, c); s.asnContacts(ctx, &a); values[i] = a }()
	}
	websites, meta := s.websites(ctx, origins)
	work.Wait()
	for i := range values {
		values[i].Website = websites[values[i].ASN]
		values[i].Sources["website"] = meta
		if meta.Status == "success" && values[i].Website == "" {
			empty := meta
			empty.Status = "empty"
			values[i].Sources["website"] = empty
		}
	}
	return values
}

func (s *LiveService) asnContacts(ctx context.Context, a *ASNDetails) {
	data, err := s.liveHTTP(ctx, "ripe", "/data/abuse-contact-finder/data.json", ripeQuery(a.ASN), nil, "")
	meta := liveMeta("ripe:abuse-contact-finder", err)
	a.AbuseContacts = []string{}
	if err == nil {
		d := nested(liveObject(data), "data")
		meta.ObservedAt = textField(d, "latest_time")
		var contacts []string
		if string(d["abuse_contacts"]) == "null" || json.Unmarshal(d["abuse_contacts"], &contacts) != nil || textField(nested(d, "parameters"), "resource") != asnText(a.ASN) {
			meta = liveMeta(meta.Source, &CodeError{"IP_DETAILS_INVALID_RESPONSE"})
		} else {
			seen := map[string]bool{}
			for _, contact := range contacts {
				contact = strings.TrimSpace(contact)
				address, err := mail.ParseAddress(contact)
				if err != nil || address.Address != contact || len(contact) > 254 {
					meta.Status = "partial"
					continue
				}
				if !seen[contact] {
					a.AbuseContacts = append(a.AbuseContacts, contact)
					seen[contact] = true
				}
			}
			sort.Strings(a.AbuseContacts)
			if len(a.AbuseContacts) == 0 && meta.Status == "success" {
				meta.Status = "empty"
			}
		}
	}
	a.Sources["abuseContacts"] = meta
}
