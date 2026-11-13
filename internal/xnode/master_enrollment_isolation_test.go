package xnode

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type blockedEnrollmentRepository struct {
	Repository
	mu      sync.Mutex
	token   []byte
	entered chan struct{}
	release chan struct{}
}

func (r *blockedEnrollmentRepository) RotateXNodeToken(ctx context.Context, _ string, token []byte) error {
	close(r.entered)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-r.release:
	}
	r.mu.Lock()
	r.token = append([]byte(nil), token...)
	r.mu.Unlock()
	return nil
}

func (r *blockedEnrollmentRepository) XNodeNodeByToken(_ context.Context, token []byte) (Node, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !bytes.Equal(r.token, token) {
		return Node{}, ErrNotFound
	}
	return Node{ID: "rotating"}, nil
}

func TestEnrollmentDatabaseWaitPreservesFleetAccessAndFencesOldToken(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	repo := &blockedEnrollmentRepository{token: []byte("old"), entered: make(chan struct{}), release: make(chan struct{})}
	m := NewMaster(NewService(repo, nil, nil, nil, nil), nil)
	m.peers["healthy"] = &agentPeer{nodeID: "healthy"}
	rotated := make(chan error, 1)
	go func() { rotated <- m.rotateEnrollment(ctx, "rotating", []byte("new")) }()
	select {
	case <-repo.entered:
	case <-ctx.Done():
		t.Fatal("rotation did not reach the database")
	}
	connected := make(chan bool, 1)
	go func() { connected <- m.Connected("healthy") }()
	select {
	case live := <-connected:
		if !live {
			t.Fatal("unrelated peer disappeared")
		}
	case <-ctx.Done():
		t.Fatal("enrollment database wait blocked fleet reads")
	}
	registered := make(chan error, 1)
	go func() {
		registered <- m.registerPeer(ctx, "rotating", []byte("old"), wire.AgentHello{}, &agentPeer{nodeID: "rotating"})
	}()
	close(repo.release)
	if err := <-rotated; err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-registered:
		if !errors.Is(err, ErrUnavailable) {
			t.Fatal("old token crossed the committed rotation", err)
		}
	case <-ctx.Done():
		t.Fatal("old-token registration did not finish")
	}
	if m.Connected("rotating") {
		t.Fatal("retired enrollment became a live peer")
	}
}

func TestEnrollmentWaitCancellationReleasesReferences(t *testing.T) {
	m := NewMaster(NewService(nil, nil, nil, nil, nil), nil)
	release, err := m.lockEnrollment(context.Background(), "same")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	waiting := make(chan error, 1)
	go func() {
		release, err := m.lockEnrollment(ctx, "same")
		if release != nil {
			release()
		}
		waiting <- err
	}()
	cancel()
	select {
	case err := <-waiting:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("same-node waiter ignored cancellation")
	}
	release()
	if len(m.enrollmentLocks) != 0 {
		t.Fatal("enrollment locks retained idle node identifiers")
	}
}
