package metrics

import (
	"fmt"
	"net"
	"strings"
	"time"
)

const (
	defaultStatsdPort   = 8125
	defaultStatsdPrefix = "launcher."
)

// statsdMetrics emits DogStatsD-format metrics over UDP. Dimensions live in
// tags rather than the metric name, so the same emit code talks to the
// Datadog agent, statsd_exporter, the OTel collector, Telegraf, Vector, or a
// hand-rolled receiver — receiver-side mapping decides how each dimension is
// rolled up or labelled.
//
// Off by default: a constructor call with an empty host returns a no-op
// implementation that never opens a socket.
type statsdMetrics struct {
	addr   string
	prefix string

	launcher       string
	repo           string
	pkg            string
	cmd            string
	subcmd         string
	partition      uint8
	startTimestamp time.Time
}

// NewStatsdMetricsCollector returns a Metrics implementation that emits
// DogStatsD-format packets to host:port. host == "" disables the exporter
// entirely (Send becomes a no-op, no socket is opened). prefix is prepended
// to every metric name; defaults to "launcher." when empty.
func NewStatsdMetricsCollector(launcher, host string, port int, prefix string) Metrics {
	if host == "" {
		return &noopMetrics{}
	}
	if port == 0 {
		port = defaultStatsdPort
	}
	if prefix == "" {
		prefix = defaultStatsdPrefix
	}
	return &statsdMetrics{
		addr:     fmt.Sprintf("%s:%d", host, port),
		prefix:   prefix,
		launcher: launcher,
	}
}

func (m *statsdMetrics) Collect(uid uint8, repo, pkg, group, name string) error {
	if group == "" {
		return fmt.Errorf("unknown command")
	}
	m.repo = repo
	m.pkg = pkg
	m.cmd = group
	m.subcmd = name
	m.partition = uid
	m.startTimestamp = time.Now()
	return nil
}

func (m *statsdMetrics) Send(cmdExitCode int, cmdError error) error {
	status := "ok"
	if cmdError != nil || cmdExitCode != 0 {
		status = "ko"
	}
	durationMs := time.Since(m.startTimestamp).Milliseconds()
	tags := m.formatTags(status)

	packets := []string{
		fmt.Sprintf("%sduration:%d|ms|#%s", m.prefix, durationMs, tags),
		fmt.Sprintf("%scount:1|c|#%s", m.prefix, tags),
	}
	return sendUDP(m.addr, packets)
}

func (m *statsdMetrics) formatTags(status string) string {
	pairs := []string{
		"launcher:" + sanitizeTagValue(m.launcher),
		"repo:" + sanitizeTagValue(m.repo),
		"pkg:" + sanitizeTagValue(m.pkg),
		"cmd:" + sanitizeTagValue(m.cmd),
		"subcmd:" + sanitizeTagValue(m.subcmd),
		fmt.Sprintf("partition:%d", m.partition),
		"status:" + status,
	}
	return strings.Join(pairs, ",")
}

// sanitizeTagValue strips characters that would break the DogStatsD wire
// format (commas, pipes, hashes, newlines) and replaces them with
// underscores. Tag keys are statically defined above so they don't need
// sanitising; values come from user-provided command/package names so they
// can in principle contain anything.
func sanitizeTagValue(v string) string {
	r := strings.NewReplacer(",", "_", "|", "_", "#", "_", "\n", "_", " ", "_")
	return r.Replace(v)
}

func sendUDP(addr string, packets []string) error {
	conn, err := net.Dial("udp", addr)
	if err != nil {
		return fmt.Errorf("statsd dial %s: %w", addr, err)
	}
	defer conn.Close()
	for _, p := range packets {
		if _, err := conn.Write([]byte(p)); err != nil {
			return fmt.Errorf("statsd write: %w", err)
		}
	}
	return nil
}

type noopMetrics struct{}

func (n *noopMetrics) Collect(uid uint8, repo, pkg, group, name string) error { return nil }
func (n *noopMetrics) Send(cmdExitCode int, cmdError error) error             { return nil }
