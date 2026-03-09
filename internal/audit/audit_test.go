package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewDisabled(t *testing.T) {
	l, err := New("", "")
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
	_, err := New(filepath.Join(dir, "audit.log"), "invalid")
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
}

func TestLogAll(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	l, err := New(path, "all")
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
	l, err := New(path, "blocked")
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
	l, err := New(path, "allowed")
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
	l, err := New(path, "none")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if l != nil {
		t.Fatal("expected nil logger for mode=none")
	}
}

func TestNewModeEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	l, err := New(path, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if l != nil {
		t.Fatal("expected nil logger for empty mode")
	}
}

func TestAuditLabelsFilteredByBlockedMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	l, err := New(path, "blocked")
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
	l, err := New(path, "allowed")
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
	l, err := New(path, "all")
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
	l, err := New(path, "all")
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
