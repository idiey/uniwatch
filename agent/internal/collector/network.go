package collector

import (
	"context"
	"fmt"
	"strings"

	"github.com/shirou/gopsutil/v3/net"
)

// collectNetwork gathers per-interface IO counters, skipping loopback interfaces.
func collectNetwork(ctx context.Context) ([]NetInfo, error) {
	counters, err := net.IOCountersWithContext(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("collector: network io counters: %w", err)
	}

	infos := make([]NetInfo, 0, len(counters))
	for _, c := range counters {
		// Skip loopback interfaces (lo, lo0, Loopback Pseudo-Interface 1, etc.).
		name := strings.ToLower(c.Name)
		if name == "lo" || name == "lo0" || strings.HasPrefix(name, "loopback") {
			continue
		}

		infos = append(infos, NetInfo{
			Interface:   c.Name,
			BytesSent:   int64(c.BytesSent),
			BytesRecv:   int64(c.BytesRecv),
			PacketsSent: int64(c.PacketsSent),
			PacketsRecv: int64(c.PacketsRecv),
		})
	}

	return infos, nil
}
