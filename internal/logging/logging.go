// internal/logging/logging.go
//
// Two-stream logging:
//   - Human activity log: rotated plain-text (one line per meaningful event).
//   - Engineering JSONL: structured JSON, also rotated, for diagnostics.
//
// Both streams redact secrets before writing (see redact.go).

package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Level is the log verbosity level.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Logger is the main logger struct. It writes to both streams simultaneously.
type Logger struct {
	mu       sync.Mutex
	activity io.Writer // human-readable
	jsonl    io.Writer // structured JSONL
	minLevel Level
}

// New creates a Logger writing to the given writers.
// Pass os.Stdout / os.Stderr for development; use RotatingFile for production.
func New(activity, jsonl io.Writer, minLevel Level) *Logger {
	return &Logger{activity: activity, jsonl: jsonl, minLevel: minLevel}
}

// entry is the JSON structure written to the JSONL stream.
type entry struct {
	TS     string         `json:"ts"`
	Level  string         `json:"level"`
	Msg    string         `json:"msg"`
	Fields map[string]any `json:"fields,omitempty"`
}

// Info logs a human-readable message at INFO level.
func (l *Logger) Info(msg string, fields ...Field) { l.log(LevelInfo, msg, fields) }

// Warn logs at WARN level.
func (l *Logger) Warn(msg string, fields ...Field) { l.log(LevelWarn, msg, fields) }

// Error logs at ERROR level.
func (l *Logger) Error(msg string, fields ...Field) { l.log(LevelError, msg, fields) }

// Debug logs at DEBUG level.
func (l *Logger) Debug(msg string, fields ...Field) { l.log(LevelDebug, msg, fields) }

// Field is a key-value pair attached to a log entry.
type Field struct {
	Key   string
	Value any
}

// F creates a Field.
func F(key string, value any) Field { return Field{Key: key, Value: value} }

func (l *Logger) log(level Level, msg string, fields []Field) {
	if level < l.minLevel {
		return
	}

	ts := time.Now().UTC().Format(time.RFC3339)
	redactedMsg := RedactString(msg)

	fm := make(map[string]any, len(fields))
	for _, f := range fields {
		fm[f.Key] = RedactField(f.Key, f.Value)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Human activity stream.
	if l.activity != nil {
		line := fmt.Sprintf("%s [%s] %s", ts, level.String(), redactedMsg)
		for k, v := range fm {
			line += fmt.Sprintf(" %s=%v", k, v)
		}
		_, _ = fmt.Fprintln(l.activity, line)
	}

	// JSONL stream.
	if l.jsonl != nil {
		e := entry{TS: ts, Level: level.String(), Msg: redactedMsg, Fields: fm}
		b, _ := json.Marshal(e)
		_, _ = fmt.Fprintf(l.jsonl, "%s\n", b)
	}
}

// RotatingFile implements io.Writer with size- and age-based rotation.
type RotatingFile struct {
	mu       sync.Mutex
	path     string
	maxBytes int64
	maxAge   time.Duration
	file     *os.File
	size     int64
	openedAt time.Time
}

// NewRotatingFile opens (or creates) path and rotates when the file exceeds
// maxBytes or maxAge. Pass 0 for either to disable that limit.
func NewRotatingFile(path string, maxBytes int64, maxAge time.Duration) (*RotatingFile, error) {
	rf := &RotatingFile{
		path:     path,
		maxBytes: maxBytes,
		maxAge:   maxAge,
	}
	if err := rf.open(); err != nil {
		return nil, err
	}
	return rf, nil
}

func (rf *RotatingFile) open() error {
	f, err := os.OpenFile(rf.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	rf.file = f
	rf.size = info.Size()
	rf.openedAt = time.Now()
	return nil
}

func (rf *RotatingFile) Write(p []byte) (int, error) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if rf.shouldRotate(int64(len(p))) {
		if err := rf.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := rf.file.Write(p)
	rf.size += int64(n)
	return n, err
}

func (rf *RotatingFile) shouldRotate(incoming int64) bool {
	if rf.maxBytes > 0 && rf.size+incoming > rf.maxBytes {
		return true
	}
	if rf.maxAge > 0 && time.Since(rf.openedAt) > rf.maxAge {
		return true
	}
	return false
}

func (rf *RotatingFile) rotate() error {
	if rf.file != nil {
		_ = rf.file.Close()
	}
	ts := time.Now().UTC().Format("20060102-150405")
	rotated := rf.path + "." + ts
	_ = os.Rename(rf.path, rotated)
	return rf.open()
}

// Close flushes and closes the underlying file.
func (rf *RotatingFile) Close() error {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if rf.file != nil {
		return rf.file.Close()
	}
	return nil
}
