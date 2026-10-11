package iplookup

import (
	"encoding/json"
	"strconv"
	"strings"
)

type object = map[string]json.RawMessage

func nested(o object, key string) object { var v object; _ = json.Unmarshal(o[key], &v); return v }
func textField(o object, key string) string {
	var v string
	if json.Unmarshal(o[key], &v) != nil {
		var n json.Number
		if json.Unmarshal(o[key], &n) == nil {
			v = n.String()
		}
	}
	v = strings.TrimSpace(v)
	if v == "N/A" || v == "N\\A" || v == "-" || strings.Contains(strings.ToLower(v), "required") || strings.HasPrefix(strings.ToLower(v), "premium field") {
		return ""
	}
	if len(v) > 256 {
		return ""
	}
	return v
}
func flag(o object, key string) *bool {
	var v *bool
	if json.Unmarshal(o[key], &v) != nil {
		return nil
	}
	return v
}
func boolean(v bool) *bool { return &v }
func numeric(o object, key string) *float64 {
	if raw, ok := o[key]; !ok || string(raw) == "null" {
		return nil
	}
	var v float64
	if json.Unmarshal(o[key], &v) == nil {
		return &v
	}
	return nil
}
func addScore(p *ProviderResult, o object, key, label string) {
	if value := numeric(o, key); value != nil && *value >= 0 {
		p.Scores[label] = *value
	}
}
func networkType(v string) string {
	switch strings.ToLower(v) {
	case "residential", "fixed line isp", "isp", "cable/dsl":
		return "residential"
	case "mobile", "mobile isp", "cellular", "mob", "isp/mob":
		return "mobile"
	case "data center", "data center/web hosting/transit", "hosting", "dch", "content delivery network", "content_delivery_network", "cdn":
		return "datacenter"
	case "corporate", "commercial", "business", "organization", "education", "government", "university/college/school", "com", "edu", "gov", "org", "mil", "lib", "college", "school", "military", "library":
		return "business"
	case "satellite", "sat":
		return "satellite"
	default:
		return ""
	}
}
func allKnown(values ...*bool) bool {
	for _, v := range values {
		if v == nil {
			return false
		}
	}
	return true
}

func parseProvider(id, ip string, o object) (ProviderResult, error) {
	p := ProviderResult{ID: id, Status: "success", Scores: map[string]float64{}}
	var err error
	switch id {
	case "abuseipdb":
		err = parseAbuse(&p, ip, nested(o, "data"))
	case "ipapi":
		err = parseIPAPI(&p, ip, o)
	case "scamalytics":
		err = parseScamalytics(&p, ip, o)
	case "maxmind":
		err = parseMaxmind(&p, ip, o)
	case "ipqs":
		err = parseIPQS(&p, o)
	case "ip2location":
		err = parseIP2(&p, ip, o)
	default:
		err = &CodeError{"IP_LOOKUP_PROVIDER_UNAVAILABLE"}
	}
	if err != nil {
		return p, err
	}
	enrichProvider(&p, o)
	if !p.Complete {
		p.Status = "partial"
		p.ErrorCode = "IP_LOOKUP_MISSING_FIELDS"
	}
	return p, nil
}

func validReturnedIP(o object, key, ip string) bool {
	canonical, err := CanonicalIP(textField(o, key))
	return err == nil && canonical == ip
}

func parseAbuse(p *ProviderResult, ip string, o object) error {
	if !validReturnedIP(o, "ipAddress", ip) {
		return &CodeError{"IP_LOOKUP_INVALID_RESPONSE"}
	}
	p.Facts = Facts{Country: textField(o, "countryCode"), NetworkType: networkType(textField(o, "usageType"))}
	addScore(p, o, "abuseConfidenceScore", "abuse_confidence")
	if n := numeric(o, "totalReports"); n != nil && *n >= 0 && *n <= 1000000000 {
		v := int(*n)
		p.Reports = &v
	}
	if score := numeric(o, "abuseConfidenceScore"); score != nil {
		p.Signals.Abuse = boolean(*score > 0 || (p.Reports != nil && *p.Reports > 0))
	}
	if textField(o, "usageType") != "" {
		p.Signals.Datacenter = boolean(p.Facts.NetworkType == "datacenter")
	}
	p.Signals.Tor = flag(o, "isTor")
	p.Complete = allKnown(p.Signals.Abuse, p.Signals.Datacenter, p.Signals.Tor) && p.Reports != nil
	return nil
}

func parseIPAPI(p *ProviderResult, ip string, o object) error {
	if !validReturnedIP(o, "ip", ip) {
		return &CodeError{"IP_LOOKUP_INVALID_RESPONSE"}
	}
	asn, company, location := nested(o, "asn"), nested(o, "company"), nested(o, "location")
	p.ContextScores = &NetworkScores{Company: score(company), ASN: score(asn)}
	p.Facts = Facts{Country: textField(location, "country_code"), City: textField(location, "city"), ASN: textField(asn, "asn")}
	p.Signals = Signals{Abuse: flag(o, "is_abuser"), Datacenter: flag(o, "is_datacenter"), VPN: ipapiVPN(o), Proxy: flag(o, "is_proxy"), Tor: flag(o, "is_tor")}
	p.Facts.NetworkType = ipapiNetwork(o)
	for label, source := range map[string]object{"company_abuse_ratio": company, "asn_abuse_ratio": asn} {
		parts := strings.Fields(textField(source, "abuser_score"))
		if len(parts) > 0 {
			if n, err := strconv.ParseFloat(parts[0], 64); err == nil && n >= 0 && n <= 1 {
				p.Scores[label] = n
			}
		}
	}
	p.Complete = allKnown(p.Signals.Abuse, p.Signals.Datacenter, p.Signals.VPN, p.Signals.Proxy, p.Signals.Tor)
	return nil
}

func ipapiNetwork(o object) string {
	unknown := false
	for _, item := range []struct{ key, kind string }{{"is_datacenter", "datacenter"}, {"is_satellite", "satellite"}, {"is_mobile", "mobile"}} {
		value := flag(o, item.key)
		if value != nil && *value {
			return item.kind
		}
		unknown = unknown || value == nil
	}
	if !unknown {
		return "residential"
	}
	return ""
}

func ipapiVPN(o object) *bool {
	if v := flag(o, "is_vpn"); v != nil {
		return v
	}
	raw, exists := o["is_vpn"]
	if !exists || string(raw) == "null" {
		return nil
	}
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return nil
	}
	switch value := v.(type) {
	case string:
		return boolean(value != "")
	case float64:
		return boolean(value != 0)
	default:
		return boolean(true)
	}
}
