package execx

import (
	"strings"
	"testing"
)

func TestExecMissingTool(t *testing.T) {
	code, err := Exec(ExecPlan{})
	if code != 2 {
		t.Errorf("expected exit code 2, got %d", code)
	}
	if err == nil || !strings.Contains(err.Error(), "missing tool") {
		t.Errorf("expected error containing \"missing tool\", got %v", err)
	}
}

func TestExecToolNotFound(t *testing.T) {
	code, err := Exec(ExecPlan{Tool: "nonexistent-binary-xyz"})
	if code != 127 {
		t.Errorf("expected exit code 127, got %d", code)
	}
	if err == nil || !strings.Contains(err.Error(), "not found in PATH") {
		t.Errorf("expected error containing \"not found in PATH\", got %v", err)
	}
}

func TestExecPlanFields(t *testing.T) {
	plan := ExecPlan{
		Tool: "kubectl",
		Args: []string{"get", "pods"},
		Env:  []string{"FOO=bar"},
	}
	if plan.Tool != "kubectl" {
		t.Errorf("unexpected Tool: %s", plan.Tool)
	}
	if len(plan.Args) != 2 || plan.Args[0] != "get" || plan.Args[1] != "pods" {
		t.Errorf("unexpected Args: %v", plan.Args)
	}
	if len(plan.Env) != 1 || plan.Env[0] != "FOO=bar" {
		t.Errorf("unexpected Env: %v", plan.Env)
	}
}
