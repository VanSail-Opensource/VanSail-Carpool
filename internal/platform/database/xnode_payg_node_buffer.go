package database

import (
	"cmp"
	"context"
	"database/sql"
	"math/big"
	"slices"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type paygNodeMinuteKey struct{ user, node, minute string }
type paygNodeBytes struct{ upload, download big.Int }
type paygNodeMinutes map[paygNodeMinuteKey]*paygNodeBytes

func (m paygNodeMinutes) add(key paygNodeMinuteKey, upload, download *big.Int) {
	value := m[key]
	if value == nil {
		value = &paygNodeBytes{}
		m[key] = value
	}
	value.upload.Add(&value.upload, upload)
	value.download.Add(&value.download, download)
}

func recordPAYGNodeSampleTx(ctx context.Context, tx *sql.Tx, user, node string, sample wire.TrafficSample, buffer *nativeReportBuffer) error {
	now, minute := time.Now().UTC().Truncate(time.Minute), sample.Through.UTC().Truncate(time.Minute)
	if minute.Before(now.Add(-xnode.PAYGWindow)) || minute.After(now) || sample.UploadBytes == 0 && sample.DownloadBytes == 0 {
		return nil
	}
	key := paygNodeMinuteKey{user, node, stamp(minute)}
	upload, download := new(big.Int).SetUint64(sample.UploadBytes), new(big.Int).SetUint64(sample.DownloadBytes)
	if buffer != nil {
		if buffer.paygHistory == nil {
			buffer.paygHistory = paygNodeMinutes{}
		}
		buffer.paygHistory.add(key, upload, download)
		return nil
	}
	values := paygNodeMinutes{}
	values.add(key, upload, download)
	return values.flush(ctx, tx)
}

func (m paygNodeMinutes) flush(ctx context.Context, tx *sql.Tx) error {
	keys := make([]paygNodeMinuteKey, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(a, b paygNodeMinuteKey) int {
		return cmp.Or(cmp.Compare(a.user, b.user), cmp.Compare(a.node, b.node), cmp.Compare(a.minute, b.minute))
	})
	for start := 0; start < len(keys); start += trafficSQLRows {
		var users, nodes, minutes, uploads, downloads []string
		for _, key := range keys[start:min(start+trafficSQLRows, len(keys))] {
			users, nodes, minutes = append(users, key.user), append(nodes, key.node), append(minutes, key.minute)
			uploads, downloads = append(uploads, m[key].upload.String()), append(downloads, m[key].download.String())
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO xnode_payg_node_minutes(user_id,node_id,minute,upload_bytes,download_bytes) SELECT user_id,node_id,minute,upload::numeric,download::numeric FROM unnest($1::text[],$2::text[],$3::timestamptz[],$4::text[],$5::text[]) AS v(user_id,node_id,minute,upload,download) ON CONFLICT(user_id,node_id,minute) DO UPDATE SET upload_bytes=xnode_payg_node_minutes.upload_bytes+excluded.upload_bytes,download_bytes=xnode_payg_node_minutes.download_bytes+excluded.download_bytes`, users, nodes, minutes, uploads, downloads); err != nil {
			return err
		}
	}
	return nil
}
