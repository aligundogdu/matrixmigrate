package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Logger provides file-based logging
type Logger struct {
	file   *os.File
	mu     sync.Mutex
	closed bool
}

var (
	instance *Logger
	once     sync.Once
)

// Init initializes the global logger
func Init(dataDir string) error {
	var initErr error
	once = sync.Once{} // Reset for re-initialization
	once.Do(func() {
		logPath := filepath.Join(dataDir, "migration.log")

		// Ensure directory exists
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			initErr = fmt.Errorf("failed to create log directory: %w", err)
			return
		}

		// Open log file (append mode)
		file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			initErr = fmt.Errorf("failed to open log file: %w", err)
			return
		}

		instance = &Logger{file: file}

		// Write session header
		instance.writeHeader()
	})
	return initErr
}

// Close closes the logger
func Close() {
	if instance != nil && !instance.closed {
		instance.mu.Lock()
		defer instance.mu.Unlock()
		instance.file.Close()
		instance.closed = true
	}
}

func (l *Logger) writeHeader() {
	l.mu.Lock()
	defer l.mu.Unlock()
	header := fmt.Sprintf("\n%s\n=== Migration Session Started: %s ===\n%s\n",
		"════════════════════════════════════════════════════════════",
		time.Now().Format("2006-01-02 15:04:05"),
		"════════════════════════════════════════════════════════════")
	l.file.WriteString(header)
}

func (l *Logger) write(level, message string) {
	if l == nil || l.closed {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("15:04:05")
	line := fmt.Sprintf("[%s] %s: %s\n", timestamp, level, message)
	l.file.WriteString(line)
	l.file.Sync() // Flush immediately
}

// Info logs an info message
func Info(format string, args ...interface{}) {
	if instance != nil {
		instance.write("INFO", fmt.Sprintf(format, args...))
	}
}

// Error logs an error message
func Error(format string, args ...interface{}) {
	if instance != nil {
		instance.write("ERROR", fmt.Sprintf(format, args...))
	}
}

// Warn logs a warning message
func Warn(format string, args ...interface{}) {
	if instance != nil {
		instance.write("WARN", fmt.Sprintf(format, args...))
	}
}

// Success logs a success message
func Success(format string, args ...interface{}) {
	if instance != nil {
		instance.write("OK", fmt.Sprintf(format, args...))
	}
}

// Step logs a step/progress message
func Step(stage string, current, total int, item string) {
	if instance != nil {
		if item != "" {
			instance.write("STEP", fmt.Sprintf("[%s] %d/%d: %s", stage, current, total, item))
		} else {
			instance.write("STEP", fmt.Sprintf("[%s] %d/%d", stage, current, total))
		}
	}
}

