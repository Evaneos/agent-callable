package audit

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Logger writes audit trail entries to a file.
// A nil Logger is safe to use (all methods are no-ops).
type Logger struct {
	mu          sync.Mutex
	file        *os.File
	mode        string // "blocked", "allowed", "all"
	maxEntries  int
	maskSecrets bool
}

// New creates a Logger. Returns nil if file is empty or mode is "none"/empty.
// When maxEntries > 0, the log is trimmed to that many lines on open.
func New(file, mode string, maxEntries int, doMaskSecrets bool) (*Logger, error) {
	if file == "" || mode == "" || mode == "none" {
		return nil, nil
	}

	switch mode {
	case "blocked", "allowed", "all":
	default:
		return nil, fmt.Errorf("audit: invalid mode %q (none, blocked, allowed, all)", mode)
	}

	// Ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		return nil, fmt.Errorf("audit: cannot create directory for %s: %w", file, err)
	}

	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("audit: cannot open %s: %w", file, err)
	}

	// Fix permissions on pre-existing files.
	_ = os.Chmod(file, 0600)

	l := &Logger{file: f, mode: mode, maxEntries: maxEntries, maskSecrets: doMaskSecrets}

	// Rotate on open (each invocation writes at most 1 entry).
	if err := l.rotate(file); err != nil {
		fmt.Fprintf(os.Stderr, "agent-callable: audit rotation warning: %v\n", err)
	}

	return l, nil
}

// rotate trims the log to maxEntries lines, keeping the most recent.
func (l *Logger) rotate(path string) error {
	if l.maxEntries <= 0 {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return nil
	}

	lines := bytes.Split(bytes.TrimRight(data, "\n"), []byte("\n"))
	if len(lines) <= l.maxEntries {
		return nil
	}

	// Keep the last maxEntries lines.
	keep := lines[len(lines)-l.maxEntries:]
	var buf bytes.Buffer
	for _, line := range keep {
		buf.Write(line)
		buf.WriteByte('\n')
	}

	// Rewrite file: close current fd, truncate-write, reopen for append.
	l.file.Close()

	if err := os.WriteFile(path, buf.Bytes(), 0600); err != nil {
		// Try to reopen in append mode even on write failure.
		l.file, _ = os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		return fmt.Errorf("audit: rotation write failed: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("audit: reopen after rotation: %w", err)
	}
	l.file = f
	return nil
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

	if l.maskSecrets {
		cmd = maskSecrets(cmd)
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
