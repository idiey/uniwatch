package payload

// HeartbeatPayload represents a heartbeat signal from an agent.
type HeartbeatPayload struct {
	SchemaVersion string `json:"schema_version"`
	AgentID       string `json:"agent_id"`
	PayloadType   string `json:"payload_type"`
	Timestamp     string `json:"timestamp"`
	Status        string `json:"status"`
	AgentVersion  string `json:"agent_version"`
	UptimeSeconds int64  `json:"uptime_seconds"`
}

// HostInfo holds host-level information.
type HostInfo struct {
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Kernel   string `json:"kernel"`
}

// CPUInfo holds CPU metrics.
type CPUInfo struct {
	Cores      int     `json:"cores"`
	UsagePct   float64 `json:"usage_pct"`
	LoadAvg1   float64 `json:"load_avg_1"`
	LoadAvg5   float64 `json:"load_avg_5"`
	LoadAvg15  float64 `json:"load_avg_15"`
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

// DiskInfo holds disk metrics for a single mount point.
type DiskInfo struct {
	Device     string  `json:"device"`
	MountPoint string  `json:"mount_point"`
	TotalBytes int64   `json:"total_bytes"`
	UsedBytes  int64   `json:"used_bytes"`
	FreeByte   int64   `json:"free_bytes"`
	UsedPct    float64 `json:"used_pct"`
}

// NetInfo holds network interface metrics.
type NetInfo struct {
	Interface  string `json:"interface"`
	BytesSent  int64  `json:"bytes_sent"`
	BytesRecv  int64  `json:"bytes_recv"`
	PacketsSent int64 `json:"packets_sent"`
	PacketsRecv int64 `json:"packets_recv"`
}

// WatchedProcess holds info about a monitored process.
type WatchedProcess struct {
	PID     int     `json:"pid"`
	Name    string  `json:"name"`
	CPUPct  float64 `json:"cpu_pct"`
	MemPct  float64 `json:"mem_pct"`
	Status  string  `json:"status"`
}

// ProcessInfo holds aggregate process information.
type ProcessInfo struct {
	TotalCount int              `json:"total_count"`
	Running    int              `json:"running"`
	Watched    []WatchedProcess `json:"watched"`
}

// MetricsPayload represents a full system metrics snapshot.
type MetricsPayload struct {
	SchemaVersion string      `json:"schema_version"`
	AgentID       string      `json:"agent_id"`
	SystemID      string      `json:"system_id"`
	Platform      string      `json:"platform"`
	Environment   string      `json:"environment"`
	Timestamp     string      `json:"timestamp"`
	PayloadType   string      `json:"payload_type"`
	Host          HostInfo    `json:"host"`
	CPU           CPUInfo     `json:"cpu"`
	Memory        MemInfo     `json:"memory"`
	Disk          []DiskInfo  `json:"disk"`
	Network       []NetInfo   `json:"network"`
	Processes     ProcessInfo `json:"processes"`
}

// LogEntry represents a single log line.
type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	TraceID   string                 `json:"trace_id"`
	Fields    map[string]interface{} `json:"fields"`
}

// LogPayload represents a batch of log entries from an agent.
type LogPayload struct {
	SchemaVersion string     `json:"schema_version"`
	AgentID       string     `json:"agent_id"`
	Timestamp     string     `json:"timestamp"`
	PayloadType   string     `json:"payload_type"`
	Service       string     `json:"service"`
	Environment   string     `json:"environment"`
	Entries       []LogEntry `json:"entries"`
}

// APMSpan represents a single distributed trace span.
type APMSpan struct {
	SpanID     string            `json:"span_id"`
	TraceID    string            `json:"trace_id"`
	ParentID   string            `json:"parent_id"`
	Operation  string            `json:"operation"`
	StartTime  string            `json:"start_time"`
	DurationMs float64           `json:"duration_ms"`
	Status     string            `json:"status"`
	Tags       map[string]string `json:"tags"`
}

// APMPayload represents a batch of APM spans from an agent.
type APMPayload struct {
	SchemaVersion string    `json:"schema_version"`
	AgentID       string    `json:"agent_id"`
	Timestamp     string    `json:"timestamp"`
	PayloadType   string    `json:"payload_type"`
	Service       string    `json:"service"`
	Environment   string    `json:"environment"`
	Spans         []APMSpan `json:"spans"`
}
