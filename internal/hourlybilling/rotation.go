package hourlybilling

// These cursors contain no financial/admission authority. Evicting idle owners
// only restarts a scheduling rotation; paid intervals and pending jobs persist.
type deliveryCursor struct {
	readiness, application int
	used                   uint64
}

func (s *Service) deliveryOffset(owner string, application bool, count int) int {
	if count == 0 {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cursor, found := s.deliveryCursors[owner]
	if !found && len(s.deliveryCursors) >= 8192 {
		var oldest string
		var sequence uint64 = ^uint64(0)
		for id, existing := range s.deliveryCursors {
			if existing.used <= sequence {
				oldest, sequence = id, existing.used
			}
		}
		delete(s.deliveryCursors, oldest)
	}
	offset := cursor.readiness % count
	if application {
		offset = cursor.application % count
		cursor.application = (offset + 4) % count
	} else {
		cursor.readiness = (offset + 8) % count
	}
	s.deliverySequence++
	cursor.used = s.deliverySequence
	s.deliveryCursors[owner] = cursor
	return offset
}
