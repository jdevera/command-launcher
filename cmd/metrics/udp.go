package metrics

import (
	"fmt"
	"net"
)

func sendUDP(addr string, packets []string) error {
	conn, err := net.Dial("udp", addr)
	if err != nil {
		return fmt.Errorf("udp dial %s: %w", addr, err)
	}
	defer conn.Close()

	for _, packet := range packets {
		if _, err := conn.Write([]byte(packet)); err != nil {
			return fmt.Errorf("udp write: %w", err)
		}
	}
	return nil
}
