package admission

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// cpuGauge reads how starved for CPU this process actually is.
//
// The reading is Linux pressure-stall information (PSI): the fraction of the
// last ten seconds in which at least one runnable task sat waiting for a CPU
// it could not get. That is a truer "out of CPU" signal than load average or
// utilisation — a box pinned at 100% but keeping up shows no stall, while a
// box where players' game actions queue behind each other does. avg10 is the
// window: long enough to ignore a single spike, short enough that the gate
// reopens within seconds of the storm passing.
//
// Where PSI cannot be read — macOS dev machines, older kernels, cgroup v1 —
// the gauge reports not-ok and the gate stays open, the same rule the memory
// gauge follows: an unreadable gauge must not be able to wedge the server
// shut.
type cpuGauge struct {
	// sample reads the current avg10 stall fraction (0..1). A field rather
	// than a method so tests can drive the controller without a PSI file.
	sample func() (stall float64, ok bool)

	ttl time.Duration

	mu        sync.Mutex
	cachedAt  time.Time
	stall     float64
	ok        bool
	haveCache bool
}

func newCPUGauge() *cpuGauge {
	return &cpuGauge{sample: readCPUPressure, ttl: gaugeTTL}
}

// newStubCPUGauge builds a gauge with a fixed reading, for tests.
func newStubCPUGauge(stall float64, ok bool) *cpuGauge {
	return &cpuGauge{
		sample: func() (float64, bool) { return stall, ok },
		ttl:    0,
	}
}

func (g *cpuGauge) read() (stall float64, ok bool) {
	if g == nil || g.sample == nil {
		return 0, false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.haveCache && g.ttl > 0 && time.Since(g.cachedAt) < g.ttl {
		return g.stall, g.ok
	}
	g.stall, g.ok = g.sample()
	g.cachedAt = time.Now()
	g.haveCache = true
	return g.stall, g.ok
}

// The cgroup's own pressure file first — in a container that is the limit
// that matters — then the host-wide file for uncontainered deployments.
const (
	cgroupCPUPressure = "/sys/fs/cgroup/cpu.pressure"
	procCPUPressure   = "/proc/pressure/cpu"
)

func readCPUPressure() (float64, bool) {
	if v, ok := readPSISomeAvg10(cgroupCPUPressure); ok {
		return v, true
	}
	return readPSISomeAvg10(procCPUPressure)
}

// readPSISomeAvg10 pulls avg10 off the "some" line of a PSI file:
//
//	some avg10=1.53 avg60=0.87 avg300=0.42 total=58393
//	full avg10=0.00 ...
//
// "some" (any task stalled) rather than "full" (every task stalled) because
// the gate is meant to trip while the server is degrading, not once it has
// seized entirely. The kernel's percentage is returned as a 0..1 fraction so
// the watermark reads like MemoryHighWatermark does.
func readPSISomeAvg10(path string) (float64, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != "some" {
			continue
		}
		for _, f := range fields[1:] {
			val, found := strings.CutPrefix(f, "avg10=")
			if !found {
				continue
			}
			v, err := strconv.ParseFloat(val, 64)
			if err != nil {
				return 0, false
			}
			return v / 100, true
		}
	}
	return 0, false
}
