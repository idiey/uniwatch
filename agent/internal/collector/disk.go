package collector

import (
	"context"
	"fmt"

	"github.com/idiey/uniwatch-agent/internal/config"
	"github.com/rs/zerolog"
	"github.com/shirou/gopsutil/v3/disk"
)

// collectDisk gathers per-partition disk usage. It logs a WARNING via log if
// any mount point's used percentage exceeds cfg.DiskAlertThreshold.
func collectDisk(ctx context.Context, cfg *config.Config, log zerolog.Logger) ([]DiskInfo, error) {
	partitions, err := disk.PartitionsWithContext(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("collector: disk partitions: %w", err)
	}

	infos := make([]DiskInfo, 0, len(partitions))
	for _, p := range partitions {
		usage, err := disk.UsageWithContext(ctx, p.Mountpoint)
		if err != nil {
			// Non-fatal: log and skip mounts that cannot be stat'd.
			log.Warn().
				Err(err).
				Str("mount_point", p.Mountpoint).
				Msg("collector: disk usage unavailable for mount")
			continue
		}

		info := DiskInfo{
			MountPoint: p.Mountpoint,
			TotalBytes: int64(usage.Total),
			UsedBytes:  int64(usage.Used),
			FreeBytes:  int64(usage.Free),
			UsedPct:    usage.UsedPercent,
		}

		if usage.UsedPercent >= float64(cfg.DiskAlertThreshold) {
			log.Warn().
				Str("mount_point", p.Mountpoint).
				Float64("used_pct", usage.UsedPercent).
				Int("threshold_pct", cfg.DiskAlertThreshold).
				Msg("collector: disk usage exceeds alert threshold")
		}

		infos = append(infos, info)
	}

	return infos, nil
}

