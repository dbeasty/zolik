//go:build !unix

package botstats

import "time"

// ProcessCPU is unknown here, so BotCPUShare reads zero rather than a guess.
func ProcessCPU() time.Duration { return 0 }
