package harness

import (
	"fmt"
	"net"
	"strconv"
)

func FindFreeLocalAddr() (string, error) {
	if addr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort("localhost", "0")); err != nil {
		return "", fmt.Errorf("failed to resolve a local TCP address: %w", err)
	} else if listener, err := net.ListenTCP("tcp", addr); err != nil {
		return "", fmt.Errorf("failed to listen the randomly chosen addr '%s': %w", addr, err)
	} else {
		if err := listener.Close(); err != nil {
			return "", fmt.Errorf("failed to release chosen addr '%s': %w", addr, err)
		}
		return addr.IP.String() + ":" + strconv.Itoa(listener.Addr().(*net.TCPAddr).Port), nil
	}
}
