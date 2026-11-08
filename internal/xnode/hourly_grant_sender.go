package xnode

import (
	"sort"
	"sync"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/hourly"
)

const maxHourlyPendingGrants = 8192

type hourlyGrantDelivery struct {
	grant            hourly.Grant
	pending, sending bool
}
type hourlyGrantSender struct {
	mu        sync.Mutex
	grants    map[hourly.Owner]*hourlyGrantDelivery
	running   bool
	cursor    string
	ready     []hourly.Owner
	nextPrune time.Time
}

func newHourlyGrantSender() *hourlyGrantSender {
	return &hourlyGrantSender{grants: map[hourly.Owner]*hourlyGrantDelivery{}}
}

func (s *hourlyGrantSender) prune(now time.Time) {
	if now.Before(s.nextPrune) {
		return
	}
	s.nextPrune = now.Add(time.Second)
	for owner, delivery := range s.grants {
		if !delivery.grant.ConservativeUntil.After(now) {
			delete(s.grants, owner)
		}
	}
}

// offer coalesces owners, rejects older sequences and retains bounded high-water
// evidence through expiry. It never changes the allocator's reservations.
func (s *hourlyGrantSender) offer(now time.Time, grant hourly.Grant) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune(now)
	if !grant.ConservativeUntil.After(now) {
		return false
	}
	if previous := s.grants[grant.Owner]; previous != nil {
		if previous.grant.Sequence > grant.Sequence || previous.grant.Sequence == grant.Sequence && (previous.grant != grant || previous.sending) {
			return false
		}
		if previous.grant.Sequence == grant.Sequence {
			previous.pending = true
		} else {
			s.grants[grant.Owner] = &hourlyGrantDelivery{grant: grant, pending: true}
		}
	} else {
		if len(s.grants) >= maxHourlyPendingGrants {
			var victim hourly.Owner
			var earliest time.Time
			for owner, delivery := range s.grants {
				if !delivery.pending && !delivery.sending && (earliest.IsZero() || delivery.grant.ConservativeUntil.Before(earliest)) {
					victim, earliest = owner, delivery.grant.ConservativeUntil
				}
			}
			if earliest.IsZero() {
				return false
			}
			delete(s.grants, victim) // Intent metadata only; allocator holds remain intact.
		}
		s.grants[grant.Owner] = &hourlyGrantDelivery{grant: grant, pending: true}
	}
	if s.running {
		return false
	}
	s.running = true
	return true
}

func (s *hourlyGrantSender) next(now time.Time) (hourly.Grant, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune(now)
	for {
		if len(s.ready) == 0 {
			for owner, delivery := range s.grants {
				if delivery.pending && !delivery.sending {
					s.ready = append(s.ready, owner)
				}
			}
			sort.Slice(s.ready, func(i, j int) bool {
				if s.ready[i].ID == s.ready[j].ID {
					return s.ready[i].Generation < s.ready[j].Generation
				}
				return s.ready[i].ID < s.ready[j].ID
			})
			if len(s.ready) == 0 {
				s.running = false
				return hourly.Grant{}, false
			}
			start := sort.Search(len(s.ready), func(i int) bool { return s.ready[i].ID > s.cursor })
			if start < len(s.ready) {
				s.ready = append(append(make([]hourly.Owner, 0, len(s.ready)), s.ready[start:]...), s.ready[:start]...)
			}
		}
		owner := s.ready[0]
		s.ready[0] = hourly.Owner{}
		s.ready = s.ready[1:]
		delivery := s.grants[owner]
		if delivery == nil || !delivery.pending || delivery.sending {
			continue
		}
		if !delivery.grant.ConservativeUntil.After(now) {
			delete(s.grants, owner)
			continue
		}
		s.cursor = owner.ID
		delivery.pending, delivery.sending = false, true
		return delivery.grant, true
	}
}

func (s *hourlyGrantSender) complete(grant hourly.Grant) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if delivery := s.grants[grant.Owner]; delivery != nil && delivery.grant.Sequence == grant.Sequence {
		delivery.sending = false
	}
}

func (s *hourlyGrantSender) stop() { s.mu.Lock(); s.running = false; s.mu.Unlock() }
