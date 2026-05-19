package metrics

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStatsd_DisabledWhenHostEmpty(t *testing.T) {
	m := NewStatsdMetricsCollector("cola", "", 0, "")
	_, ok := m.(*noopMetrics)
	assert.True(t, ok, "empty host should yield a no-op collector")

	// no-op methods don't panic and don't try to dial.
	assert.NoError(t, m.Collect(3, "default", "pkg", "grp", "cmd"))
	assert.NoError(t, m.Send(0, nil))
}

func TestStatsd_TagFormatting(t *testing.T) {
	m := &statsdMetrics{launcher: "cola"}
	m.repo = "default"
	m.pkg = "hotfix"
	m.cmd = "hotfix"
	m.subcmd = "create"
	m.partition = 3

	tags := m.formatTags("ok")

	for _, want := range []string{
		"launcher:cola",
		"repo:default",
		"pkg:hotfix",
		"cmd:hotfix",
		"subcmd:create",
		"partition:3",
		"status:ok",
	} {
		assert.Contains(t, tags, want)
	}
}

func TestStatsd_SanitizeTagValue(t *testing.T) {
	for _, tc := range []struct {
		in, out string
	}{
		{"normal", "normal"},
		{"with space", "with_space"},
		{"with,comma", "with_comma"},
		{"with|pipe", "with_pipe"},
		{"with#hash", "with_hash"},
		{"with\nnewline", "with_newline"},
	} {
		assert.Equal(t, tc.out, sanitizeTagValue(tc.in), "input: %q", tc.in)
	}
}

// TestStatsd_EndToEnd_UDP verifies the on-wire packet shape against a real
// UDP listener — no parsing library, just byte-level matching. Sanity check
// that Send writes well-formed DogStatsD lines and that Collect → Send
// preserves all the dimensions through the tag map.
func TestStatsd_EndToEnd_UDP(t *testing.T) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	assert.NoError(t, err)
	defer conn.Close()

	port := conn.LocalAddr().(*net.UDPAddr).Port
	m := NewStatsdMetricsCollector("cola", "127.0.0.1", port, "test.")

	assert.NoError(t, m.Collect(7, "default", "hotfix", "hotfix", "create"))
	assert.NoError(t, m.Send(0, nil))

	packets := readPackets(t, conn, 2, 500*time.Millisecond)
	assert.Len(t, packets, 2)

	hasDuration := false
	hasCount := false
	for _, p := range packets {
		switch {
		case strings.HasPrefix(p, "test.duration:") && strings.Contains(p, "|ms|#"):
			hasDuration = true
			assertCommonTags(t, p)
		case strings.HasPrefix(p, "test.count:1|c|#"):
			hasCount = true
			assertCommonTags(t, p)
		}
	}
	assert.True(t, hasDuration, "missing duration packet, got: %v", packets)
	assert.True(t, hasCount, "missing count packet, got: %v", packets)
}

func TestStatsd_StatusReflectsExitCode(t *testing.T) {
	for _, tc := range []struct {
		name     string
		exit     int
		err      error
		wantTag  string
	}{
		{"exit 0 + nil err", 0, nil, "status:ok"},
		{"exit 1", 1, nil, "status:ko"},
		{"non-nil err", 0, fmt.Errorf("boom"), "status:ko"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
			assert.NoError(t, err)
			defer conn.Close()

			port := conn.LocalAddr().(*net.UDPAddr).Port
			m := NewStatsdMetricsCollector("cola", "127.0.0.1", port, "")

			assert.NoError(t, m.Collect(0, "r", "p", "c", "s"))
			assert.NoError(t, m.Send(tc.exit, tc.err))

			packets := readPackets(t, conn, 2, 500*time.Millisecond)
			for _, p := range packets {
				assert.Contains(t, p, tc.wantTag)
			}
		})
	}
}

func assertCommonTags(t *testing.T, packet string) {
	t.Helper()
	for _, want := range []string{
		"launcher:cola",
		"repo:default",
		"pkg:hotfix",
		"cmd:hotfix",
		"subcmd:create",
		"partition:7",
		"status:ok",
	} {
		assert.Contains(t, packet, want, "missing tag in packet: %s", packet)
	}
}

func readPackets(t *testing.T, conn *net.UDPConn, want int, deadline time.Duration) []string {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(deadline))
	out := []string{}
	buf := make([]byte, 4096)
	for len(out) < want {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			break
		}
		out = append(out, string(buf[:n]))
	}
	return out
}
