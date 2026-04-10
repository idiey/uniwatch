package collector

import (
	"context"
	"sync"
	"time"

	"github.com/idiey/uniwatch-agent/internal/config"
	"github.com/rs/zerolog"
)

// MetricsPayload is the top-level JSON sent to the hub for each collection cycle.
type MetricsPayload struct {
	SchemaVersion string      `json:"schema_version"`
	AgentID       string      `json:"agent_id"`
	Timestamp     string      `json:"timestamp"`
	PayloadType   string      `json:"payload_type"`
	CPU           CPUInfo     `json:"cpu"`
	Memory        MemInfo     `json:"memory"`
	Disk          []DiskInfo  `json:"disk"`
	Network       []NetInfo   `json:"network"`
	Processes     ProcessInfo `json:"processes"`
}

// CPUInfo holds CPU metrics.
type CPUInfo struct {
	Cores     int     `json:"cores"`
	UsagePct  float64 `json:"usage_pct"`
	LoadAvg1  float64 `json:"load_avg_1"`
	LoadAvg5  float64 `json:"load_avg_5"`
	LoadAvg15 float64 `json:"load_avg_15"`
}

// MemInfo holds memory metrics.
type MemInfo struct {
	TotalBytes     int64   `json:"total_bytes"`
	UsedBytes      int64   `json:"used_bytes"`
	FreeBytes      int64   `json:"free_bytes"`
	UsedPct        float64 `json:"used_pct"`
	SwapTotalBytes int64   `json:"swap_total_bytes"`
	SwapUsedBytes  int64   `json:"swap_used_bytes"`
}

// DiskInfo holds per-mount disk metrics.
type DiskInfo struct {
	MountPoint string  `json:"mount_point"`
	TotalBytes int64   `json:"total_bytes"`
	UsedBytes  int64   `json:"used_bytes"`
	FreeBytes  int64   `json:"free_bytes"`
	UsedPct    float64 `json:"used_pct"`
}

// NetInfo holds per-interface network counters.
type NetInfo struct {
	Interface   string `json:"interface"`
	BytesSent   int64  `json:"bytes_sent"`
	BytesRecv   int64  `json:"bytes_recv"`
	PacketsSent int64  `json:"packets_sent"`
	PacketsRecv int64  `json:"packets_recv"`
}

// WatchedProcess holds metrics for a single monitored process.
type WatchedProcess struct {
	Name   string  `json:"name"`
	Status string  `json:"status"`
	PID    int32   `json:"pid"`
	CPUPct float64 `json:"cpu_pct"`
	MemMB  float64 `json:"mem_mb"`
}

// ProcessInfo holds aggregate and per-process info.
type ProcessInfo struct {
	TotalRunning int              `json:"total_running"`
	Watched      []WatchedProcess `json:"watched"`
}

// Collector gathers system metrics on a configurable interval.
type Collector struct {
	cfg zerolog.Logger
	log zerolog.Logger
	c   *config.Config
}

// New creates a Collector backed by cfg and using log for structured logging.
func New(cfg *config.Config, log zerolog.Logger) *Collector {
	return &Collector{
		c:   cfg,
		log: log,
	}
}

// collectResult carries a single sub-collector's outcome.
type collectResult struct {
	cpu  CPUInfo
	mem  MemInfo
	disk []DiskInfo
	net  []NetInfo
	proc ProcessInfo
	err  error
}

// Collect performs a single collection run, gathering all metrics in parallel,
// and returns a MetricsPayload ready to be forwarded to the hub.
func (c *Collector) Collect(ctx context.Context) (MetricsPayload, error) {
	type cpuResult struct {
		v   CPUInfo
		err error
	}
	type memResult struct {
		v   MemInfo
		err error
	}
	type diskResult struct {
		v   []DiskInfo
		err error
	}
	type netResult struct {
		v   []NetInfo
		err error
	}
	type procResult struct {
		v   ProcessInfo
		err error
	}

	cpuCh := make(chan cpuResult, 1)
	memCh := make(chan memResult, 1)
	diskCh := make(chan diskResult, 1)
	netCh := make(chan netResult, 1)
	procCh := make(chan procResult, 1)

	var wg sync.WaitGroup
	wg.Add(5)

	go func() {
		defer wg.Done()
		v, err := collectCPU(ctx)
		cpuCh <- cpuResult{v, err}
	}()
	go func() {
		defer wg.Done()
		v, err := collectMemory(ctx)
		memCh <- memResult{v, err}
	}()
	go func() {
		defer wg.Done()
		v, err := collectDisk(ctx, c.c, c.log)
		diskCh <- diskResult{v, err}
	}()
	go func() {
		defer wg.Done()
		v, err := collectNetwork(ctx)
		netCh <- netResult{v, err}
	}()
	go func() {
		defer wg.Done()
		v, err := collectProcesses(ctx, c.c.MonitorServices)
		procCh <- procResult{v, err}
	}()

	wg.Wait()

	cpuRes := <-cpuCh
	if cpuRes.err != nil {
		return MetricsPayload{}, cpuRes.err
	}

	memRes := <-memCh
	if memRes.err != nil {
		return MetricsPayload{}, memRes.err
	}

	diskRes := <-diskCh
	if diskRes.err != nil {
		return MetricsPayload{}, diskRes.err
	}

	netRes := <-netCh
	if netRes.err != nil {
		return MetricsPayload{}, netRes.err
	}

	procRes := <-procCh
	if procRes.err != nil {
		return MetricsPayload{}, procRes.err
	}

	payload := MetricsPayload{
		SchemaVersion: "1.0",
		AgentID:       c.c.AgentID,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		PayloadType:   "metrics",
		CPU:           cpuRes.v,
		Memory:        memRes.v,
		Disk:          diskRes.v,
		Network:       netRes.v,
		Processes:     procRes.v,
	}

	c.log.Debug().
		Str("agent_id", c.c.AgentID).
		Str("timestamp", payload.Timestamp).
		Msg("collector: metrics collected successfully")

	return payload, nil
}

// Start collects metrics immediately, then every cfg.CollectIntervalDuration,
// sending each MetricsPayload to out. Blocks until ctx is cancelled.
func (c *Collector) Start(ctx context.Context, out chan<- MetricsPayload) error {
	collect := func() {
		payload, err := c.Collect(ctx)
		if err != nil {
			c.log.Error().Err(err).Msg("collector: collection failed")
			return
		}
		select {
		case out <- payload:
		case <-ctx.Done():
		}
	}

	// Collect immediately on first call.
	collect()

	ticker := time.NewTicker(c.c.CollectIntervalDuration)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			collect()
		}
	}
}
