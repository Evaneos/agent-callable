package execx

import (
	"strings"
	"testing"
)

func TestExecShellEmptyExpression(t *testing.T) {
	code, err := ExecShell(ShellPlan{})
	if code != 2 {
		t.Errorf("expected exit code 2, got %d", code)
	}
	if err == nil || !strings.Contains(err.Error(), "empty shell expression") {
		t.Errorf("expected error containing \"empty shell expression\", got %v", err)
	}
}
