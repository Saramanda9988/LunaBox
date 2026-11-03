//go:build !windows

package perf

import "time"

func processMemory() (workingSet, private uint64) { return 0, 0 }

func processStartTime() time.Time { return time.Now() }
