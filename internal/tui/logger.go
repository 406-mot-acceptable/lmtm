package tui

import (
	"fmt"
	"sync"
	"time"
)

// LogLevel represents the severity of a log message
type LogLevel int

const (
	LogDebug LogLevel = iota
	LogInfo
	LogWarning
	LogError
)

func (l LogLevel) String() string {
	switch l {
	case LogDebug:
		return "DEBUG"
	case LogInfo:
		return "INFO"
	case LogWarning:
		return "WARN"
	case LogError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

func (l LogLevel) Color() string {
	switch l {
	case LogDebug:
		return "240" // Gray
	case LogInfo:
		return "12"  // Blue
	case LogWarning:
		return "11"  // Yellow
	case LogError:
		return "9"   // Red
	default:
		return "7"
	}
}

// LogEntry represents a single log message
type LogEntry struct {
	Time    time.Time
	Level   LogLevel
	Message string
}

// Logger handles application logging
type Logger struct {
	entries []LogEntry
	mu      sync.RWMutex
	maxSize int
}

// NewLogger creates a new logger with a maximum number of entries
func NewLogger(maxSize int) *Logger {
	return &Logger{
		entries: make([]LogEntry, 0, maxSize),
		maxSize: maxSize,
	}
}

// Log adds a log entry
func (l *Logger) Log(level LogLevel, format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := LogEntry{
		Time:    time.Now(),
		Level:   level,
		Message: fmt.Sprintf(format, args...),
	}

	l.entries = append(l.entries, entry)

	// Keep only last maxSize entries
	if len(l.entries) > l.maxSize {
		l.entries = l.entries[len(l.entries)-l.maxSize:]
	}
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	l.Log(LogDebug, format, args...)
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	l.Log(LogInfo, format, args...)
}

// Warning logs a warning message
func (l *Logger) Warning(format string, args ...interface{}) {
	l.Log(LogWarning, format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.Log(LogError, format, args...)
}

// GetEntries returns all log entries (most recent last)
func (l *Logger) GetEntries() []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// Return a copy
	entries := make([]LogEntry, len(l.entries))
	copy(entries, l.entries)
	return entries
}

// GetRecent returns the most recent N entries
func (l *Logger) GetRecent(n int) []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if n > len(l.entries) {
		n = len(l.entries)
	}

	entries := make([]LogEntry, n)
	copy(entries, l.entries[len(l.entries)-n:])
	return entries
}

// Clear clears all log entries
func (l *Logger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = make([]LogEntry, 0, l.maxSize)
}
