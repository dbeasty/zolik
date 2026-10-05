package capacity

import (
	"os"
	"runtime"
	"strconv"
	"strings"
)

// CPUQuota is how many cores this process may actually use: the container's
// CPU limit where one is set, else GOMAXPROCS.
//
// Not GOMAXPROCS alone. Go sizes GOMAXPROCS from the CPU limit since 1.25, but
// never below 2 — a container limited to one core still schedules onto two —
// so anything that divides the box's CPU between consumers, as the bot
// governor does, would hand out twice what a one-core container has. Found by
// the soak (docs/bot-compute-gating-plan.md §5.7): --cpus=1 read GOMAXPROCS=2.
func CPUQuota() float64 {
	if q, ok := cgroupCPUQuota(); ok {
		return q
	}
	return float64(runtime.GOMAXPROCS(0))
}

func cgroupCPUQuota() (float64, bool) {
	// cgroup v2: "max 100000" (no limit) or "<quota> <period>".
	if b, err := os.ReadFile("/sys/fs/cgroup/cpu.max"); err == nil {
		return parseCPUMax(string(b))
	}
	// cgroup v1: quota is -1 for no limit.
	q, errQ := os.ReadFile("/sys/fs/cgroup/cpu/cpu.cfs_quota_us")
	p, errP := os.ReadFile("/sys/fs/cgroup/cpu/cpu.cfs_period_us")
	if errQ != nil || errP != nil {
		return 0, false
	}
	return parseQuotaPeriod(strings.TrimSpace(string(q)), strings.TrimSpace(string(p)))
}

func parseCPUMax(s string) (float64, bool) {
	f := strings.Fields(s)
	if len(f) != 2 {
		return 0, false
	}
	return parseQuotaPeriod(f[0], f[1])
}

func parseQuotaPeriod(quota, period string) (float64, bool) {
	if quota == "max" || quota == "-1" {
		return 0, false
	}
	q, err1 := strconv.ParseFloat(quota, 64)
	p, err2 := strconv.ParseFloat(period, 64)
	if err1 != nil || err2 != nil || q <= 0 || p <= 0 {
		return 0, false
	}
	return q / p, true
}
