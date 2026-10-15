package session

import (
	"os"
	"syscall"
)

// CleanStale removes sessions whose PID is no longer alive.
// Returns the number of sessions removed.
func CleanStale() (int, error) {
	s, err := loadActive()
	if err != nil {
		return 0, err
	}

	removed := 0
	for id, info := range s.Sessions {
		if !isAlive(info.PID) {
			os.Remove(info.SocketPath)
			delete(s.Sessions, id)
			removed++
		}
	}

	if removed > 0 {
		if err := saveActive(s); err != nil {
			return removed, err
		}
	}
	return removed, nil
}

func isAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 checks if process exists without actually sending a signal
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
