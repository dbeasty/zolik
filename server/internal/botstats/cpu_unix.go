//go:build unix

package botstats

import (
	"syscall"
	"time"
)

// ProcessCPU is user+system CPU this process has used since it started, or
// zero where the platform cannot say.
func ProcessCPU() time.Duration {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
}
