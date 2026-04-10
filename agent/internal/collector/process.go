package collector

import (
	"context"
	"fmt"
	"strings"

	"github.com/shirou/gopsutil/v3/process"
)

// collectProcesses scans all running processes and matches them against
// watchedNames. For each watched name it reports running/stopped, PID, CPU%,
// and memory in MB. TotalRunning is the count of watched names that are running.
func collectProcesses(ctx context.Context, watchedNames []string) (ProcessInfo, error) {
	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return ProcessInfo{}, fmt.Errorf("collector: list processes: %w", err)
	}

	// Build a lookup: lower-case name → first matching process.
	type procEntry struct {
		pid    int32
		cpuPct float64
		memMB  float64
	}
	found := make(map[string]procEntry, len(watchedNames))

	for _, p := range procs {
		name, err := p.NameWithContext(ctx)
		if err != nil {
			// Process may have exited between listing and querying — skip.
			continue
		}

		lname := strings.ToLower(name)
		for _, wname := range watchedNames {
			if strings.ToLower(wname) == lname {
				if _, already := found[strings.ToLower(wname)]; already {
					break // keep the first match
				}
				cpuPct, cpuErr := p.CPUPercentWithContext(ctx)
				if cpuErr != nil {
					cpuPct = 0
				}

				memInfo, memErr := p.MemoryInfoWithContext(ctx)
				var memMB float64
				if memErr == nil && memInfo != nil {
					memMB = float64(memInfo.RSS) / (1024 * 1024)
				}

				found[strings.ToLower(wname)] = procEntry{
					pid:    p.Pid,
					cpuPct: cpuPct,
					memMB:  memMB,
				}
				break
			}
		}
	}

	watched := make([]WatchedProcess, 0, len(watchedNames))
	totalRunning := 0

	for _, name := range watchedNames {
		entry, running := found[strings.ToLower(name)]
		wp := WatchedProcess{
			Name: name,
		}
		if running {
			wp.Status = "running"
			wp.PID = entry.pid
			wp.CPUPct = entry.cpuPct
			wp.MemMB = entry.memMB
			totalRunning++
		} else {
			wp.Status = "stopped"
			wp.PID = 0
			wp.CPUPct = 0
			wp.MemMB = 0
		}
		watched = append(watched, wp)
	}

	return ProcessInfo{
		TotalRunning: totalRunning,
		Watched:      watched,
	}, nil
}
