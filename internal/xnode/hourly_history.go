package xnode

// HourlyTrafficHistoryRepository is implemented only after accepted hourly
// samples and dropped-coverage evidence share the native journal's atomic
// persistence and replay deduplication. It must be a memory-only readiness check.
// Credit settlement support alone cannot advertise this contract.
type HourlyTrafficHistoryRepository interface {
	HourlyTrafficHistoryReady() bool
}

func (m *Master) hourlyTrafficHistoryReady() bool {
	repo, ok := m.service.repo.(HourlyTrafficHistoryRepository)
	return ok && repo.HourlyTrafficHistoryReady()
}
