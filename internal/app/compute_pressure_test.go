package app

import (
	"testing"
	"time"
)

func TestComputePressureRequiresPersistentOverloadAndRecovery(t *testing.T) {
	s := computePressureSampler{}
	now := time.Now()
	if s.update(.95, now) {
		t.Fatal("one high sample activated offload")
	}
	if !s.update(.95, now.Add(5*time.Second)) {
		t.Fatal("persistent overload did not offload")
	}
	if !s.update(.7, now.Add(10*time.Second)) {
		t.Fatal("one recovery sample resumed local work")
	}
	if !s.update(.8, now.Add(20*time.Second)) {
		t.Fatal("interrupted recovery resumed local work")
	}
	s.update(.7, now.Add(25*time.Second))
	if s.update(.7, now.Add(55*time.Second)) {
		t.Fatal("sustained recovery did not resume local work")
	}
}
