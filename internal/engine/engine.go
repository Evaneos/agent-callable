package engine

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/evaneos/agent-callable/internal/audit"
	"github.com/evaneos/agent-callable/internal/config"
	"github.com/evaneos/agent-callable/internal/execx"
	"github.com/evaneos/agent-callable/internal/shell"
	"github.com/evaneos/agent-callable/internal/spec"
	"github.com/evaneos/agent-callable/internal/tools/docker"
	"github.com/evaneos/agent-callable/internal/tools/gcloud"
	"github.com/evaneos/agent-callable/internal/tools/gh"
	"github.com/evaneos/agent-callable/internal/tools/git"
	"github.com/evaneos/agent-callable/internal/tools/kubectl"
	"github.com/evaneos/agent-callable/internal/tools/npm"
	"github.com/evaneos/agent-callable/internal/tools/pulumi"
)

type Engine struct {
	reg          *spec.Registry
	builtinSet   map[string]bool
	globalConfig *config.GlobalConfig
	audit        *audit.Logger
}

func NewDefault() *Engine {
	cfgs, gc, errs := config.LoadAll(shell.DefaultConfigs)
	for _, err := range errs {
		fmt.Fprintf(os.Stderr, "agent-callable: config warning: %v\n", err)
	}
	return New(gc, cfgs)
}

// New creates an Engine from an explicit config and tool list.
func New(gc *config.GlobalConfig, cfgs []config.ConfigTool) *Engine {
	r := spec.NewRegistry()

	// Go builtins (complex logic not expressible in TOML)
	r.Register(kubectl.New())
	r.Register(git.New())
	r.Register(gh.New())
	r.Register(docker.New(gc.WritableDirs))
	r.Register(npm.New())
	r.Register(pulumi.New())
	r.Register(gcloud.New())

	// Wrapper builtins (delegate to inner command validation)
	r.Register(spec.NewWrapper("timeout", spec.ExtractAfterFlagsAndN(1, map[string]bool{
		"-k": true, "--kill-after": true,
		"-s": true, "--signal": true,
	})))
	r.Register(spec.NewWrapper("nice", spec.ExtractAfterFlags(map[string]bool{
		"-n": true, "--adjustment": true,
	})))
	r.Register(spec.NewWrapper("xargs", spec.ExtractAfterFlags(map[string]bool{
		"-n": true, "--max-args": true,
		"-P": true, "--max-procs": true,
		"-s": true, "--max-chars": true,
		"-a": true, "--arg-file": true,
		"-I": true, "--replace": true,
		"-L": true, "--max-lines": true,
		"-d": true, "--delimiter": true,
		"--process-slot-var": true,
	})))

	builtins := make(map[string]bool)
	for _, name := range r.Names() {
		builtins[name] = true
	}

	// Config-driven tools (default TOML + user-defined, override builtin if same name)
	for _, cfg := range cfgs {
		name := strings.ToLower(cfg.ToolConfig.Name)
		if builtins[name] {
			r.Unregister(name)
		}
		r.Register(cfg.ToToolSpec(gc.WritableDirs))
	}

	// Deny-by-default: only builtins explicitly set to true are kept.
	for name := range builtins {
		if !gc.Builtins[name] {
			r.Unregister(name)
		}
	}

	auditLogger, err := audit.New(gc.Audit.File, gc.Audit.Mode, gc.Audit.MaxEntries, gc.Audit.MaskSecrets)
	if err != nil {
		fmt.Fprintf(os.Stderr, "agent-callable: audit warning: %v\n", err)
	}

	e := &Engine{reg: r, builtinSet: builtins, globalConfig: gc, audit: auditLogger}

	// Inject CheckFunc into wrapper tools.
	wrapperFn := e.checkInnerWithNonInteractiveArgs()
	for _, name := range r.Names() {
		t, ok := r.Get(name)
		if !ok {
			continue
		}
		if w, ok := t.(interface {
			SetCheckFunc(func(string, []string) ([]string, error))
		}); ok {
			w.SetCheckFunc(wrapperFn)
		}
	}

	return e
}

// Registry returns the tool registry.
func (e *Engine) Registry() *spec.Registry {
	return e.reg
}

// GlobalConfig returns the loaded global configuration.
func (e *Engine) GlobalConfig() *config.GlobalConfig {
	return e.globalConfig
}

// Audit returns the audit logger (may be nil if disabled).
func (e *Engine) Audit() *audit.Logger {
	return e.audit
}

// Close releases resources (audit log file, etc.).
func (e *Engine) Close() {
	e.audit.Close()
}

// ShellValidateOpts returns the standard options for shell.Validate,
// including writable dirs and the inner-command check function.
func (e *Engine) ShellValidateOpts() shell.ValidateOpts {
	return shell.ValidateOpts{
		WritableDirs: e.globalConfig.WritableDirs,
		CheckFunc:    e.CheckInnerFunc(),
	}
}

// checkInnerWithNonInteractiveArgs returns a function that validates a command and
// propagates NonInteractiveArgs from the inner tool. Used by wrapper tools.
func (e *Engine) checkInnerWithNonInteractiveArgs() func(name string, args []string) ([]string, error) {
	return func(name string, args []string) ([]string, error) {
		cr := e.Check(append([]string{name}, args...))
		if !cr.Allowed {
			return nil, fmt.Errorf("%s", cr.Reason)
		}
		// cr.Args = original args + NonInteractiveArgs (from engine.Check).
		// Extract the extra args by comparing lengths.
		if len(cr.Args) > len(args) {
			return cr.Args[len(args):], nil
		}
		return nil, nil
	}
}

