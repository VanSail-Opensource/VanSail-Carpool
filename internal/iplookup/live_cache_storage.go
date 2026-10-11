package iplookup

import "time"

// storeLiveCache is called with mu held; successful bounded partial data is reusable.
func (s *LiveService) storeLiveCache(key string, data []byte) {
	if len(data) > 16*1024*1024 {
		return
	}
	now := s.now()
	for id, entry := range s.cache {
		if !now.Before(entry.expires) {
			delete(s.cache, id)
		}
	}
	delete(s.cache, key)
	for {
		size := len(data)
		oldest := ""
		var used time.Time
		for id, entry := range s.cache {
			size += len(entry.data)
			if oldest == "" || entry.used.Before(used) {
				oldest, used = id, entry.used
			}
		}
		if len(s.cache) < 64 && size <= 16*1024*1024 {
			break
		}
		delete(s.cache, oldest)
	}
	s.cache[key] = liveCacheEntry{data: data, expires: now.Add(time.Minute), used: now}
}

func liveCacheable(r LiveDetails) bool {
	metas := []LiveMeta{r.Topology.LiveMeta, r.Topology.RelationshipMeta, r.Topology.ClassificationMeta, r.Scores.LiveMeta, r.Block.LiveMeta, r.PTR.LiveMeta, r.Shodan.LiveMeta}
	for _, a := range r.ASNs {
		for _, meta := range a.Sources {
			metas = append(metas, meta)
		}
		metas = append(metas, a.Traffic.BotHuman.LiveMeta, a.Traffic.Devices.LiveMeta, a.Traffic.IPVersion.LiveMeta)
	}
	for _, meta := range metas {
		if meta.Status == "error" || (meta.ErrorCode != "" && meta.Status != "restricted") {
			return false
		}
	}
	return true
}
