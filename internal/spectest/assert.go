package spectest

import (
	"slices"
	"strings"
	"testing"

	"github.com/evaneos/agent-callable/internal/spec"
)

// AssertAllowed checks that tool.Check(args) returns Allow.
func AssertAllowed(t *testing.T, tool spec.ToolSpec, args []string) {
	t.Helper()
	res := tool.Check(args, spec.RuntimeCtx{})
	if res.Decision != spec.DecisionAllow {
		t.Errorf("expected allowed for %v, got deny: %s", args, res.Reason)
	}
}

// AssertBlocked checks that tool.Check(args) returns Deny.
func AssertBlocked(t *testing.T, tool spec.ToolSpec, args []string) {
	t.Helper()
	res := tool.Check(args, spec.RuntimeCtx{})
	if res.Decision == spec.DecisionAllow {
		t.Errorf("expected blocked for %v", args)
	}
}

// AssertAllowedBatch runs AssertAllowed for each case as a named sub-test.
func AssertAllowedBatch(t *testing.T, tool spec.ToolSpec, cases [][]string) {
	t.Helper()
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			t.Helper()
			AssertAllowed(t, tool, args)
		})
	}
}

// AssertBlockedBatch runs AssertBlocked for each case as a named sub-test.
func AssertBlockedBatch(t *testing.T, tool spec.ToolSpec, cases [][]string) {
	t.Helper()
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			t.Helper()
			AssertBlocked(t, tool, args)
		})
	}
}

// AssertNoNonInteractiveArgs checks that tool.Check(args) returns Allow with no NonInteractiveArgs.
func AssertNoNonInteractiveArgs(t *testing.T, tool spec.ToolSpec, args []string) {
	t.Helper()
	res := tool.Check(args, spec.RuntimeCtx{})
	if res.Decision != spec.DecisionAllow {
		t.Errorf("expected allowed for %v, got deny: %s", args, res.Reason)
	}
	if len(res.NonInteractiveArgs) != 0 {
		t.Errorf("expected no NonInteractiveArgs for %v, got %v", args, res.NonInteractiveArgs)
	}
}

// AssertNoNonInteractiveArgsBatch runs AssertNoNonInteractiveArgs for each case as a named sub-test.
func AssertNoNonInteractiveArgsBatch(t *testing.T, tool spec.ToolSpec, cases [][]string) {
	t.Helper()
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			t.Helper()
			AssertNoNonInteractiveArgs(t, tool, args)
		})
	}
}

// AssertNonInteractiveArgs checks that tool.Check(args) returns Allow with the expected NonInteractiveArgs.
func AssertNonInteractiveArgs(t *testing.T, tool spec.ToolSpec, args []string, wantExtra []string) {
	t.Helper()
	res := tool.Check(args, spec.RuntimeCtx{})
	if res.Decision != spec.DecisionAllow {
		t.Errorf("expected allowed for %v, got deny: %s", args, res.Reason)
		return
	}
	if !slices.Equal(res.NonInteractiveArgs, wantExtra) {
		t.Errorf("NonInteractiveArgs for %v: got %v, want %v", args, res.NonInteractiveArgs, wantExtra)
	}
}
