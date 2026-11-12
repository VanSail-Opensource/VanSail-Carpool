package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func unusedNodeDeletionFixture(t *testing.T) *Store {
	t.Helper()
	s := newTestStore(t)
	if err := s.SaveXNodeConfiguration(context.Background(), xnode.Configuration{ID: "delete-config", Name: "Delete fixture", DocumentCipher: "encrypted"}); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestDirectNodeDeleteRequiresUnusedDisabledState(t *testing.T) {
	s := unusedNodeDeletionFixture(t)
	ctx := context.Background()
	for _, state := range []string{"enabled", "enrolled", "published", "retired", "removing", "stream_history", "usage_history", "endpoint"} {
		t.Run(state, func(t *testing.T) {
			node := xnode.Node{ID: state, Name: state, ConfigID: "delete-config", SettingsCipher: "encrypted", Enabled: state == "enabled"}
			if err := s.SaveXNodeNode(ctx, node); err != nil {
				t.Fatal(err)
			}
			var err error
			at := stamp(time.Now().UTC())
			switch state {
			case "enrolled":
				err = s.BindXNodeAgent(ctx, node.ID, "offline-enrolled-agent")
			case "published":
				_, err = s.db.ExecContext(ctx, `UPDATE xnode_nodes SET desired_revision=1 WHERE id=$1`, node.ID)
			case "retired":
				_, err = s.db.ExecContext(ctx, `UPDATE xnode_nodes SET retired_at=$2 WHERE id=$1`, node.ID, at)
			case "removing":
				_, err = s.db.ExecContext(ctx, `UPDATE xnode_nodes SET removing_at=$2 WHERE id=$1`, node.ID, at)
			case "stream_history":
				_, err = s.db.ExecContext(ctx, `INSERT INTO xnode_traffic_streams(agent_id,stream_id,node_id,has_gaps,created_at,updated_at) VALUES('history-agent','stream',$1,true,$2,$2)`, node.ID, at)
			case "usage_history":
				_, err = s.db.ExecContext(ctx, `INSERT INTO xnode_usage_coverage(node_id,agent_id,stream_id,sequence,digest,reason,observed_at,dropped_native,dropped_hourly) VALUES($1,'coverage-agent','stream',1,'digest','gap',$2,1,0)`, node.ID, time.Now().UTC())
			case "endpoint":
				_, err = s.db.ExecContext(ctx, `INSERT INTO xnode_endpoints(id,node_id,inbound_tag,name,address,port,overrides_cipher,created_at,updated_at) VALUES('endpoint',$1,'paid','Endpoint','example.com',443,'encrypted',$2,$2)`, node.ID, at)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = s.DeleteXNodeNode(ctx, node.ID); !errors.Is(err, xnode.ErrConflict) {
				t.Fatal("ineligible direct deletion did not require durable removal", err)
			}
			if _, err = s.XNodeNode(ctx, node.ID); err != nil {
				t.Fatal("protected node disappeared", err)
			}
		})
	}
	node := xnode.Node{ID: "unused", Name: "Unused", ConfigID: "delete-config", SettingsCipher: "encrypted"}
	if err := s.SaveXNodeNode(ctx, node); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := s.DeleteXNodeNode(ctx, node.ID); err != nil {
			t.Fatal("unused cleanup/replay failed", err)
		}
	}
	if _, err := s.XNodeNode(ctx, node.ID); !errors.Is(err, xnode.ErrNotFound) {
		t.Fatal("unused node was not deleted", err)
	}
}

func TestDirectNodeDeleteRechecksAConcurrentFirstEnrollment(t *testing.T) {
	s := unusedNodeDeletionFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	node := xnode.Node{ID: "enrolling", Name: "Enrolling", ConfigID: "delete-config", SettingsCipher: "encrypted"}
	if err := s.SaveXNodeNode(ctx, node); err != nil {
		t.Fatal(err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE xnode_nodes SET agent_id='new-agent' WHERE id=$1`, node.ID); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- s.DeleteXNodeNode(ctx, node.ID) }()
	blocked := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err = s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database()
 AND pid<>pg_backend_pid() AND wait_event_type='Lock' AND strpos(query,'SELECT enabled=0')>0 AND strpos(query,'FOR UPDATE')>0)`).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !blocked {
		t.Fatal("direct deletion did not wait for first enrollment")
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = <-done; !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("concurrent enrollment was deleted", err)
	}
	current, err := s.XNodeNode(ctx, node.ID)
	if err != nil || current.AgentID != "new-agent" {
		t.Fatal("enrolled node was not preserved", current, err)
	}
}
