package database

import (
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

type firewallRecordHeap []xnode.FirewallRecord

func firewallRecordLess(at time.Time, id string, other time.Time, otherID string) bool {
	return at.Before(other) || at.Equal(other) && id < otherID
}
func (h firewallRecordHeap) Len() int { return len(h) }
func (h firewallRecordHeap) Less(i, j int) bool {
	return firewallRecordLess(h[i].DetectedAt, h[i].ID, h[j].DetectedAt, h[j].ID)
}
func (h firewallRecordHeap) Swap(i, j int)   { h[i], h[j] = h[j], h[i] }
func (h *firewallRecordHeap) Push(value any) { *h = append(*h, value.(xnode.FirewallRecord)) }
func (h *firewallRecordHeap) Pop() any {
	old := *h
	value := old[len(old)-1]
	*h = old[:len(old)-1]
	return value
}
