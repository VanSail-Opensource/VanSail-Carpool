package iplookup

import (
	"container/heap"
	"context"
	"encoding/binary"

	"github.com/RoaringBitmap/roaring/v2"
)

func countPrefixIndexesContext(ctx context.Context, indexes []*prefixIndex) (int, error) {
	for _, index := range indexes {
		if index != nil && index.disk != nil {
			return countDiskIndexes(ctx, indexes)
		}
	}
	return countPrefixIndexes(indexes), nil
}

// countPrefixIndexes avoids allocating a full union of every published source.
// It retains one iterator per source in the current prefix-length bucket.
func countPrefixIndexes(indexes []*prefixIndex) int {
	if len(indexes) == 1 {
		return indexes[0].count()
	}
	count := 0
	for bits := range 33 {
		buckets := []*roaring.Bitmap{}
		for _, index := range indexes {
			if index != nil && index.v4[bits] != nil {
				buckets = append(buckets, index.v4[bits])
			}
		}
		switch len(buckets) {
		case 0:
		case 1:
			count += int(buckets[0].GetCardinality())
		case 2:
			count += int(buckets[0].OrCardinality(buckets[1]))
		default:
			streams := prefixStreams{}
			for _, bucket := range buckets {
				stream := &prefixStream{v4: bucket.Iterator()}
				if stream.advance() {
					streams = append(streams, stream)
				}
			}
			count += streams.count()
		}
	}
	for bits := range 129 {
		streams := prefixStreams{}
		for _, index := range indexes {
			if index != nil && len(index.v6[bits]) > 0 {
				stream := &prefixStream{v6: index.v6[bits]}
				stream.advance()
				streams = append(streams, stream)
			}
		}
		count += streams.count()
	}
	return count
}

type prefixStream struct {
	v4    roaring.IntPeekable
	v6    [][16]byte
	pos   int
	value [16]byte
}

func (s *prefixStream) advance() bool {
	if s.v4 != nil {
		if !s.v4.HasNext() {
			return false
		}
		binary.BigEndian.PutUint32(s.value[:4], s.v4.Next())
		return true
	}
	if s.pos == len(s.v6) {
		return false
	}
	s.value = s.v6[s.pos]
	s.pos++
	return true
}

type prefixStreams []*prefixStream

func (h prefixStreams) Len() int           { return len(h) }
func (h prefixStreams) Less(i, j int) bool { return compareIPv6(h[i].value, h[j].value) < 0 }
func (h prefixStreams) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *prefixStreams) Push(v any)        { *h = append(*h, v.(*prefixStream)) }
func (h *prefixStreams) Pop() any {
	last := len(*h) - 1
	v := (*h)[last]
	(*h)[last] = nil
	*h = (*h)[:last]
	return v
}

func (h prefixStreams) count() int {
	heap.Init(&h)
	count := 0
	var previous [16]byte
	for len(h) > 0 {
		stream := h[0]
		if count == 0 || previous != stream.value {
			count++
			previous = stream.value
		}
		if stream.advance() {
			heap.Fix(&h, 0)
		} else {
			heap.Pop(&h)
		}
	}
	return count
}
