package admission

import (
	"os"
	"path/filepath"
	"testing"
)

func writePSI(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cpu.pressure")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadPSISomeAvg10(t *testing.T) {
	path := writePSI(t, "some avg10=12.53 avg60=8.87 avg300=4.42 total=58393\nfull avg10=1.20 avg60=0.80 avg300=0.40 total=12345\n")
	got, ok := readPSISomeAvg10(path)
	if !ok {
		t.Fatal("readPSISomeAvg10 not ok on a well-formed file")
	}
	// The kernel's percentage comes back as a fraction, matching how the
	// watermark is configured.
	if got != 0.1253 {
		t.Fatalf("stall = %v, want 0.1253", got)
	}
}

// The "full" line must not be mistaken for "some" — full means every task
// stalled, which is far past where the gate should have tripped.
func TestReadPSISomeAvg10SkipsTheFullLine(t *testing.T) {
	path := writePSI(t, "full avg10=99.00 avg60=0.00 avg300=0.00 total=1\nsome avg10=2.00 avg60=0.00 avg300=0.00 total=1\n")
	got, ok := readPSISomeAvg10(path)
	if !ok || got != 0.02 {
		t.Fatalf("stall = %v ok=%v, want 0.02 from the some line", got, ok)
	}
}

func TestReadPSISomeAvg10MissingFile(t *testing.T) {
	if _, ok := readPSISomeAvg10(filepath.Join(t.TempDir(), "absent")); ok {
		t.Fatal("ok = true for a missing file")
	}
}

func TestReadPSISomeAvg10Garbage(t *testing.T) {
	path := writePSI(t, "some avg10=not-a-number avg60=0 total=0\n")
	if _, ok := readPSISomeAvg10(path); ok {
		t.Fatal("ok = true for an unparseable avg10")
	}
}
