package app

import (
	"testing"

	"zolik/server/internal/admission"
)

func TestNewAdmissionExplicitCeiling(t *testing.T) {
	c := newAdmission(Config{
		AdmissionMaxConnections:   42,
		AdmissionWaitingRoomRatio: 0.8,
		AdmissionMemoryWatermark:  0.85,
		AdmissionCPUWatermark:     0.25,
	})
	if c == nil {
		t.Fatal("newAdmission returned nil")
	}
	s := c.Snapshot()
	if s.MaxConnections != 42 {
		t.Fatalf("MaxConnections = %d, want 42", s.MaxConnections)
	}
}

func TestNewAdmissionDisabledCeiling(t *testing.T) {
	c := newAdmission(Config{
		AdmissionMaxConnections:   -1,
		AdmissionWaitingRoomRatio: 0.8,
		AdmissionMemoryWatermark:  0.85,
		AdmissionCPUWatermark:     0.25,
	})
	s := c.Snapshot()
	if s.MaxConnections != 0 {
		t.Fatalf("MaxConnections = %d, want 0 when ceiling disabled", s.MaxConnections)
	}
	// With no ceiling configured, many connections still fit.
	for i := 0; i < 100; i++ {
		rel, err := c.Admit(admission.ClassGameplay)
		if err != nil {
			t.Fatalf("Admit %d: %v", i, err)
		}
		rel.Release()
	}
}
