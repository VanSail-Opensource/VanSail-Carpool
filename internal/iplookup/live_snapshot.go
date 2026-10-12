package iplookup

import "encoding/json"

func emitCachedDetails(r LiveDetails, emit func(LiveEvent) error) (LiveDetails, error) {
	r = CompactLiveDetails(r)
	if emit != nil {
		data, err := json.Marshal(r)
		if err != nil {
			return LiveDetails{}, err
		}
		if err := emit(liveComplete(r.IP, data)); err != nil {
			return LiveDetails{}, err
		}
	}
	return r, nil
}

// LivePartial reports missing/error coverage without changing the quality verdict.
func LivePartial(r LiveDetails) bool {
	metas := []LiveMeta{r.Topology.LiveMeta, r.Topology.ClassificationMeta, r.Scores.LiveMeta, r.Block.LiveMeta, r.PTR.LiveMeta, r.Shodan.LiveMeta}
	if relationshipMetaFailed(r.Topology.RelationshipMeta) {
		return true
	}
	if r.Transfers != nil {
		metas = append(metas, r.Transfers.LiveMeta)
	}
	for _, asn := range r.ASNs {
		for _, meta := range asn.Sources {
			metas = append(metas, meta)
		}
		metas = append(metas, asn.Traffic.BotHuman.LiveMeta, asn.Traffic.Devices.LiveMeta, asn.Traffic.IPVersion.LiveMeta)
	}
	for _, meta := range metas {
		if meta.Status == "error" || meta.Status == "partial" {
			return true
		}
	}
	return false
}

// relationshipMetaFailed ignores code-less relationship degradation: edges
// that stay merely "observed" reflect the inherent coverage of the dated
// CAIDA dataset (routinely supplemented by cloudflare), not a fetch failure.
// Such lineage must not mark the whole check partial; real relationship
// errors always record an error code.
func relationshipMetaFailed(m LiveMeta) bool {
	return m.Status == "error" || (m.Status == "partial" && m.ErrorCode != "")
}
