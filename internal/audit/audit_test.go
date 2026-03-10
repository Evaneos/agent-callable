package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewDisabled(t *testing.T) {
	l, err := New("", "", 0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if l != nil {
		t.Fatal("expected nil logger when file is empty")
	}
	// Methods should be safe on nil.
	l.Log("ALLOWED", "kubectl", []string{"get", "pods"})
	l.Close()
}

func TestNewInvalidMode(t *testing.T) {
	dir := t.TempDir()
	_, err := New(filepath.Join(dir, "audit.log"), "invalid", 0, false)
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
}

func TestLogAll(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	l, err := New(path, "all", 0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer l.Close()

	l.Log("ALLOWED", "kubectl", []string{"get", "pods"})
	l.Log("BLOCKED", "git", []string{"push"})

	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), string(data))
	}
	if !strings.Contains(lines[0], "ALLOWED") || !strings.Contains(lines[0], "kubectl get pods") {
		t.Errorf("unexpected first line: %s", lines[0])
	}
	if !strings.Contains(lines[1], "BLOCKED") || !strings.Contains(lines[1], "git push") {
		t.Errorf("unexpected second line: %s", lines[1])
	}
}

func TestLogBlockedOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	l, err := New(path, "blocked", 0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer l.Close()

	l.Log("ALLOWED", "kubectl", []string{"get", "pods"})
	l.Log("BLOCKED", "git", []string{"push"})

	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d: %q", len(lines), string(data))
	}
	if !strings.Contains(lines[0], "BLOCKED") {
		t.Errorf("expected BLOCKED line, got: %s", lines[0])
	}
}

func TestLogAllowedOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	l, err := New(path, "allowed", 0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer l.Close()

	l.Log("ALLOWED", "kubectl", []string{"get", "pods"})
	l.Log("BLOCKED", "git", []string{"push"})

	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d: %q", len(lines), string(data))
	}
	if !strings.Contains(lines[0], "ALLOWED") {
		t.Errorf("expected ALLOWED line, got: %s", lines[0])
	}
}

func TestNewModeNone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	l, err := New(path, "none", 0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if l != nil {
		t.Fatal("expected nil logger for mode=none")
	}
}

func TestNewModeEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	l, err := New(path, "", 0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if l != nil {
		t.Fatal("expected nil logger for empty mode")
	}
}

func TestAuditLabelsFilteredByBlockedMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	l, err := New(path, "blocked", 0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer l.Close()

	l.Log("AUDIT_ALLOWED", "git", []string{"status"})
	l.Log("AUDIT_BLOCKED", "kubectl", []string{"delete", "pod"})

	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d: %q", len(lines), string(data))
	}
	if !strings.Contains(lines[0], "AUDIT_BLOCKED") {
		t.Errorf("expected AUDIT_BLOCKED, got: %s", lines[0])
	}
}

func TestAuditLabelsFilteredByAllowedMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	l, err := New(path, "allowed", 0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer l.Close()

	l.Log("AUDIT_ALLOWED", "git", []string{"status"})
	l.Log("AUDIT_BLOCKED", "kubectl", []string{"delete", "pod"})

	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d: %q", len(lines), string(data))
	}
	if !strings.Contains(lines[0], "AUDIT_ALLOWED") {
		t.Errorf("expected AUDIT_ALLOWED, got: %s", lines[0])
	}
}

func TestAuditLabelsInAllMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	l, err := New(path, "all", 0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer l.Close()

	l.Log("AUDIT_ALLOWED", "git", []string{"status"})
	l.Log("AUDIT_BLOCKED", "kubectl", []string{"delete", "pod"})

	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), string(data))
	}
}

func TestLogNoArgs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	l, err := New(path, "all", 0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer l.Close()

	l.Log("ALLOWED", "docker", nil)

	data, _ := os.ReadFile(path)
	line := strings.TrimSpace(string(data))
	parts := strings.Split(line, "\t")
	if len(parts) != 3 {
		t.Fatalf("expected 3 tab-separated fields, got %d: %q", len(parts), line)
	}
	if parts[2] != "docker" {
		t.Errorf("expected bare 'docker', got %q", parts[2])
	}
}

func TestFilePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	l, err := New(path, "all", 0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	l.Close()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("expected permissions 0600, got %04o", perm)
	}
}

func TestFilePermissionsFixExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	// Create with wrong permissions.
	if err := os.WriteFile(path, []byte("old\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	l, err := New(path, "all", 0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	l.Close()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("expected permissions 0600, got %04o", perm)
	}
}

func TestMkdirParent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "dir", "audit.log")
	l, err := New(path, "all", 0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	l.Close()

	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file to exist: %v", err)
	}
}

func TestRotation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")

	// Pre-fill with 10 lines.
	var buf strings.Builder
	for i := range 10 {
		fmt.Fprintf(&buf, "2025-01-01T00:00:00Z\tALLOWED\tcmd%d\n", i)
	}
	if err := os.WriteFile(path, []byte(buf.String()), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Open with maxEntries=5 triggers rotation.
	l, err := New(path, "all", 5, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	l.Close()

	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines after rotation, got %d", len(lines))
	}
	// Should keep the last 5 (cmd5..cmd9).
	if !strings.Contains(lines[0], "cmd5") {
		t.Errorf("expected first kept line to be cmd5, got: %s", lines[0])
	}
	if !strings.Contains(lines[4], "cmd9") {
		t.Errorf("expected last kept line to be cmd9, got: %s", lines[4])
	}
}

func TestRotationNoTrimWhenUnderLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")

	var buf strings.Builder
	for i := range 3 {
		fmt.Fprintf(&buf, "2025-01-01T00:00:00Z\tALLOWED\tcmd%d\n", i)
	}
	if err := os.WriteFile(path, []byte(buf.String()), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	l, err := New(path, "all", 10, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	l.Close()

	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines (no trim), got %d", len(lines))
	}
}

func TestRotationDisabledWhenZero(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")

	var buf strings.Builder
	for i := range 100 {
		fmt.Fprintf(&buf, "2025-01-01T00:00:00Z\tALLOWED\tcmd%d\n", i)
	}
	if err := os.WriteFile(path, []byte(buf.String()), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	l, err := New(path, "all", 0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	l.Close()

	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 100 {
		t.Fatalf("expected 100 lines (no rotation), got %d", len(lines))
	}
}

func TestMaskSecretsIntegration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	l, err := New(path, "all", 0, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer l.Close()

	l.Log("ALLOWED", "gh", []string{"auth", "login", "--token", "ghp_secret123"})

	data, _ := os.ReadFile(path)
	line := string(data)
	if strings.Contains(line, "ghp_secret123") {
		t.Errorf("expected secret to be masked, got: %s", line)
	}
	if !strings.Contains(line, "****") {
		t.Errorf("expected **** in masked output, got: %s", line)
	}
}

func TestMaskSecretsDisabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	l, err := New(path, "all", 0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer l.Close()

	l.Log("ALLOWED", "gh", []string{"auth", "login", "--token", "ghp_secret123"})

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "ghp_secret123") {
		t.Errorf("expected secret NOT to be masked when disabled, got: %s", string(data))
	}
}

func TestRotationMaxEntriesOne(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	var buf strings.Builder
	for i := range 5 {
		fmt.Fprintf(&buf, "2025-01-01T00:00:00Z\tALLOWED\tcmd%d\n", i)
	}
	if err := os.WriteFile(path, []byte(buf.String()), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	l, err := New(path, "all", 1, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	l.Close()

	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line after rotation, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "cmd4") {
		t.Errorf("expected last entry cmd4, got: %s", lines[0])
	}
}

func TestRotationExactlyAtLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	var buf strings.Builder
	for i := range 5 {
		fmt.Fprintf(&buf, "2025-01-01T00:00:00Z\tALLOWED\tcmd%d\n", i)
	}
	if err := os.WriteFile(path, []byte(buf.String()), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	l, err := New(path, "all", 5, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	l.Close()

	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines (no trim at exact limit), got %d", len(lines))
	}
}

func TestRotationThenWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	var buf strings.Builder
	for i := range 10 {
		fmt.Fprintf(&buf, "2025-01-01T00:00:00Z\tALLOWED\tcmd%d\n", i)
	}
	if err := os.WriteFile(path, []byte(buf.String()), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	l, err := New(path, "all", 5, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Write a new entry after rotation.
	l.Log("BLOCKED", "git", []string{"push"})
	l.Close()

	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 6 {
		t.Fatalf("expected 6 lines (5 kept + 1 new), got %d", len(lines))
	}
	if !strings.Contains(lines[5], "git push") {
		t.Errorf("expected new entry at end, got: %s", lines[5])
	}
}

func TestRotationEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	if err := os.WriteFile(path, []byte(""), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	l, err := New(path, "all", 5, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	l.Log("ALLOWED", "kubectl", []string{"get", "pods"})
	l.Close()

	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
}

func TestRotationNewFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	// File doesn't exist yet — rotation should be a no-op.
	l, err := New(path, "all", 5, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	l.Log("ALLOWED", "git", []string{"status"})
	l.Close()

	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
}

func TestModeFilterWithMasking(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	l, err := New(path, "blocked", 0, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer l.Close()

	// ALLOWED should be filtered out by mode=blocked.
	l.Log("ALLOWED", "gh", []string{"auth", "login", "--token", "ghp_secret"})
	// BLOCKED should be logged and masked.
	l.Log("BLOCKED", "gh", []string{"auth", "login", "--token", "ghp_secret"})

	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line (only blocked), got %d", len(lines))
	}
	if strings.Contains(lines[0], "ghp_secret") {
		t.Errorf("expected secret to be masked, got: %s", lines[0])
	}
	if !strings.Contains(lines[0], "BLOCKED") {
		t.Errorf("expected BLOCKED label, got: %s", lines[0])
	}
}

func TestMaskSecretsEnvVarInLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	l, err := New(path, "all", 0, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer l.Close()

	l.Log("ALLOWED", "env", []string{"DATABASE_URL=postgres://u:p@host/db", "TERM=xterm", "./app"})

	data, _ := os.ReadFile(path)
	line := string(data)
	if strings.Contains(line, "postgres://") {
		t.Errorf("expected DATABASE_URL value to be masked, got: %s", line)
	}
	if !strings.Contains(line, "TERM=xterm") {
		t.Errorf("expected TERM to remain unmasked, got: %s", line)
	}
}

func TestNewErrorBadPath(t *testing.T) {
	// Path to a directory (not a file) — OpenFile should fail.
	dir := t.TempDir()
	_, err := New(dir, "all", 0, false)
	if err == nil {
		t.Fatal("expected error when path is a directory")
	}
}

func TestNewErrorMkdirFail(t *testing.T) {
	// Use /dev/null as parent — cannot create subdirectory in a device file.
	_, err := New("/dev/null/impossible/audit.log", "all", 0, false)
	if err == nil {
		t.Fatal("expected error when parent dir cannot be created")
	}
}

func TestParentDirPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "audit.log")
	l, err := New(path, "all", 0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	l.Close()

	info, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0700 {
		t.Errorf("expected parent dir permissions 0700, got %04o", perm)
	}
}
