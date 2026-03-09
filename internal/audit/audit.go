package audit

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// Logger writes audit trail entries to a file.
// A nil Logger is safe to use (all methods are no-ops).
type Logger struct {
	mu   sync.Mutex
	file *os.File
	mode string // "blocked", "allowed", "all"
}

// New creates a Logger. Returns nil if file is empty or mode is "none"/empty.
func New(file, mode string) (*Logger, error) {
	if file == "" || mode == "" || mode == "none" {
		return nil, nil
	}

	switch mode {
	case "blocked", "allowed", "all":
	default:
		return nil, fmt.Errorf("audit: invalid mode %q (none, blocked, allowed, all)", mode)
	}

	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("audit: cannot open %s: %w", file, err)
	}

	return &Logger{file: f, mode: mode}, nil
}

// Log writes an audit entry. Safe to call on a nil Logger.
func (l *Logger) Log(decision, tool string, args []string) {
	if l == nil {
		return
	}

	switch l.mode {
	case "blocked":
		if decision != "BLOCKED" && decision != "AUDIT_BLOCKED" {
			return
		}
	case "allowed":
		if decision != "ALLOWED" && decision != "AUDIT_ALLOWED" {
			return
		}
	}

	ts := time.Now().UTC().Format(time.RFC3339)
	cmd := tool
	if len(args) > 0 {
		cmd = tool + " " + strings.Join(args, " ")
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(l.file, "%s\t%s\t%s\n", ts, decision, cmd)
}

// Close closes the underlying file. Safe to call on a nil Logger.
func (l *Logger) Close() {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.file.Close()
}
