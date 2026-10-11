package iplookup

import (
	"context"
	"encoding/json"
	"net/url"
	"sort"
	"time"
)

// TransferHistory is a normalized, dated registry history; absent means legacy.
type TransferHistory struct {
	LiveMeta
	Transfers []IPTransfer `json:"transfers"`
}

// IPTransfer records the published movement of a resource between holders.
type IPTransfer struct {
	SourceResource      string `json:"sourceResource"`
	RecipientResource   string `json:"recipientResource"`
	SourceHolderName    string `json:"sourceHolderName"`
	RecipientHolderName string `json:"recipientHolderName"`
	SourceRegistry      string `json:"sourceRegistry"`
	RecipientRegistry   string `json:"recipientRegistry"`
	TransferTime        string `json:"transferTime"`
	TransferType        string `json:"transferType"`
}

func (s *LiveService) transfers(ctx context.Context, ip string) TransferHistory {
	result := TransferHistory{Transfers: []IPTransfer{}}
	data, err := s.liveHTTP(ctx, "ripe", "/data/transfer-history/data.json", url.Values{"resource": {ip}}, nil, "")
	result.LiveMeta = liveMeta("ripe:transfer-history", err)
	if err != nil {
		return result
	}
	d := nested(liveObject(data), "data")
	parameters := nested(d, "parameters")
	result.ObservedAt = textField(parameters, "latest_time")
	var rows []object
	if string(d["transfers"]) == "null" || json.Unmarshal(d["transfers"], &rows) != nil || textField(parameters, "resource") != ip {
		result.LiveMeta = liveMeta("ripe:transfer-history", &CodeError{"IP_DETAILS_INVALID_RESPONSE"})
		return result
	}
	seen := map[IPTransfer]bool{}
	for _, row := range rows {
		date := textField(row, "transferTime")
		valid := false
		for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02"} {
			if parsed, err := time.Parse(layout, date); err == nil {
				date = parsed.UTC().Format(time.RFC3339)
				valid = true
				break
			}
		}
		r := IPTransfer{SourceResource: textField(row, "sourceResource"), RecipientResource: textField(row, "recipientResource"), SourceHolderName: textField(row, "sourceHolderName"), RecipientHolderName: textField(row, "recipientHolderName"), SourceRegistry: textField(row, "sourceRegistry"), RecipientRegistry: textField(row, "recipientRegistry"), TransferTime: date, TransferType: textField(row, "transferType")}
		if !valid || r.RecipientResource == "" {
			result.Status = "partial"
			continue
		}
		if !seen[r] {
			result.Transfers = append(result.Transfers, r)
			seen[r] = true
		}
	}
	sort.Slice(result.Transfers, func(i, j int) bool {
		a, b := result.Transfers[i], result.Transfers[j]
		return a.TransferTime > b.TransferTime || a.TransferTime == b.TransferTime && a.RecipientResource < b.RecipientResource
	})
	if len(result.Transfers) > 1000 {
		result.Transfers = result.Transfers[:1000]
		result.Status = "partial"
	}
	if len(result.Transfers) == 0 && result.Status == "success" {
		result.Status = "empty"
	}
	return result
}
