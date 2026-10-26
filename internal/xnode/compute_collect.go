package xnode

import (
	"context"
	"encoding/base64"
	"errors"
	"time"

	"github.com/VanSail-Opensource/X-Node/pkg/compute"
)

func (p *computePool) collect(ctx context.Context, a *computeAttempt, peer *agentPeer) (compute.Results, error) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	chunks := map[int][]byte{}
	total := 0
	size := 0
	hash := ""
	for {
		select {
		case <-ctx.Done():
			return compute.Results{}, ctx.Err()
		case <-peer.ctx.Done():
			return compute.Results{}, ErrUnavailable
		case <-tick.C:
			p.mu.Lock()
			expired := time.Now().After(a.lease)
			p.mu.Unlock()
			if expired {
				return compute.Results{}, errors.New("calculation lease expired")
			}
		case chunk := <-a.chunks:
			if chunk.Total < 1 || chunk.Total > compute.MaxCompressedBytes/compute.ChunkBytes || chunk.Index < 0 || chunk.Index >= chunk.Total || len(chunk.Data) > base64.StdEncoding.EncodedLen(compute.ChunkBytes) || len(chunk.ResultHash) != 64 {
				return compute.Results{}, ErrInvalid
			}
			if total != 0 && (total != chunk.Total || hash != chunk.ResultHash) {
				return compute.Results{}, ErrInvalid
			}
			total, hash = chunk.Total, chunk.ResultHash
			part, err := base64.StdEncoding.DecodeString(chunk.Data)
			if err != nil || len(part) > compute.ChunkBytes {
				return compute.Results{}, ErrInvalid
			}
			if previous, seen := chunks[chunk.Index]; seen {
				if compute.Digest(previous) != compute.Digest(part) {
					return compute.Results{}, ErrInvalid
				}
				continue
			}
			chunks[chunk.Index] = part
			size += len(part)
			if size > compute.MaxCompressedBytes {
				return compute.Results{}, ErrInvalid
			}
			if len(chunks) == total {
				raw := make([]byte, 0, size)
				for i := 0; i < total; i++ {
					raw = append(raw, chunks[i]...)
				}
				if compute.Digest(raw) != hash {
					return compute.Results{}, ErrInvalid
				}
				return compute.Decode(raw)
			}
		}
	}
}
