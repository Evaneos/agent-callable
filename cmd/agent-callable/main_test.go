package main

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/evaneos/agent-callable/internal/shell"
)

// enableAllBuiltins creates a temp config dir with all builtins and tools enabled.
func enableAllBuiltins(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("AGENT_CALLABLE_CONFIG_DIR", dir)
	shell.GenerateDefaults(dir)
}

func TestRunAuditAllowed(t *testing.T) {
	enableAllBuiltins(t)
	code := run([]string{"--audit", "git", "status"})
	if code != 0 {
		t.Fatalf("expected exit 0 for allowed audit, got %d", code)
	}
}

func TestRunAuditBlocked(t *testing.T) {
	enableAllBuiltins(t)
	code := run([]string{"--audit", "kubectl", "delete", "pod", "x"})
	if code != 1 {
		t.Fatalf("expected exit 1 for blocked audit, got %d", code)
	}
}

func TestRunAuditUnknownTool(t *testing.T) {
	enableAllBuiltins(t)
	code := run([]string{"--audit", "unknown-tool", "arg"})
	if code != 1 {
		t.Fatalf("expected exit 1 for unknown tool audit, got %d", code)
	}
}

func TestRunAuditNoArgs(t *testing.T) {
	enableAllBuiltins(t)
	code := run([]string{"--audit"})
	if code != 2 {
		t.Fatalf("expected exit 2 for --audit without args, got %d", code)
	}
}

func TestRunAuditShellAllowed(t *testing.T) {
	enableAllBuiltins(t)
	code := run([]string{"--audit", "--sh", "git status | grep main"})
	if code != 0 {
		t.Fatalf("expected exit 0 for allowed shell audit, got %d", code)
	}
}

func TestRunAuditShellBlocked(t *testing.T) {
	enableAllBuiltins(t)
	code := run([]string{"--audit", "--sh", "kubectl delete pod x && echo done"})
	if code != 1 {
		t.Fatalf("expected exit 1 for blocked shell audit, got %d", code)
	}
}

func TestRunAuditShellNoExpr(t *testing.T) {
	enableAllBuiltins(t)
	code := run([]string{"--audit", "--sh"})
	if code != 2 {
		t.Fatalf("expected exit 2 for --audit --sh without expression, got %d", code)
	}
}

// captureStdout runs fn and returns whatever it wrote to os.Stdout.
func captureStdout(fn func()) string {
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	b, _ := io.ReadAll(r)
	return string(b)
}

