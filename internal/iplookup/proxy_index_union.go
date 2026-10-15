package iplookup

import (
	"context"
	"github.com/RoaringBitmap/roaring/v2"
)

// mergePrefixIndexes retains previous evidence first, without mutating either
// input. A zero limit permits the aggregate across independently bounded lists.
func mergePrefixIndexes(previous, incoming *prefixIndex, limit int) *prefixIndex {
	return mergePrefixIndexesContext(context.Background(), previous, incoming, limit)
}

func mergePrefixIndexesContext(ctx context.Context, previous, incoming *prefixIndex, limit int) *prefixIndex {
	if (previous != nil && previous.disk != nil) || (incoming != nil && incoming.disk != nil) {
		return mergeDiskIndexes(ctx, previous, incoming, limit)
	}
	if incoming == nil || incoming.count() == 0 {
		return previous
	}
	if previous == nil || previous.count() == 0 {
		return incoming
	}
	out := &prefixIndex{size: previous.size}
	out.v6 = previous.v6
	for bits, bucket := range previous.v4 {
		if bucket != nil {
			out.v4[bits] = bucket.Clone()
		}
	}
	for bits, bucket := range incoming.v4 {
		if bucket == nil {
			continue
		}
		added := bucket.Clone()
		if old := out.v4[bits]; old != nil {
			added.AndNot(old)
		}
		remaining := int(added.GetCardinality())
		if limit > 0 {
			remaining = min(remaining, max(0, limit-out.size))
		}
		if remaining == 0 {
			continue
		}
		if uint64(remaining) < added.GetCardinality() {
			// Select is valid because remaining is strictly below cardinality.
			cutoff, err := added.Select(uint32(remaining))
			if err != nil {
				continue
			}
			added.RemoveRange(uint64(cutoff), 1<<32)
		}
		if out.v4[bits] == nil {
			out.v4[bits] = roaring.New()
		}
		out.v4[bits].Or(added)
		out.v4[bits].RunOptimize()
		out.size += remaining
	}
	for bits, values := range incoming.v6 {
		if len(values) == 0 {
			continue
		}
		remaining := len(values)
		if limit > 0 {
			remaining = max(0, limit-out.size)
		}
		merged, added := mergeIPv6(out.v6[bits], values, remaining)
		out.v6[bits] = merged
		out.size += added
	}
	return out
}

func mergeIPv6(previous, incoming [][16]byte, remaining int) ([][16]byte, int) {
	out := make([][16]byte, 0, len(previous)+min(len(incoming), remaining))
	i, j, added := 0, 0, 0
	for i < len(previous) || j < len(incoming) {
		if j == len(incoming) || (i < len(previous) && compareIPv6(previous[i], incoming[j]) < 0) {
			out = append(out, previous[i])
			i++
		} else if i < len(previous) && previous[i] == incoming[j] {
			out = append(out, previous[i])
			i++
			j++
		} else {
			if added < remaining {
				out = append(out, incoming[j])
				added++
			}
			j++
		}
	}
	return out, added
}
