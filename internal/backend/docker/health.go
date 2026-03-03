package docker

import "github.com/Krunal96369/voicaa/internal/backend"

// WaitForReady polls a TCP address until a connection succeeds or timeout elapses.
func WaitForReady(host string, port int, timeoutSec int, intervalSec int) error {
	return backend.WaitForTCP(host, port, timeoutSec, intervalSec)
}
