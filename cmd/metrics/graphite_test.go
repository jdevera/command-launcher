package metrics

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGraphite_EndToEnd_UDP(t *testing.T) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	require.NoError(t, err)
	defer conn.Close()

	m := &graphiteMetrics{graphiteAddr: conn.LocalAddr().String()}
	require.NoError(t, m.Collect(7, "default", "hotfix", "hotfix", "create"))
	require.NoError(t, m.Send(0, nil))

	packets := readPackets(t, conn, 3, 500*time.Millisecond)
	require.Len(t, packets, 3)

	prefix := "devtools.cdt.hotfix.hotfix.create.7."
	metrics := make(map[string][]string, len(packets))
	for _, packet := range packets {
		parts := strings.Fields(packet)
		require.Len(t, parts, 3, "invalid Graphite packet: %q", packet)
		metrics[strings.TrimPrefix(parts[0], prefix)] = parts
	}

	require.Contains(t, metrics, "count")
	require.Contains(t, metrics, "ok")
	require.Contains(t, metrics, "duration")
	assert.Equal(t, "1", metrics["count"][1])
	assert.Equal(t, "1", metrics["ok"][1])
	assert.NotEmpty(t, metrics["duration"][1])
	assert.Equal(t, metrics["count"][2], metrics["ok"][2])
	assert.Equal(t, metrics["count"][2], metrics["duration"][2])
}

func TestGraphite_FailureStatus(t *testing.T) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	require.NoError(t, err)
	defer conn.Close()

	m := &graphiteMetrics{graphiteAddr: conn.LocalAddr().String()}
	require.NoError(t, m.Collect(0, "default", "pkg", "group", "command"))
	require.NoError(t, m.Send(1, fmt.Errorf("command failed")))

	packets := readPackets(t, conn, 3, 500*time.Millisecond)
	require.Len(t, packets, 3)
	assert.Contains(t, packets, "devtools.cdt.pkg.group.command.0.ko 1 "+fmt.Sprint(m.StartTimestamp.Unix())+"\n")
}

func TestGraphite_DialFailureReturnsError(t *testing.T) {
	m := &graphiteMetrics{graphiteAddr: "invalid address"}
	require.NoError(t, m.Collect(0, "default", "pkg", "group", "command"))
	assert.Error(t, m.Send(0, nil))
}
