package spec

import "testing"

// stubTool is a minimal ToolSpec for testing.
type stubTool struct {
	name    string
	checkFn func([]string, RuntimeCtx) Result
}

func (s *stubTool) Name() string                          { return s.name }
func (s *stubTool) Check(a []string, r RuntimeCtx) Result { return s.checkFn(a, r) }
func (s *stubTool) NonInteractiveEnv() map[string]string  { return map[string]string{"FOO": "bar"} }

func TestExtendedAllowedByBuiltin(t *testing.T) {
	builtin := &stubTool{name: "gh", checkFn: func([]string, RuntimeCtx) Result {
		return Allow()
	}}
	fallback := NewConfigTool(ConfigToolOpts{
		Name: "gh", Allowed: []string{"pr"},
		Subcommands: map[string][]string{"pr": {"create"}},
	})
	ext := NewExtendedTool(builtin, fallback)

	res := ext.Check([]string{"anything"}, RuntimeCtx{})
	if res.Decision != DecisionAllow {
		t.Errorf("expected allowed by builtin, got deny: %s", res.Reason)
	}
}

func TestExtendedDeniedByBuiltinAllowedByFallback(t *testing.T) {
	builtin := &stubTool{name: "gh", checkFn: func([]string, RuntimeCtx) Result {
		return Deny("blocked by builtin")
	}}
	fallback := NewConfigTool(ConfigToolOpts{
		Name: "gh", Allowed: []string{"pr"},
		Subcommands: map[string][]string{"pr": {"create"}},
	})
	ext := NewExtendedTool(builtin, fallback)

	res := ext.Check([]string{"pr", "create"}, RuntimeCtx{})
	if res.Decision != DecisionAllow {
		t.Errorf("expected allowed by fallback, got deny: %s", res.Reason)
	}
}

func TestExtendedDeniedByBoth(t *testing.T) {
	builtin := &stubTool{name: "gh", checkFn: func([]string, RuntimeCtx) Result {
		return Deny("blocked by builtin")
	}}
	fallback := NewConfigTool(ConfigToolOpts{
		Name: "gh", Allowed: []string{"pr"},
		Subcommands: map[string][]string{"pr": {"create"}},
	})
	ext := NewExtendedTool(builtin, fallback)

	res := ext.Check([]string{"pr", "merge", "123"}, RuntimeCtx{})
	if res.Decision != DecisionDeny {
		t.Error("expected denied by both")
	}
	if res.Reason != "blocked by builtin" {
		t.Errorf("expected builtin reason, got %q", res.Reason)
	}
}

func TestExtendedName(t *testing.T) {
	builtin := &stubTool{name: "gh", checkFn: func([]string, RuntimeCtx) Result { return Allow() }}
	fallback := NewConfigTool(ConfigToolOpts{Name: "gh", Allowed: []string{"*"}, AllowAll: true})
	ext := NewExtendedTool(builtin, fallback)

	if ext.Name() != "gh" {
		t.Errorf("expected name=gh, got %q", ext.Name())
	}
}

func TestExtendedNonInteractiveEnv(t *testing.T) {
	builtin := &stubTool{name: "gh", checkFn: func([]string, RuntimeCtx) Result { return Allow() }}
	fallback := NewConfigTool(ConfigToolOpts{Name: "gh", Allowed: []string{"*"}, AllowAll: true})
	ext := NewExtendedTool(builtin, fallback)

	env := ext.NonInteractiveEnv()
	if env["FOO"] != "bar" {
		t.Errorf("expected builtin env, got %v", env)
	}
}
