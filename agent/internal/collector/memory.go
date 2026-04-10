package collector

import (
	"context"
	"fmt"

	"github.com/shirou/gopsutil/v3/mem"
)

// collectMemory gathers virtual memory and swap statistics.
func collectMemory(ctx context.Context) (MemInfo, error) {
	vm, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return MemInfo{}, fmt.Errorf("collector: virtual memory: %w", err)
	}

	swap, err := mem.SwapMemoryWithContext(ctx)
	if err != nil {
		return MemInfo{}, fmt.Errorf("collector: swap memory: %w", err)
	}

	return MemInfo{
		TotalBytes:     int64(vm.Total),
		UsedBytes:      int64(vm.Used),
		FreeBytes:      int64(vm.Free),
		UsedPct:        vm.UsedPercent,
		SwapTotalBytes: int64(swap.Total),
		SwapUsedBytes:  int64(swap.Used),
	}, nil
}