func TestRunClaude_ShellAllowed(t *testing.T) {
	enableAllBuiltins(t)
	var code int
	out := captureStdout(func() {
		code = run([]string{"--claude", "git status"})
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, `"permissionDecision":"allow"`) {
		t.Fatalf("expected allow JSON, got %q", out)
	}
}

func TestRunClaude_ShellBlocked(t *testing.T) {
	enableAllBuiltins(t)
	var code int
	out := captureStdout(func() {
		code = run([]string{"--claude", "git push origin main"})
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if strings.TrimSpace(out) != "" {
		t.Fatalf("expected empty stdout for blocked command, got %q", out)
	}
}

func TestRunClaude_PipeAllowed(t *testing.T) {
	enableAllBuiltins(t)
	var code int
	out := captureStdout(func() {
		code = run([]string{"--claude", "git log | head -5"})
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, `"permissionDecision":"allow"`) {
		t.Fatalf("expected allow JSON, got %q", out)
	}
}

func TestRunClaude_ToolAllowed(t *testing.T) {
	enableAllBuiltins(t)
	var code int
	out := captureStdout(func() {
		code = run([]string{"--claude", "git", "status"})
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, `"permissionDecision":"allow"`) {
		t.Fatalf("expected allow JSON, got %q", out)
	}
}

func TestRunClaude_ToolBlocked(t *testing.T) {
	enableAllBuiltins(t)
	var code int
	out := captureStdout(func() {
		code = run([]string{"--claude", "git", "push"})
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if strings.TrimSpace(out) != "" {
		t.Fatalf("expected empty stdout for blocked tool, got %q", out)
	}
}

func TestRunClaude_NoArgs(t *testing.T) {
	enableAllBuiltins(t)
	code := run([]string{"--claude"})
	if code != 0 {
		t.Fatalf("expected exit 0 for --claude without args, got %d", code)
	}
}

// --- Wrapper tool end-to-end tests ---

func TestRunAuditWrapperTimeoutAllowed(t *testing.T) {
	enableAllBuiltins(t)
	code := run([]string{"--audit", "timeout", "5", "git", "log"})
	if code != 0 {
		t.Fatalf("expected exit 0 for timeout git log, got %d", code)
	}
}

func TestRunAuditWrapperTimeoutBlocked(t *testing.T) {
	enableAllBuiltins(t)
	code := run([]string{"--audit", "timeout", "5", "git", "push"})
	if code != 1 {
		t.Fatalf("expected exit 1 for timeout git push, got %d", code)
	}
}

func TestRunAuditWrapperNiceAllowed(t *testing.T) {
	enableAllBuiltins(t)
	code := run([]string{"--audit", "nice", "-n", "10", "git", "status"})
	if code != 0 {
		t.Fatalf("expected exit 0 for nice git status, got %d", code)
	}
}

func TestRunAuditWrapperTimeoutUnknownInner(t *testing.T) {
	enableAllBuiltins(t)
	code := run([]string{"--audit", "timeout", "5", "unknown-cmd"})
	if code != 1 {
		t.Fatalf("expected exit 1 for timeout unknown-cmd, got %d", code)
	}
}

func TestRunAuditWrapperTimeoutNoInner(t *testing.T) {
	enableAllBuiltins(t)
	code := run([]string{"--audit", "timeout", "5"})
	if code != 1 {
		t.Fatalf("expected exit 1 for timeout with no inner command, got %d", code)
	}
}

func TestRunAuditShellWrapperAllowed(t *testing.T) {
	enableAllBuiltins(t)
	code := run([]string{"--audit", "--sh", "timeout 5 git log | head"})
	if code != 0 {
		t.Fatalf("expected exit 0 for --sh timeout pipeline, got %d", code)
	}
}

func TestRunAuditShellWrapperBlocked(t *testing.T) {
	enableAllBuiltins(t)
	code := run([]string{"--audit", "--sh", "timeout 5 git push"})
	if code != 1 {
		t.Fatalf("expected exit 1 for --sh timeout git push, got %d", code)
	}
}

func TestRunClaude_WrapperTimeoutAllowed(t *testing.T) {
	enableAllBuiltins(t)
	var code int
	out := captureStdout(func() {
		code = run([]string{"--claude", "timeout", "5", "git", "log"})
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, `"permissionDecision":"allow"`) {
		t.Fatalf("expected allow JSON, got %q", out)
	}
}

func TestRunClaude_WrapperTimeoutBlocked(t *testing.T) {
	enableAllBuiltins(t)
	var code int
	out := captureStdout(func() {
		code = run([]string{"--claude", "timeout", "5", "git", "push"})
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if strings.TrimSpace(out) != "" {
		t.Fatalf("expected empty stdout for blocked wrapper, got %q", out)
	}
}

func TestRunClaude_ShellWrapperAllowed(t *testing.T) {
	enableAllBuiltins(t)
	var code int
	out := captureStdout(func() {
		code = run([]string{"--claude", "timeout 5 git log | head"})
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, `"permissionDecision":"allow"`) {
		t.Fatalf("expected allow JSON, got %q", out)
	}
}

func TestRunClaude_ShellWrapperBlocked(t *testing.T) {
	enableAllBuiltins(t)
	var code int
	out := captureStdout(func() {
		code = run([]string{"--claude", "timeout 5 git push"})
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if strings.TrimSpace(out) != "" {
		t.Fatalf("expected empty stdout for blocked shell wrapper, got %q", out)
	}
}

func TestRunClaude_WrapperNestedAllowed(t *testing.T) {
	enableAllBuiltins(t)
	var code int
	out := captureStdout(func() {
		code = run([]string{"--claude", "nice", "-n", "10", "timeout", "5", "git", "log"})
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, `"permissionDecision":"allow"`) {
		t.Fatalf("expected allow JSON, got %q", out)
	}
}
