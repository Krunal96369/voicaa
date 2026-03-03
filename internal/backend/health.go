package backend

import (
	"fmt"
	"net"
	"time"
)

// WaitForTCP polls a TCP address until a connection succeeds or timeout elapses.
func WaitForTCP(host string, port int, timeoutSec int, intervalSec int) error {
	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
	addr := fmt.Sprintf("%s:%d", host, port)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(time.Duration(intervalSec) * time.Second)
	}
	return fmt.Errorf("server not ready after %d seconds", timeoutSec)
}
