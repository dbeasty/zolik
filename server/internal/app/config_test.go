package app

import (
	"testing"
)

func TestLoadConfigAdmissionDefaults(t *testing.T) {
	t.Setenv("ADMISSION_MAX_CONNECTIONS", "")
	t.Setenv("ADMISSION_WAITING_ROOM_RATIO", "")
	t.Setenv("ADMISSION_MEMORY_WATERMARK", "")
	t.Setenv("ADMISSION_CPU_WATERMARK", "")

	cfg := LoadConfig()
	if cfg.AdmissionMaxConnections != 0 {
		t.Fatalf("AdmissionMaxConnections = %d, want 0", cfg.AdmissionMaxConnections)
	}
	if cfg.AdmissionWaitingRoomRatio != 0.8 {
		t.Fatalf("AdmissionWaitingRoomRatio = %v, want 0.8", cfg.AdmissionWaitingRoomRatio)
	}
	if cfg.AdmissionMemoryWatermark != 0.85 {
		t.Fatalf("AdmissionMemoryWatermark = %v, want 0.85", cfg.AdmissionMemoryWatermark)
	}
	if cfg.AdmissionCPUWatermark != 0.25 {
		t.Fatalf("AdmissionCPUWatermark = %v, want 0.25", cfg.AdmissionCPUWatermark)
	}
}

func TestLoadConfigAdmissionFromEnv(t *testing.T) {
	t.Setenv("ADMISSION_MAX_CONNECTIONS", "5000")
	t.Setenv("ADMISSION_WAITING_ROOM_RATIO", "0.75")
	t.Setenv("ADMISSION_MEMORY_WATERMARK", "0.9")
	t.Setenv("ADMISSION_CPU_WATERMARK", "0.3")

	cfg := LoadConfig()
	if cfg.AdmissionMaxConnections != 5000 {
		t.Fatalf("AdmissionMaxConnections = %d, want 5000", cfg.AdmissionMaxConnections)
	}
	if cfg.AdmissionWaitingRoomRatio != 0.75 {
		t.Fatalf("AdmissionWaitingRoomRatio = %v, want 0.75", cfg.AdmissionWaitingRoomRatio)
	}
	if cfg.AdmissionMemoryWatermark != 0.9 {
		t.Fatalf("AdmissionMemoryWatermark = %v, want 0.9", cfg.AdmissionMemoryWatermark)
	}
	if cfg.AdmissionCPUWatermark != 0.3 {
		t.Fatalf("AdmissionCPUWatermark = %v, want 0.3", cfg.AdmissionCPUWatermark)
	}
}

func TestLoadConfigAdmissionInvalidEnvFallsBack(t *testing.T) {
	t.Setenv("ADMISSION_MAX_CONNECTIONS", "not-a-number")
	t.Setenv("ADMISSION_CPU_WATERMARK", "bad")

	cfg := LoadConfig()
	if cfg.AdmissionMaxConnections != 0 {
		t.Fatalf("AdmissionMaxConnections = %d, want fallback 0", cfg.AdmissionMaxConnections)
	}
	if cfg.AdmissionCPUWatermark != 0.25 {
		t.Fatalf("AdmissionCPUWatermark = %v, want fallback 0.25", cfg.AdmissionCPUWatermark)
	}
}

func TestLoadConfigAdmissionMaxConnectionsNegativeOne(t *testing.T) {
	t.Setenv("ADMISSION_MAX_CONNECTIONS", "-1")
	cfg := LoadConfig()
	if cfg.AdmissionMaxConnections != -1 {
		t.Fatalf("AdmissionMaxConnections = %d, want -1", cfg.AdmissionMaxConnections)
	}
}
