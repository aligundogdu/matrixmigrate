package migration

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// MaxLogFileSize is the maximum size of a log file before rotation (10MB)
	MaxLogFileSize = 10 * 1024 * 1024

	// LogStatusSuccess indicates a successful operation
	LogStatusSuccess = "SUCCESS"
	// LogStatusFailed indicates a failed operation
	LogStatusFailed = "FAILED"
	// LogStatusSkipped indicates a skipped operation
	LogStatusSkipped = "SKIPPED"
)

// Logger handles migration logging to a file
type Logger struct {
	mu       sync.Mutex
	file     *os.File
	filePath string
	size     int64
}

// NewLogger creates a new logger instance
func NewLogger(dataDir string) (*Logger, error) {
	logPath := filepath.Join(dataDir, "migration.log")

	// Check if file exists and get size
	var size int64
	if info, err := os.Stat(logPath); err == nil {
		size = info.Size()
	}

	// Open file in append mode
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return &Logger{
		file:     file,
		filePath: logPath,
		size:     size,
	}, nil
}

// Close closes the log file
func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp time.Time
	Stage     string
	Item      string
	Status    string
	Error     string
}

// Log writes a log entry to the file
func (l *Logger) Log(entry LogEntry) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Check if rotation is needed
	if l.size >= MaxLogFileSize {
		if err := l.rotate(); err != nil {
			return err
		}
	}

	// Format the log line
	line := l.formatEntry(entry)

	// Write to file
	n, err := l.file.WriteString(line)
	if err != nil {
		return fmt.Errorf("failed to write log: %w", err)
	}
	l.size += int64(n)

	return nil
}

// formatEntry formats a log entry as a plain text line
func (l *Logger) formatEntry(entry LogEntry) string {
	timestamp := entry.Timestamp.Format("2006-01-02 15:04:05")
	
	if entry.Error != "" {
		return fmt.Sprintf("[%s] [%s] %s: %s - %s | Error: %s\n",
			timestamp, entry.Status, entry.Stage, entry.Item, entry.Status, entry.Error)
	}
	
	return fmt.Sprintf("[%s] [%s] %s: %s\n",
		timestamp, entry.Status, entry.Stage, entry.Item)
}

// rotate rotates the log file
func (l *Logger) rotate() error {
	// Close current file
	if err := l.file.Close(); err != nil {
		return fmt.Errorf("failed to close log file: %w", err)
	}

	// Rename current file with timestamp
	timestamp := time.Now().Format("20060102-150405")
	rotatedPath := l.filePath + "." + timestamp

	if err := os.Rename(l.filePath, rotatedPath); err != nil {
		return fmt.Errorf("failed to rotate log file: %w", err)
	}

	// Open new file
	file, err := os.OpenFile(l.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to create new log file: %w", err)
	}

	l.file = file
	l.size = 0

	return nil
}

// LogSuccess logs a successful operation
func (l *Logger) LogSuccess(stage, item string) error {
	return l.Log(LogEntry{
		Timestamp: time.Now(),
		Stage:     stage,
		Item:      item,
		Status:    LogStatusSuccess,
	})
}

// LogFailed logs a failed operation
func (l *Logger) LogFailed(stage, item string, err error) error {
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	return l.Log(LogEntry{
		Timestamp: time.Now(),
		Stage:     stage,
		Item:      item,
		Status:    LogStatusFailed,
		Error:     errMsg,
	})
}

// LogSkipped logs a skipped operation
func (l *Logger) LogSkipped(stage, item, reason string) error {
	return l.Log(LogEntry{
		Timestamp: time.Now(),
		Stage:     stage,
		Item:      item,
		Status:    LogStatusSkipped,
		Error:     reason,
	})
}

// WriteHeader writes a section header to the log
func (l *Logger) WriteHeader(title string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	header := fmt.Sprintf("\n=== %s [%s] ===\n", title, timestamp)
	
	n, err := l.file.WriteString(header)
	if err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}
	l.size += int64(n)

	return nil
}
