package metrics

import (
	"fmt"
	"net"
	"strconv"
	"time"
)

const (
	graphitePort = 3341
)

type graphiteMetrics struct {
	graphiteAddr   string
	PkgName        string
	CmdName        string
	SubCmdName     string
	StartTimestamp time.Time
	UserPartition  uint8
}

func NewGraphiteMetricsCollector(host string) Metrics {
	return &graphiteMetrics{
		graphiteAddr: net.JoinHostPort(host, strconv.Itoa(graphitePort)),
	}
}

func (metrics *graphiteMetrics) Collect(uid uint8, repo, pkg, group, name string) error {
	if group == "" {
		return fmt.Errorf("unknown command")
	}

	metrics.PkgName = pkg
	metrics.CmdName = group
	metrics.SubCmdName = name
	metrics.StartTimestamp = time.Now()
	metrics.UserPartition = uid

	return nil
}

func (metrics *graphiteMetrics) Send(cmdExitCode int, cmdError error) error {
	duration := time.Now().UnixNano() - metrics.StartTimestamp.UnixNano()
	timestamp := metrics.StartTimestamp.Unix()
	prefix := metrics.prefix()
	graphiteMetrics := []string{
		fmt.Sprintf("%s.duration %d %d\n", prefix, duration, timestamp),
		fmt.Sprintf("%s.count 1 %d\n", prefix, timestamp),
	}

	if cmdError != nil || cmdExitCode != 0 {
		graphiteMetrics = append(graphiteMetrics, fmt.Sprintf("%s.ko 1 %d\n", prefix, timestamp))
	} else {
		graphiteMetrics = append(graphiteMetrics, fmt.Sprintf("%s.ok 1 %d\n", prefix, timestamp))
	}

	return sendUDP(metrics.graphiteAddr, graphiteMetrics)
}

func (metrics *graphiteMetrics) prefix() string {
	return fmt.Sprintf("devtools.cdt.%s.%s.%s.%d", metrics.PkgName, metrics.CmdName, metrics.SubCmdName, metrics.UserPartition)
}