// CheckInnerFunc returns a function that validates a command against the
// engine's policy. Used by --sh shell validation (no NonInteractiveArgs needed).
func (e *Engine) CheckInnerFunc() func(name string, args []string) error {
	inner := e.checkInnerWithNonInteractiveArgs()
	return func(name string, args []string) error {
		_, err := inner(name, args)
		return err
	}
}

// buildEnv constructs a process environment by merging env maps onto os.Environ,
// then setting global non-interactive overrides (PAGER, CI).
func (e *Engine) buildEnv(envMaps ...map[string]string) []string {
	env := os.Environ()
	for _, m := range envMaps {
		for k, v := range m {
			env = upsertEnv(env, k, v)
		}
	}
	env = upsertEnv(env, "PAGER", "cat")
	env = upsertEnv(env, "CI", "1")
	return env
}

// ShellEnv builds the environment for shell execution, merging
// non-interactive env from all tools referenced in the expression.
func (e *Engine) ShellEnv(toolNames []string) []string {
	var maps []map[string]string
	for _, name := range toolNames {
		t, ok := e.reg.Get(name)
		if !ok {
			continue
		}
		if m := t.NonInteractiveEnv(); m != nil {
			maps = append(maps, m)
		}
	}
	return e.buildEnv(maps...)
}

// CheckResult holds the outcome of a policy check.
type CheckResult struct {
	Allowed  bool
	Tool     string
	Args     []string
	Reason   string // empty if allowed
	ExitCode int    // suggested exit code when blocked (0 if allowed)
}

// Check validates a command against the policy without executing it.
func (e *Engine) Check(args []string) CheckResult {
	if len(args) == 0 {
		return CheckResult{ExitCode: 2, Reason: "agent-callable: usage: agent-callable <command> [args...]"}
	}

	tool := strings.TrimSpace(args[0])
	if tool == "" {
		return CheckResult{ExitCode: 2, Reason: "agent-callable: empty command"}
	}
	if tool == "agent-callable" {
		return CheckResult{Tool: tool, ExitCode: 126, Reason: fmt.Sprintf("agent-callable: command %q not supported via agent-callable", tool)}
	}

	toolArgs := args[1:]
	// Control-character check is centralized here. Individual tool Check()
	// methods can assume args are free of control characters.
	if spec.ContainsControlCharacters(args) {
		return CheckResult{Tool: tool, Args: toolArgs, ExitCode: 2, Reason: "agent-callable: invalid arguments (control characters)"}
	}

	t, ok := e.reg.Get(tool)
	if !ok {
		allowed := e.reg.Names()
		slices.Sort(allowed)
		return CheckResult{
			Tool:     tool,
			Args:     toolArgs,
			ExitCode: 1,
			Reason:   fmt.Sprintf("agent-callable: command blocked (%s not supported). Use %s directly.\nSupported tools: %s", tool, tool, strings.Join(allowed, ", ")),
		}
	}

	res := t.Check(toolArgs, spec.RuntimeCtx{})
	if res.Decision != spec.DecisionAllow {
		reason := strings.TrimSpace(res.Reason)
		if reason == "" {
			reason = "not allowed"
		}
		fullCmd := tool + " " + strings.Join(toolArgs, " ")
		return CheckResult{
			Tool:     tool,
			Args:     toolArgs,
			ExitCode: 1,
			Reason:   fmt.Sprintf("agent-callable: command blocked. Use %s directly.\nReason: %s\nCommand: %s", tool, reason, fullCmd),
		}
	}

	return CheckResult{Allowed: true, Tool: tool, Args: append(toolArgs, res.NonInteractiveArgs...)}
}

func (e *Engine) Run(args []string) (int, error) {
	cr := e.Check(args)
	if !cr.Allowed {
		if cr.Tool != "" {
			e.audit.Log("BLOCKED", cr.Tool, cr.Args)
		}
		return cr.ExitCode, fmt.Errorf("%s", cr.Reason)
	}

	e.audit.Log("ALLOWED", cr.Tool, cr.Args)

	return execx.Exec(execx.ExecPlan{
		Tool: cr.Tool,
		Args: cr.Args,
		Env:  e.buildEnv(e.toolEnv(cr.Tool)),
	})
}

// toolEnv returns the non-interactive env map for a tool, or nil.
func (e *Engine) toolEnv(name string) map[string]string {
	t, ok := e.reg.Get(name)
	if !ok {
		return nil
	}
	return t.NonInteractiveEnv()
}

// ToolEntry represents a registered tool with its source.
type ToolEntry struct {
	Name   string
	Source string // "built-in" or "config"
}

// ListTools returns all registered tools sorted by name.
func (e *Engine) ListTools() []ToolEntry {
	names := e.reg.Names()
	slices.Sort(names)
	entries := make([]ToolEntry, len(names))
	for i, name := range names {
		source := "config"
		if e.builtinSet[name] {
			source = "built-in"
		}
		entries[i] = ToolEntry{Name: name, Source: source}
	}
	return entries
}

func upsertEnv(env []string, key, value string) []string {
	for i, kv := range env {
		if k, _, ok := strings.Cut(kv, "="); ok && k == key {
			env[i] = key + "=" + value
			return env
		}
	}
	return append(env, key+"="+value)
}
