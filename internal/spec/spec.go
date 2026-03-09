package spec

type Decision int

const (
	DecisionDeny Decision = iota
	DecisionAllow
)

type Result struct {
	Decision           Decision
	Reason             string   // non-empty if Deny
	NonInteractiveArgs []string // CLI flags injected at execution (e.g. --non-interactive)
}

type RuntimeCtx struct {
	// Reserved for later (cwd, filtered env, etc.).
}

type ToolSpec interface {
	// Name returns the real command name: "kubectl", "git", "cat", ...
	Name() string
	// Check decides if execution is allowed (deny-by-default).
	Check(args []string, rt RuntimeCtx) Result
	// NonInteractiveEnv returns env overrides that disable prompts.
	// (e.g. GIT_TERMINAL_PROMPT=0). Applied only if the command is allowed.
	NonInteractiveEnv() map[string]string
}
