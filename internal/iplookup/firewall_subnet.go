package iplookup

import "slices"

func withoutDetectedSubnet(source *SubnetResult) *SubnetResult {
	if source == nil {
		return nil
	}
	result := *source
	result.Sources = slices.DeleteFunc(slices.Clone(source.Sources), func(item InventorySource) bool { return item.ID == detectedSourceID })
	result.Addresses = slices.Clone(source.Addresses)
	for i := range result.Addresses {
		row := &result.Addresses[i]
		if !slices.Contains(row.Sources, detectedSourceID) {
			continue
		}
		row.Sources = withoutDetectedIDs(row.Sources)
		abuse, proxy := false, false
		for _, id := range row.Sources {
			for _, item := range result.Sources {
				if item.ID == id {
					if item.Category == AbuseCategory {
						abuse = true
					} else {
						proxy = true
					}
				}
			}
		}
		if !abuse && row.Abuse == "flagged" {
			row.Abuse = "skipped"
		}
		if !proxy && row.Proxy == "flagged" {
			row.Proxy = "success"
		}
		if !abuse && !proxy && row.IPAPI != "flagged" && !confirmedPorts(row.Ports) {
			row.Status = "unknown"
		}
	}
	return &result
}

func confirmedPorts(ports []PortProbe) bool {
	for _, port := range ports {
		for _, finding := range port.Findings {
			if finding.Confidence == "confirmed" {
				return true
			}
		}
	}
	return false
}
