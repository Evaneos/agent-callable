package spec

// ExtendedToolSpec wraps a built-in tool with a TOML fallback.
// The built-in Check runs first. If it allows, done.
// If it denies, the TOML fallback is consulted.
// This lets users extend a built-in's allowlist via config
// without losing its custom validation logic.
type ExtendedToolSpec struct {
	builtin  ToolSpec
	fallback ToolSpec
}

func NewExtendedTool(builtin, fallback ToolSpec) *ExtendedToolSpec {
	return &ExtendedToolSpec{builtin: builtin, fallback: fallback}
}

func (e *ExtendedToolSpec) Name() string { return e.builtin.Name() }

func (e *ExtendedToolSpec) NonInteractiveEnv() map[string]string {
	return e.builtin.NonInteractiveEnv()
}

func (e *ExtendedToolSpec) Check(args []string, rt RuntimeCtx) Result {
	res := e.builtin.Check(args, rt)
	if res.Decision == DecisionAllow {
		return res
	}
	fallback := e.fallback.Check(args, rt)
	if fallback.Decision == DecisionAllow {
		return fallback
	}
	return res
}
