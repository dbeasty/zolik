package admission

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// memoryGauge reads how much memory this process is actually allowed and how
// much of it is genuinely spoken for.
//
// "Genuinely" is the whole difficulty. A cgroup's memory.current counts the
// page cache, and the KDB engine writes its data files through that cache — so
// a server that has been running a while reads as near its limit while most of
// that figure is clean file pages the kernel would drop without a second
// thought. Gating on the raw number would refuse players to protect memory
// that was never under threat, and would get worse the longer the process ran.
//
// So the reading subtracts the reclaimable file pages (inactive_file), which
// is the same distinction the kernel itself makes when deciding whether to
// reclaim or to invoke the OOM killer. What is left is roughly the anonymous
// memory that actually has nowhere to go — the part a new connection adds to.
type memoryGauge struct {
	// sample reads the current (used, limit) pair. A field rather than a
	// method so tests can drive the controller without a cgroup filesystem.
	sample func() (used, limit uint64, ok bool)

	// Readings are cached briefly: Admit runs on every connection, and three
	// small file reads per arrival during a connection storm is exactly the
	// wrong time to add syscalls.
	ttl time.Duration

	mu        sync.Mutex
	cachedAt  time.Time
	used      uint64
	limit     uint64
	ok        bool
	haveCache bool
}

const gaugeTTL = time.Second

func newMemoryGauge() *memoryGauge {
	return &memoryGauge{sample: readCgroupMemory, ttl: gaugeTTL}
}

// newStubGauge builds a gauge with a fixed reading, for tests.
func newStubGauge(used, limit uint64, ok bool) *memoryGauge {
	return &memoryGauge{
		sample: func() (uint64, uint64, bool) { return used, limit, ok },
		ttl:    0,
	}
}

func (g *memoryGauge) read() (used, limit uint64, ok bool) {
	if g == nil || g.sample == nil {
		return 0, 0, false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.haveCache && g.ttl > 0 && time.Since(g.cachedAt) < g.ttl {
		return g.used, g.limit, g.ok
	}
	g.used, g.limit, g.ok = g.sample()
	g.cachedAt = time.Now()
	g.haveCache = true
	return g.used, g.limit, g.ok
}

// cgroup v2 paths, then v1. Both are read relative to the standard mount
// point; a process in a container sees its own limits there.
const (
	v2Current = "/sys/fs/cgroup/memory.current"
	v2Max     = "/sys/fs/cgroup/memory.max"
	v2Stat    = "/sys/fs/cgroup/memory.stat"

	v1Usage = "/sys/fs/cgroup/memory/memory.usage_in_bytes"
	v1Limit = "/sys/fs/cgroup/memory/memory.limit_in_bytes"
	v1Stat  = "/sys/fs/cgroup/memory/memory.stat"
)

// v1NoLimit is what cgroup v1 reports when nothing is capped: a sentinel near
// the top of the address space rather than a word like "max". Treating it as a
// real limit would make every unconstrained host read as 0% used and would
// hide a genuinely misconfigured one, so it is rejected outright.
const v1NoLimit = uint64(1) << 62

func readCgroupMemory() (used, limit uint64, ok bool) {
	if used, limit, ok = readCgroupV2(); ok {
		return used, limit, true
	}
	return readCgroupV1()
}

func readCgroupV2() (uint64, uint64, bool) {
	limit, ok := readUintFile(v2Max)
	if !ok {
		// "max" — no limit set on this cgroup.
		return 0, 0, false
	}
	current, ok := readUintFile(v2Current)
	if !ok {
		return 0, 0, false
	}
	reclaimable := statField(v2Stat, "inactive_file")
	return subFloor(current, reclaimable), limit, true
}

func readCgroupV1() (uint64, uint64, bool) {
	limit, ok := readUintFile(v1Limit)
	if !ok || limit >= v1NoLimit {
		return 0, 0, false
	}
	usage, ok := readUintFile(v1Usage)
	if !ok {
		return 0, 0, false
	}
	reclaimable := statField(v1Stat, "total_inactive_file")
	return subFloor(usage, reclaimable), limit, true
}

func subFloor(a, b uint64) uint64 {
	if b >= a {
		return 0
	}
	return a - b
}

// readUintFile reads a file holding a single integer. A "max" (cgroup v2's
// spelling of "no limit") reports not-ok rather than a number, so callers
// cannot mistake it for a very large limit.
func readUintFile(path string) (uint64, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	s := strings.TrimSpace(string(b))
	if s == "" || s == "max" {
		return 0, false
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// statField pulls one "name value" line out of a cgroup memory.stat. A missing
// file or field is 0, which makes the caller's subtraction a no-op — the
// conservative direction, since it can only make the gauge read higher.
func statField(path, field string) uint64 {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		name, value, found := strings.Cut(strings.TrimSpace(sc.Text()), " ")
		if !found || name != field {
			continue
		}
		v, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
		if err != nil {
			return 0
		}
		return v
	}
	return 0
}

// MemoryLimit reports the memory ceiling this process is running under, and
// whether one could be read at all. app wiring uses it to derive a connection
// ceiling and to set GOMEMLIMIT.
func MemoryLimit() (uint64, bool) {
	_, limit, ok := readCgroupMemory()
	return limit, ok
}
