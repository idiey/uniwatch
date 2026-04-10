package collector

import (
	"context"
	"fmt"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/load"
)

// collectCPU gathers CPU usage percentage, core count, and load averages.
// It uses interval=0 for an instantaneous CPU percent measurement.
func collectCPU(ctx context.Context) (CPUInfo, error) {
	// Percent with interval=0 returns the instant delta since last call.
	percents, err := cpu.PercentWithContext(ctx, 0, false)
	if err != nil {
		return CPUInfo{}, fmt.Errorf("collector: cpu percent: %w", err)
	}

	var usagePct float64
	if len(percents) > 0 {
		usagePct = percents[0]
	}

	counts, err := cpu.CountsWithContext(ctx, true)
	if err != nil {
		return CPUInfo{}, fmt.Errorf("collector: cpu counts: %w", err)
	}

	avg, err := load.AvgWithContext(ctx)
	if err != nil {
		return CPUInfo{}, fmt.Errorf("collector: load avg: %w", err)
	}

	return CPUInfo{
		Cores:     counts,
		UsagePct:  usagePct,
		LoadAvg1:  avg.Load1,
		LoadAvg5:  avg.Load5,
		LoadAvg15: avg.Load15,
	}, nil
}
