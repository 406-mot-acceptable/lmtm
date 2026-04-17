package ssh

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

var (
	tunnelLogger *log.Logger
	logOnce      sync.Once
)

// tunnelLog returns a logger that writes to ~/.lmtm/tunnel.log.
// The log file is created on first use and kept open for the process lifetime.
func tunnelLog() *log.Logger {
	logOnce.Do(func() {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "/tmp"
		}
		dir := filepath.Join(home, ".lmtm")
		os.MkdirAll(dir, 0700)

		path := filepath.Join(dir, "tunnel.log")
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			// Fallback: discard logs silently.
			tunnelLogger = log.New(os.Stderr, "[tunnel] ", log.LstdFlags)
			return
		}
		tunnelLogger = log.New(f, "", log.LstdFlags)
		fmt.Fprintf(os.Stderr, "tunnel log: %s\n", path)
	})
	return tunnelLogger
}
