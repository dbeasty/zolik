package app

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/pprof"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

// Where the memory went, answerable from outside the process.
//
// Written because the question "can this server hold more than four players"
// could not be answered from anything the server said about itself. The
// capacity snapshot reports one number — the cgroup reading the admission gate
// decides on — and one number cannot tell apart the three things that add up to
// it: live Go objects, garbage the collector has not got to yet, and file pages
// the kernel is holding on our behalf. Those three have completely different
// answers. Garbage means the collector is doing what it was told; file pages
// mean nothing is wrong at all; live objects mean somebody has to go and look.
//
// So this reports all three, side by side, from the one place that can see all
// three — and, with ?gc=1, will collect first so that what remains is live by
// definition.
//
// Behind DEBUG_ENDPOINTS, which is off unless APP_ENV is local. It exposes the
// process's shape and can be made to stop the world, and neither belongs on a
// public listener.

type cgroupMemory struct {
	// Current is the cgroup's own total, page cache included.
	Current uint64 `json:"currentBytes"`
	Max     uint64 `json:"maxBytes"`
	// Anon is memory with nowhere to go: the Go heap, stacks, everything the
	// kernel would have to swap or OOM over. This is the number that matters.
	Anon uint64 `json:"anonBytes"`
	// File is the page cache for files this process read or wrote — the KDB
	// data directory, mostly. Reclaimable, and counted in Current.
	File         uint64 `json:"fileBytes"`
	ActiveFile   uint64 `json:"activeFileBytes"`
	InactiveFile uint64 `json:"inactiveFileBytes"`
	Slab         uint64 `json:"slabBytes"`
	Sock         uint64 `json:"sockBytes"`
	// Gauged is what the admission controller actually gates on:
	// Current minus the pages it considers freely reclaimable.
	Gauged uint64 `json:"gaugedBytes"`
}

type goMemory struct {
	// HeapAlloc is live-and-not-yet-collected: reachable objects plus garbage.
	HeapAlloc uint64 `json:"heapAllocBytes"`
	// HeapInuse is the address space those objects sit in, and HeapIdle the
	// span the runtime has kept back for next time. HeapReleased is the part
	// of that it has already handed to the kernel.
	HeapInuse    uint64 `json:"heapInuseBytes"`
	HeapIdle     uint64 `json:"heapIdleBytes"`
	HeapReleased uint64 `json:"heapReleasedBytes"`
	HeapObjects  uint64 `json:"heapObjects"`
	StackInuse   uint64 `json:"stackInuseBytes"`
	MSpanInuse   uint64 `json:"mspanInuseBytes"`
	MCacheInuse  uint64 `json:"mcacheInuseBytes"`
	GCSys        uint64 `json:"gcSysBytes"`
	OtherSys     uint64 `json:"otherSysBytes"`
	// Sys is everything the runtime has taken from the OS, ever, minus what
	// it has given back.
	Sys uint64 `json:"sysBytes"`
	// NextGC is the heap size the collector is aiming at. Read it beside
	// GOMEMLIMIT: a NextGC near the limit is a collector that has been told
	// it may use that much, not a leak.
	NextGC     uint64 `json:"nextGCBytes"`
	NumGC      uint32 `json:"numGC"`
	GOMEMLIMIT int64  `json:"goMemLimitBytes"`
	Goroutines int    `json:"goroutines"`
}

type memoryReport struct {
	// Collected says whether a collection was forced before these numbers
	// were taken. When it is true, HeapAlloc is live data by definition,
	// which is the only reading that answers "how much does this actually
	// need".
	Collected bool         `json:"collected"`
	Cgroup    cgroupMemory `json:"cgroup"`
	Go        goMemory     `json:"go"`
	// Budgets are the four thresholds this process is running under, in
	// bytes, so they can be read against the numbers above rather than
	// worked out from four different files.
	Budgets map[string]int64 `json:"budgets"`
}

// registerDebugRoutes hangs the memory report and pprof off the router.
func (a *App) registerDebugRoutes(r chi.Router) {
	r.Get("/debug/memory", a.memoryReport)

	// The standard profiles, on the same flag. `/debug/pprof/heap?gc=1` is
	// the one that names names: it attributes live bytes to the call site
	// that allocated them, which is the difference between knowing the heap
	// is large and knowing what is in it.
	r.HandleFunc("/debug/pprof/", pprof.Index)
	r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	r.HandleFunc("/debug/pprof/profile", pprof.Profile)
	r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	r.HandleFunc("/debug/pprof/trace", pprof.Trace)
	// Named profiles do not have their own functions; Index dispatches them,
	// but only when it is mounted at the path it expects.
	for _, name := range []string{"goroutine", "heap", "allocs", "threadcreate", "block", "mutex"} {
		r.Handle("/debug/pprof/"+name, pprof.Handler(name))
	}
}

