package config

import (
	"testing"

	"github.com/evaneos/agent-callable/internal/spec"
)

func TestToToolSpecAllowAll(t *testing.T) {
	ct := ConfigTool{ToolConfig: ToolConfig{Name: "mycli"}, AllowAll: true}
	tool := ct.ToToolSpec(nil)
	res := tool.Check([]string{"anything"}, spec.RuntimeCtx{})
	if res.Decision != spec.DecisionAllow {
		t.Errorf("expected allowed, got deny: %s", res.Reason)
	}
}

func TestToToolSpecFlat(t *testing.T) {
	ct := ConfigTool{ToolConfig: ToolConfig{
		Name:    "mytool",
		Allowed: []string{"list"},
	}}
	tool := ct.ToToolSpec(nil)

	res := tool.Check([]string{"list"}, spec.RuntimeCtx{})
	if res.Decision != spec.DecisionAllow {
		t.Errorf("expected allowed for list, got deny: %s", res.Reason)
	}

	res = tool.Check([]string{"delete"}, spec.RuntimeCtx{})
	if res.Decision == spec.DecisionAllow {
		t.Error("expected blocked for delete")
	}
}

func TestToToolSpecEnv(t *testing.T) {
	ct := ConfigTool{ToolConfig: ToolConfig{
		Name:    "mytool",
		Allowed: []string{"list"},
		Env:     map[string]string{"FOO": "bar"},
	}}
	tool := ct.ToToolSpec(nil)
	env := tool.NonInteractiveEnv()
	if env["FOO"] != "bar" {
		t.Errorf("expected FOO=bar, got %v", env)
	}
}