func (a *App) memoryReport(w http.ResponseWriter, req *http.Request) {
	rep := memoryReport{}

	// A collection first, when asked. Two calls rather than one: runtime.GC
	// makes the garbage collectable, and FreeOSMemory also returns the freed
	// spans to the kernel, which is what makes the cgroup reading below move
	// as well as the Go one. Expensive and stop-the-world, hence opt-in.
	if req.URL.Query().Get("gc") != "" {
		runtime.GC()
		debug.FreeOSMemory()
		rep.Collected = true
	}

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	rep.Go = goMemory{
		HeapAlloc:    ms.HeapAlloc,
		HeapInuse:    ms.HeapInuse,
		HeapIdle:     ms.HeapIdle,
		HeapReleased: ms.HeapReleased,
		HeapObjects:  ms.HeapObjects,
		StackInuse:   ms.StackInuse,
		MSpanInuse:   ms.MSpanInuse,
		MCacheInuse:  ms.MCacheInuse,
		GCSys:        ms.GCSys,
		OtherSys:     ms.OtherSys,
		Sys:          ms.Sys,
		NextGC:       ms.NextGC,
		NumGC:        ms.NumGC,
		GOMEMLIMIT:   debug.SetMemoryLimit(-1),
		Goroutines:   runtime.NumGoroutine(),
	}

	rep.Cgroup = readCgroupBreakdown()
	rep.Budgets = a.memoryBudgets(rep.Cgroup.Max)

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(rep)
}

// memoryBudgets restates the thresholds this process runs under. They live in
// four different places by design — the collector's, the admission gate's, the
// storage engine's write shedding and its cache pool — and the point of listing
// them together is that their *order* is the thing that decides how the server
// behaves under pressure, and no single file shows the order.
func (a *App) memoryBudgets(limit uint64) map[string]int64 {
	if limit == 0 {
		return map[string]int64{}
	}
	l := float64(limit)
	return map[string]int64{
		"cgroupLimit":         int64(limit),
		"admissionWatermark":  int64(l * a.cfg.AdmissionMemoryWatermark),
		"goMemLimit":          debug.SetMemoryLimit(-1),
		"kdbWriteShedding":    int64(l * 0.85),
		"kdbHotTierCachePool": int64(l * 0.5),
	}
}

func readCgroupBreakdown() cgroupMemory {
	var c cgroupMemory
	c.Current, _ = readUint("/sys/fs/cgroup/memory.current")
	c.Max, _ = readUint("/sys/fs/cgroup/memory.max")
	stat := readStat("/sys/fs/cgroup/memory.stat")
	if c.Current == 0 && c.Max == 0 {
		c.Current, _ = readUint("/sys/fs/cgroup/memory/memory.usage_in_bytes")
		c.Max, _ = readUint("/sys/fs/cgroup/memory/memory.limit_in_bytes")
		stat = readStat("/sys/fs/cgroup/memory/memory.stat")
	}
	c.Anon = stat["anon"] + stat["total_rss"]
	c.File = stat["file"] + stat["total_cache"]
	c.ActiveFile = stat["active_file"] + stat["total_active_file"]
	c.InactiveFile = stat["inactive_file"] + stat["total_inactive_file"]
	c.Slab = stat["slab"]
	c.Sock = stat["sock"]
	if c.Current > c.InactiveFile {
		c.Gauged = c.Current - c.InactiveFile
	}
	return c
}

func readUint(path string) (uint64, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	s := strings.TrimSpace(string(b))
	if s == "" || s == "max" {
		return 0, false
	}
	v, err := strconv.ParseUint(s, 10, 64)
	return v, err == nil
}

func readStat(path string) map[string]uint64 {
	out := map[string]uint64{}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		name, value, found := strings.Cut(strings.TrimSpace(sc.Text()), " ")
		if !found {
			continue
		}
		if v, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64); err == nil {
			out[name] = v
		}
	}
	return out
}
