package engine

import (
	"slices"
	"strings"
	"testing"

	"github.com/evaneos/agent-callable/internal/config"
	"github.com/evaneos/agent-callable/internal/shell"
)

func TestUnknownToolBlocked(t *testing.T) {
	e := newTestEngine()
	code, err := e.Run([]string{"does-not-exist", "arg"})
	if code == 0 || err == nil {
		t.Fatalf("expected blocked for unknown tool")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckAllowed(t *testing.T) {
	e := newTestEngine()
	cr := e.Check([]string{"git", "status"})
	if !cr.Allowed {
		t.Fatalf("expected allowed, got blocked: %s", cr.Reason)
	}
	if cr.Tool != "git" {
		t.Fatalf("expected tool=git, got %q", cr.Tool)
	}
}

func TestCheckBlocked(t *testing.T) {
	e := newTestEngine()
	cr := e.Check([]string{"kubectl", "delete", "pod", "x"})
	if cr.Allowed {
		t.Fatal("expected blocked for kubectl delete")
	}
	if cr.Reason == "" {
		t.Fatal("expected non-empty reason")
	}
}

func TestCheckUnknownTool(t *testing.T) {
	e := newTestEngine()
	cr := e.Check([]string{"unknown-tool"})
	if cr.Allowed {
		t.Fatal("expected blocked for unknown tool")
	}
	if cr.Reason == "" {
		t.Fatal("expected non-empty reason")
	}
}

func TestCheckEmptyArgs(t *testing.T) {
	e := newTestEngine()
	cr := e.Check([]string{})
	if cr.Allowed {
		t.Fatal("expected blocked for empty args")
	}
	if cr.ExitCode != 2 {
		t.Fatalf("expected ExitCode 2, got %d", cr.ExitCode)
	}
	if !strings.Contains(cr.Reason, "usage:") {
		t.Fatalf("expected reason to contain \"usage:\", got %q", cr.Reason)
	}
}

func TestCheckEmptyCommand(t *testing.T) {
	e := newTestEngine()
	cr := e.Check([]string{""})
	if cr.Allowed {
		t.Fatal("expected blocked for empty command")
	}
	if cr.ExitCode != 2 {
		t.Fatalf("expected ExitCode 2, got %d", cr.ExitCode)
	}
	if !strings.Contains(cr.Reason, "empty command") {
		t.Fatalf("expected reason to contain \"empty command\", got %q", cr.Reason)
	}
}

func TestCheckRecursionDetection(t *testing.T) {
	e := newTestEngine()
	cr := e.Check([]string{"agent-callable"})
	if cr.Allowed {
		t.Fatal("expected blocked for agent-callable recursion")
	}
	if cr.ExitCode != 126 {
		t.Fatalf("expected ExitCode 126, got %d", cr.ExitCode)
	}
	if !strings.Contains(cr.Reason, "not supported via agent-callable") {
		t.Fatalf("expected reason to contain \"not supported via agent-callable\", got %q", cr.Reason)
	}
}

func TestCheckControlCharacters(t *testing.T) {
	e := newTestEngine()
	cr := e.Check([]string{"git", "status\x00"})
	if cr.Allowed {
		t.Fatal("expected blocked for control characters")
	}
	if cr.ExitCode != 2 {
		t.Fatalf("expected ExitCode 2, got %d", cr.ExitCode)
	}
	if !strings.Contains(cr.Reason, "control characters") {
		t.Fatalf("expected reason to contain \"control characters\", got %q", cr.Reason)
	}
}

func TestCheckToolNotInRegistryListsTools(t *testing.T) {
	e := newTestEngine()
	cr := e.Check([]string{"unknown-tool"})
	if cr.Allowed {
		t.Fatal("expected blocked for unknown tool")
	}
	if !strings.Contains(cr.Reason, "Supported tools:") {
		t.Fatalf("expected reason to contain \"Supported tools:\", got %q", cr.Reason)
	}
	for _, entry := range e.ListTools() {
		if !strings.Contains(cr.Reason, entry.Name) {
			t.Fatalf("expected reason to list tool %q, got %q", entry.Name, cr.Reason)
		}
	}
}

func TestKnownToolButBlockedCommand(t *testing.T) {
	e := newTestEngine()
	code, err := e.Run([]string{"kubectl", "delete", "pod", "x"})
	if code == 0 || err == nil {
		t.Fatalf("expected blocked kubectl delete")
	}
	msg := err.Error()
	if !strings.Contains(msg, "command blocked") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(msg, "Command: kubectl delete pod x") {
		t.Fatalf("expected full command in error message, got: %v", err)
	}
}

// --- Builtin deny-by-default tests ---

var allBuiltinNames = []string{"kubectl", "git", "gh", "docker", "npm", "pulumi", "gcloud", "timeout", "nice"}

func allBuiltinsEnabled() map[string]bool {
	m := make(map[string]bool)
	for _, name := range allBuiltinNames {
		m[name] = true
	}
	return m
}

func newTestEngine() *Engine {
	return New(&config.GlobalConfig{Builtins: allBuiltinsEnabled()}, nil)
}

func TestBuiltinsAllEnabledExplicitly(t *testing.T) {
	builtins := make(map[string]bool)
	for _, name := range allBuiltinNames {
		builtins[name] = true
	}
	gc := &config.GlobalConfig{Builtins: builtins}
	e := New(gc, nil)

	for _, name := range allBuiltinNames {
		if _, ok := e.reg.Get(name); !ok {
			t.Errorf("expected builtin %s to be registered when explicitly true", name)
		}
	}
}

func TestBuiltinsAllDisabledExplicitly(t *testing.T) {
	builtins := make(map[string]bool)
	for _, name := range allBuiltinNames {
		builtins[name] = false
	}
	gc := &config.GlobalConfig{Builtins: builtins}
	e := New(gc, nil)

	for _, name := range allBuiltinNames {
		if _, ok := e.reg.Get(name); ok {
			t.Errorf("expected builtin %s to be unregistered when explicitly false", name)
		}
	}
}

func TestBuiltinsDenyByDefaultWhenSectionExists(t *testing.T) {
	// [builtins] section exists but only git = true.
	gc := &config.GlobalConfig{Builtins: map[string]bool{"git": true}}
	e := New(gc, nil)

	if _, ok := e.reg.Get("git"); !ok {
		t.Error("expected git to be registered (explicitly true)")
	}
	for _, name := range allBuiltinNames {
		if name == "git" {
			continue
		}
		if _, ok := e.reg.Get(name); ok {
			t.Errorf("expected builtin %s to be unregistered (absent from [builtins] = denied)", name)
		}
	}
}

func TestBuiltinsNilSectionDeniesAll(t *testing.T) {
	// No [builtins] section → all builtins denied (deny-by-default).
	gc := &config.GlobalConfig{Builtins: nil}
	e := New(gc, nil)

	for _, name := range allBuiltinNames {
		if _, ok := e.reg.Get(name); ok {
			t.Errorf("expected builtin %s to be unregistered when [builtins] section absent", name)
		}
	}
}

func TestBuiltinsEmptyMapDisablesAll(t *testing.T) {
	// [builtins] section exists but empty → all denied.
	gc := &config.GlobalConfig{Builtins: map[string]bool{}}
	e := New(gc, nil)

	for _, name := range allBuiltinNames {
		if _, ok := e.reg.Get(name); ok {
			t.Errorf("expected builtin %s to be unregistered (empty [builtins] section)", name)
		}
	}
}

func TestCheckInnerFunc(t *testing.T) {
	e := newTestEngine()
	checkFn := e.CheckInnerFunc()

	// Allowed inner command
	if err := checkFn("git", []string{"status"}); err != nil {
		t.Errorf("expected allowed for git status, got: %v", err)
	}

	// Blocked inner command
	if err := checkFn("git", []string{"push"}); err == nil {
		t.Error("expected error for git push")
	}

	// Unknown tool
	if err := checkFn("unknown-tool", []string{}); err == nil {
		t.Error("expected error for unknown tool")
	}
}

func TestWrapperTimeoutAllowed(t *testing.T) {
	e := newTestEngine()
	cr := e.Check([]string{"timeout", "5", "git", "log"})
	if !cr.Allowed {
		t.Fatalf("expected allowed, got blocked: %s", cr.Reason)
	}
}

func TestWrapperTimeoutDenied(t *testing.T) {
	e := newTestEngine()
	cr := e.Check([]string{"timeout", "5", "git", "push"})
	if cr.Allowed {
		t.Fatal("expected blocked for timeout wrapping git push")
	}
}

func TestWrapperTimeoutUnknownInner(t *testing.T) {
	e := newTestEngine()
	cr := e.Check([]string{"timeout", "5", "unknown-cmd"})
	if cr.Allowed {
		t.Fatal("expected blocked for timeout wrapping unknown command")
	}
}

func TestWrapperTimeoutNoInner(t *testing.T) {
	e := newTestEngine()
	cr := e.Check([]string{"timeout", "5"})
	if cr.Allowed {
		t.Fatal("expected blocked for timeout with no inner command")
	}
}

func TestWrapperTimeoutInnerFlags(t *testing.T) {
	e := newTestEngine()
	cr := e.Check([]string{"timeout", "5", "git", "log", "--oneline", "-n", "5"})
	if !cr.Allowed {
		t.Fatalf("expected allowed, got blocked: %s", cr.Reason)
	}
}

func TestWrapperTimeoutWithOwnFlags(t *testing.T) {
	e := newTestEngine()
	cr := e.Check([]string{"timeout", "-k", "3", "-s", "KILL", "5", "git", "status"})
	if !cr.Allowed {
		t.Fatalf("expected allowed, got blocked: %s", cr.Reason)
	}
}

func TestWrapperNiceAllowed(t *testing.T) {
	e := newTestEngine()
	cr := e.Check([]string{"nice", "-n", "10", "git", "status"})
	if !cr.Allowed {
		t.Fatalf("expected allowed, got blocked: %s", cr.Reason)
	}
}

func TestWrapperNiceDenied(t *testing.T) {
	e := newTestEngine()
	cr := e.Check([]string{"nice", "-n", "10", "git", "push"})
	if cr.Allowed {
		t.Fatal("expected blocked for nice wrapping git push")
	}
}

func TestWrapperRecursive(t *testing.T) {
	e := newTestEngine()
	cr := e.Check([]string{"timeout", "5", "timeout", "3", "git", "log"})
	if !cr.Allowed {
		t.Fatalf("expected allowed for nested wrappers, got blocked: %s", cr.Reason)
	}
}

func TestWrapperRecursiveDenied(t *testing.T) {
	e := newTestEngine()
	cr := e.Check([]string{"timeout", "5", "nice", "git", "push"})
	if cr.Allowed {
		t.Fatal("expected blocked for nested wrappers with denied inner")
	}
}

func TestWrapperTimeoutSubcommands(t *testing.T) {
	e := newTestEngine()
	cr := e.Check([]string{"timeout", "5", "kubectl", "get", "pods"})
	if !cr.Allowed {
		t.Fatalf("expected allowed, got blocked: %s", cr.Reason)
	}
	cr = e.Check([]string{"timeout", "5", "kubectl", "delete", "pod", "foo"})
	if cr.Allowed {
		t.Fatal("expected blocked for timeout wrapping kubectl delete")
	}
}

func TestWrapperTimeoutConfigTool(t *testing.T) {
	echoCfg := config.ConfigTool{
		ToolConfig: config.ToolConfig{
			Name:    "echo",
			Allowed: []string{"*"},
		},
		AllowAll: true,
	}
	e := New(&config.GlobalConfig{Builtins: allBuiltinsEnabled()}, []config.ConfigTool{echoCfg})
	cr := e.Check([]string{"timeout", "5", "echo", "hello"})
	if !cr.Allowed {
		t.Fatalf("expected allowed for timeout wrapping config tool, got blocked: %s", cr.Reason)
	}
}

func newTestEngineWithWritableDirs(dirs []string) *Engine {
	cpCfg := config.ConfigTool{
		ToolConfig: config.ToolConfig{
			Name:        "cp",
			Allowed:     []string{"*"},
			WriteTarget: "last",
		},
		AllowAll: true,
	}
	gc := &config.GlobalConfig{
		Builtins:     allBuiltinsEnabled(),
		WritableDirs: dirs,
	}
	return New(gc, []config.ConfigTool{cpCfg})
}

func TestWrapperTimeoutWriteTargetAllowed(t *testing.T) {
	e := newTestEngineWithWritableDirs([]string{"/tmp"})
	cr := e.Check([]string{"timeout", "5", "cp", "src", "/tmp/dst"})
	if !cr.Allowed {
		t.Fatalf("expected allowed, got blocked: %s", cr.Reason)
	}
}

func TestWrapperTimeoutWriteTargetDenied(t *testing.T) {
	e := newTestEngineWithWritableDirs([]string{"/tmp"})
	cr := e.Check([]string{"timeout", "5", "cp", "src", "/etc/dst"})
	if cr.Allowed {
		t.Fatal("expected blocked for timeout wrapping cp to non-writable dir")
	}
}

func TestCheckNonInteractiveArgsInjected(t *testing.T) {
	e := newTestEngine()
	// pulumi preview without --non-interactive should be allowed
	// and have --non-interactive appended to Args.
	cr := e.Check([]string{"pulumi", "preview", "--diff"})
	if !cr.Allowed {
		t.Fatalf("expected allowed, got blocked: %s", cr.Reason)
	}
	if !slices.Contains(cr.Args, "--non-interactive") {
		t.Errorf("expected --non-interactive in Args, got %v", cr.Args)
	}
}

func TestCheckNonInteractiveArgsNotDuplicated(t *testing.T) {
	e := newTestEngine()
	// pulumi preview with --non-interactive already present: no duplication.
	cr := e.Check([]string{"pulumi", "preview", "--non-interactive", "--diff"})
	if !cr.Allowed {
		t.Fatalf("expected allowed, got blocked: %s", cr.Reason)
	}
	count := 0
	for _, a := range cr.Args {
		if a == "--non-interactive" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 --non-interactive, got %d in %v", count, cr.Args)
	}
}

func TestCheckNpmInstallBlockedWithoutIgnoreScripts(t *testing.T) {
	e := newTestEngine()
	cr := e.Check([]string{"npm", "install"})
	if cr.Allowed {
		t.Fatal("expected blocked for npm install without --ignore-scripts")
	}
}

func TestCheckNpmInstallAllowedWithIgnoreScripts(t *testing.T) {
	e := newTestEngine()
	cr := e.Check([]string{"npm", "install", "--ignore-scripts"})
	if !cr.Allowed {
		t.Fatalf("expected allowed, got blocked: %s", cr.Reason)
	}
}

func TestCheckNoNonInteractiveArgsForReadOnly(t *testing.T) {
	e := newTestEngine()
	// Read-only commands should not have extra args appended.
	cases := []struct {
		args []string
	}{
		{[]string{"git", "status"}},
		{[]string{"npm", "ls"}},
		{[]string{"pulumi", "version"}},
		{[]string{"npm", "run", "test"}},
	}
	for _, tc := range cases {
		cr := e.Check(tc.args)
		if !cr.Allowed {
			t.Fatalf("expected allowed for %v, got blocked: %s", tc.args, cr.Reason)
		}
		// Original args minus tool name should match cr.Args exactly.
		originalArgs := tc.args[1:]
		if len(cr.Args) != len(originalArgs) {
			t.Errorf("expected %d args for %v, got %d: %v", len(originalArgs), tc.args, len(cr.Args), cr.Args)
		}
	}
}

func TestWrapperTimeoutNonInteractiveArgsPropagated(t *testing.T) {
	e := newTestEngine()
	// timeout wrapping pulumi preview: NonInteractiveArgs should propagate through.
	cr := e.Check([]string{"timeout", "30", "pulumi", "preview", "--diff"})
	if !cr.Allowed {
		t.Fatalf("expected allowed, got blocked: %s", cr.Reason)
	}
	// --non-interactive must be present in Args (propagated from pulumi's NonInteractiveArgs).
	if !slices.Contains(cr.Args, "--non-interactive") {
		t.Errorf("expected --non-interactive in Args, got %v", cr.Args)
	}
}

func TestWrapperTimeoutNpmInstallBlocked(t *testing.T) {
	e := newTestEngine()
	// timeout wrapping npm install without --ignore-scripts: blocked.
	cr := e.Check([]string{"timeout", "120", "npm", "install"})
	if cr.Allowed {
		t.Fatal("expected blocked for timeout wrapping npm install without --ignore-scripts")
	}
}

func TestWrapperTimeoutNpmInstallAllowed(t *testing.T) {
	e := newTestEngine()
	cr := e.Check([]string{"timeout", "120", "npm", "install", "--ignore-scripts"})
	if !cr.Allowed {
		t.Fatalf("expected allowed, got blocked: %s", cr.Reason)
	}
}

func TestWrapperBuiltinsInAllBuiltins(t *testing.T) {
	all := shell.AllBuiltins()
	for _, name := range []string{"timeout", "nice"} {
		if !slices.Contains(all, name) {
			t.Errorf("expected %q in AllBuiltins(), got %v", name, all)
		}
	}
}

func TestConfigOverridesBuiltin(t *testing.T) {
	// A TOML config for "npm" should override the Go builtin.
	// The builtin blocks "npm install" without --ignore-scripts,
	// but our TOML override allows all subcommands.
	npmCfg := config.ConfigTool{
		ToolConfig: config.ToolConfig{
			Name:    "npm",
			Allowed: []string{"*"},
		},
		AllowAll: true,
	}
	gc := &config.GlobalConfig{Builtins: allBuiltinsEnabled()}
	e := New(gc, []config.ConfigTool{npmCfg})

	// With the builtin, this would be blocked. With TOML override, it's allowed.
	cr := e.Check([]string{"npm", "install"})
	if !cr.Allowed {
		t.Fatalf("expected TOML override to allow npm install, got blocked: %s", cr.Reason)
	}
}

func TestConfigOverrideStillRespectsBuiiltinToggle(t *testing.T) {
	// Even with a TOML override, disabling the builtin in [builtins] should unregister it.
	npmCfg := config.ConfigTool{
		ToolConfig: config.ToolConfig{
			Name:    "npm",
			Allowed: []string{"*"},
		},
		AllowAll: true,
	}
	gc := &config.GlobalConfig{Builtins: map[string]bool{"git": true}}
	e := New(gc, []config.ConfigTool{npmCfg})

	if _, ok := e.reg.Get("npm"); ok {
		t.Fatal("expected npm to be unregistered when not in [builtins]")
	}
}

// --- Extend mode tests ---

func TestExtendModeGHPrCreate(t *testing.T) {
	ghExtend := config.ConfigTool{
		ToolConfig: config.ToolConfig{
			Name:    "gh",
			Mode:    "extend",
			Allowed: []string{"pr"},
			Subcommands: map[string][]string{
				"pr": {"create"},
			},
		},
	}
	gc := &config.GlobalConfig{Builtins: allBuiltinsEnabled()}
	e := New(gc, []config.ConfigTool{ghExtend})

	// Extended subcommand allowed via fallback
	cr := e.Check([]string{"gh", "pr", "create"})
	if !cr.Allowed {
		t.Fatalf("expected gh pr create allowed after extend, got: %s", cr.Reason)
	}
	// Existing builtin allows still work
	cr = e.Check([]string{"gh", "pr", "list"})
	if !cr.Allowed {
		t.Fatalf("expected gh pr list still allowed, got: %s", cr.Reason)
	}
	// Blocked by both builtin and fallback
	cr = e.Check([]string{"gh", "pr", "merge", "123"})
	if cr.Allowed {
		t.Fatal("expected gh pr merge still blocked")
	}
	// Custom api logic preserved (builtin allows GET, denies POST)
	cr = e.Check([]string{"gh", "api", "/repos/X/Y"})
	if !cr.Allowed {
		t.Fatalf("expected gh api GET allowed, got: %s", cr.Reason)
	}
	cr = e.Check([]string{"gh", "api", "/repos/X/Y", "-XPOST"})
	if cr.Allowed {
		t.Fatal("expected gh api POST still blocked")
	}
}

func TestExtendModeGitPush(t *testing.T) {
	gitExtend := config.ConfigTool{
		ToolConfig: config.ToolConfig{
			Name:    "git",
			Mode:    "extend",
			Allowed: []string{"push"},
		},
	}
	gc := &config.GlobalConfig{Builtins: allBuiltinsEnabled()}
	e := New(gc, []config.ConfigTool{gitExtend})

	cr := e.Check([]string{"git", "push", "origin", "main"})
	if !cr.Allowed {
		t.Fatalf("expected git push allowed after extend, got: %s", cr.Reason)
	}
	cr = e.Check([]string{"git", "status"})
	if !cr.Allowed {
		t.Fatalf("expected git status still allowed, got: %s", cr.Reason)
	}
}

func TestExtendModeOnNonExistentToolIgnored(t *testing.T) {
	cfg := config.ConfigTool{
		ToolConfig: config.ToolConfig{
			Name:    "nonexistent",
			Mode:    "extend",
			Allowed: []string{"sub"},
		},
	}
	gc := &config.GlobalConfig{Builtins: allBuiltinsEnabled()}
	e := New(gc, []config.ConfigTool{cfg})

	cr := e.Check([]string{"nonexistent"})
	if cr.Allowed {
		t.Fatal("expected nonexistent tool still blocked")
	}
}

func TestExtendModePreservesListToolsSource(t *testing.T) {
	ghExtend := config.ConfigTool{
		ToolConfig: config.ToolConfig{
			Name:    "gh",
			Mode:    "extend",
			Allowed: []string{"pr"},
			Subcommands: map[string][]string{
				"pr": {"create"},
			},
		},
	}
	gc := &config.GlobalConfig{Builtins: allBuiltinsEnabled()}
	e := New(gc, []config.ConfigTool{ghExtend})

	for _, entry := range e.ListTools() {
		if entry.Name == "gh" {
			if entry.Source != "built-in" {
				t.Errorf("expected gh source=built-in after extend, got %q", entry.Source)
			}
			return
		}
	}
	t.Error("gh not found in ListTools")
}

// --- allow_on_any tests ---

func TestAllowOnAnyUnknownToolVersionAllowed(t *testing.T) {
	gc := &config.GlobalConfig{
		Builtins:   allBuiltinsEnabled(),
		AllowOnAny: []string{"--version", "-v", "--help", "-h"},
	}
	e := New(gc, nil)

	for _, args := range [][]string{
		{"claude", "--version"},
		{"claude", "-v"},
		{"some-unknown-tool", "--help"},
		{"some-unknown-tool", "-h"},
	} {
		cr := e.Check(args)
		if !cr.Allowed {
			t.Errorf("expected allowed for %v, got blocked: %s", args, cr.Reason)
		}
	}
}

func TestAllowOnAnyUnknownToolExtraArgBlocked(t *testing.T) {
	gc := &config.GlobalConfig{
		Builtins:   allBuiltinsEnabled(),
		AllowOnAny: []string{"--version", "--help"},
	}
	e := New(gc, nil)

	for _, args := range [][]string{
		{"claude", "--version", "extra"},
		{"claude", "subcommand"},
		{"claude", "--help", "subcommand"},
	} {
		cr := e.Check(args)
		if cr.Allowed {
			t.Errorf("expected blocked for %v (extra arg outside allow_on_any)", args)
		}
	}
}

func TestAllowOnAnyEmptyArgsBlocked(t *testing.T) {
	gc := &config.GlobalConfig{
		Builtins:   allBuiltinsEnabled(),
		AllowOnAny: []string{"--version", "--help"},
	}
	e := New(gc, nil)

	cr := e.Check([]string{"claude"})
	if cr.Allowed {
		t.Error("expected blocked for unknown tool with no args (allow_on_any requires at least one arg)")
	}
}

func TestAllowOnAnyEmptyListBlocked(t *testing.T) {
	gc := &config.GlobalConfig{
		Builtins:   allBuiltinsEnabled(),
		AllowOnAny: []string{},
	}
	e := New(gc, nil)

	cr := e.Check([]string{"claude", "--version"})
	if cr.Allowed {
		t.Error("expected blocked when allow_on_any is empty")
	}
}

func TestAllowOnAnyDoesNotShortCircuitWithNonListArgs(t *testing.T) {
	gc := &config.GlobalConfig{
		Builtins:   allBuiltinsEnabled(),
		AllowOnAny: []string{"--version", "delete"},
	}
	e := New(gc, nil)

	cr := e.Check([]string{"kubectl", "delete", "pod", "x"})
	if cr.Allowed {
		t.Error("expected kubectl delete pod x blocked (pod and x not in allow_on_any)")
	}
}

func TestAllowOnAnyRegisteredToolHelpAllowed(t *testing.T) {
	gc := &config.GlobalConfig{
		Builtins:   allBuiltinsEnabled(),
		AllowOnAny: []string{"--version", "--help"},
	}
	e := New(gc, nil)

	for _, args := range [][]string{
		{"git", "--help"},
		{"git", "--version"},
		{"kubectl", "--help"},
		{"kubectl", "--version"},
		{"docker", "--help"},
		{"gh", "--version"},
		{"npm", "--help"},
		{"pulumi", "--version"},
		{"gcloud", "--help"},
	} {
		cr := e.Check(args)
		if !cr.Allowed {
			t.Errorf("expected allowed for %v, got blocked: %s", args, cr.Reason)
		}
	}
}

func TestAllowOnAnyRegisteredToolMixedArgsBlocked(t *testing.T) {
	gc := &config.GlobalConfig{
		Builtins:   allBuiltinsEnabled(),
		AllowOnAny: []string{"--version", "--help"},
	}
	e := New(gc, nil)

	for _, args := range [][]string{
		{"kubectl", "delete", "--help"},
		{"kubectl", "--help", "pod"},
		{"docker", "rm", "--help"},
		{"npm", "install", "--version"},
	} {
		cr := e.Check(args)
		if cr.Allowed {
			t.Errorf("expected blocked for %v (mixed args with non-allow_on_any values)", args)
		}
	}
}

func TestAllowOnAnyRegisteredToolNoArgsNotShortCircuited(t *testing.T) {
	gc := &config.GlobalConfig{
		Builtins:   allBuiltinsEnabled(),
		AllowOnAny: []string{"--version", "--help"},
	}
	e := New(gc, nil)

	// Bare "git" has no args, so allow_on_any does not apply (requires >= 1 arg).
	// It falls through to the normal tool check, which blocks "git" (requires a subcommand).
	cr := e.Check([]string{"git"})
	if cr.Allowed {
		t.Error("expected git with no args blocked (git requires a subcommand)")
	}
}

func TestAllowOnAnyMultipleFlagsAllInList(t *testing.T) {
	gc := &config.GlobalConfig{
		Builtins:   allBuiltinsEnabled(),
		AllowOnAny: []string{"--version", "--help", "-h", "-v"},
	}
	e := New(gc, nil)

	for _, args := range [][]string{
		{"kubectl", "--help", "--version"},
		{"kubectl", "-h", "-v"},
		{"unknown-tool", "--help", "--version"},
		{"unknown-tool", "-h"},
	} {
		cr := e.Check(args)
		if !cr.Allowed {
			t.Errorf("expected allowed for %v (all args in allow_on_any), got blocked: %s", args, cr.Reason)
		}
	}
}

// --- allow_on_any integration tests (shell mode via CheckInnerShellFunc) ---

func newAllowOnAnyEngine(allowOnAny []string) *Engine {
	gc := &config.GlobalConfig{
		Builtins:   allBuiltinsEnabled(),
		AllowOnAny: allowOnAny,
	}
	return New(gc, nil)
}

// allBuiltinsWithShell includes bash/sh so bash -c tests work through the engine.
func allBuiltinsWithShell() map[string]bool {
	m := allBuiltinsEnabled()
	m["bash"] = true
	m["sh"] = true
	return m
}

func newAllowOnAnyEngineWithShell(allowOnAny []string) *Engine {
	gc := &config.GlobalConfig{
		Builtins:   allBuiltinsWithShell(),
		AllowOnAny: allowOnAny,
	}
	return New(gc, nil)
}

func TestAllowOnAnyShellModeUnregistered(t *testing.T) {
	e := newAllowOnAnyEngine([]string{"--version", "--help", "-h", "-v"})
	validateShell := e.CheckInnerShellFunc()

	allowed := []string{
		"chainsaw --help",
		"brew --version",
		"claude -v",
		"unknown-tool -h",
		"chainsaw --version && echo done",
		"git status && chainsaw --help",
		"chainsaw --version && brew --help",
	}
	for _, expr := range allowed {
		if err := validateShell(expr); err != nil {
			t.Errorf("expected allowed for %q, got: %v", expr, err)
		}
	}

	blocked := []string{
		"chainsaw test .",
		"chainsaw",
		"brew install foo",
		"brew search --help",
		"chainsaw test --help",
		"unknown-tool subcommand",
	}
	for _, expr := range blocked {
		if err := validateShell(expr); err == nil {
			t.Errorf("expected blocked for %q", expr)
		}
	}
}

func TestAllowOnAnyShellModeRegistered(t *testing.T) {
	e := newAllowOnAnyEngine([]string{"--version", "--help"})
	validateShell := e.CheckInnerShellFunc()

	allowed := []string{
		"kubectl --help",
		"git --version",
		"docker --help",
		"kubectl --help && git --version",
	}
	for _, expr := range allowed {
		if err := validateShell(expr); err != nil {
			t.Errorf("expected allowed for %q, got: %v", expr, err)
		}
	}

	blocked := []string{
		"kubectl delete --help",
		"docker rm --help",
	}
	for _, expr := range blocked {
		if err := validateShell(expr); err == nil {
			t.Errorf("expected blocked for %q", expr)
		}
	}
}

func TestAllowOnAnyShellModeBashC(t *testing.T) {
	e := newAllowOnAnyEngineWithShell([]string{"--version", "--help"})

	bashCases := []struct {
		args    []string
		allowed bool
	}{
		{[]string{"bash", "-c", "chainsaw --help"}, true},
		{[]string{"bash", "-c", "brew --version"}, true},
		{[]string{"bash", "-c", "chainsaw --help 2>&1"}, true},
		{[]string{"bash", "-c", "git status && chainsaw --help"}, true},
		{[]string{"bash", "-c", "chainsaw test ."}, false},
		{[]string{"bash", "-c", "chainsaw"}, false},
		{[]string{"bash", "-c", "brew install foo"}, false},
	}
	for _, tc := range bashCases {
		cr := e.Check(tc.args)
		if tc.allowed && !cr.Allowed {
			t.Errorf("expected allowed for %v, got blocked: %s", tc.args, cr.Reason)
		}
		if !tc.allowed && cr.Allowed {
			t.Errorf("expected blocked for %v", tc.args)
		}
	}
}

func TestAllowOnAnyShellModeEdgeCases(t *testing.T) {
	e := newAllowOnAnyEngine([]string{"--version", "--help"})
	validateShell := e.CheckInnerShellFunc()

	// Registered tool with only allow_on_any flags (no pipeline — head is not registered)
	if err := validateShell("kubectl --help"); err != nil {
		t.Errorf("expected allowed for kubectl --help, got: %v", err)
	}

	// allow_on_any with variable expansion as arg — blocked (non-literal)
	if err := validateShell("chainsaw $FLAG"); err == nil {
		t.Error("expected blocked for dynamic arg on unregistered tool")
	}

	// Unknown tool with quoted --help
	if err := validateShell(`chainsaw "--help"`); err != nil {
		t.Errorf("expected allowed for quoted --help, got: %v", err)
	}

	// Unknown tool with single-quoted --help
	if err := validateShell(`chainsaw '--help'`); err != nil {
		t.Errorf("expected allowed for single-quoted --help, got: %v", err)
	}
}

func TestAllowOnAnyWrapperIntegration(t *testing.T) {
	e := newAllowOnAnyEngine([]string{"--version", "--help"})

	// timeout wrapping unknown tool with --help
	cr := e.Check([]string{"timeout", "5", "chainsaw", "--help"})
	if !cr.Allowed {
		t.Errorf("expected allowed for timeout wrapping chainsaw --help, got: %s", cr.Reason)
	}

	// timeout wrapping unknown tool with subcommand — blocked
	cr = e.Check([]string{"timeout", "5", "chainsaw", "test", "."})
	if cr.Allowed {
		t.Error("expected blocked for timeout wrapping chainsaw test")
	}

	// nice wrapping unknown tool --version
	cr = e.Check([]string{"nice", "chainsaw", "--version"})
	if !cr.Allowed {
		t.Errorf("expected allowed for nice wrapping chainsaw --version, got: %s", cr.Reason)
	}
}

func TestExtendModeNonInteractiveEnvPreserved(t *testing.T) {
	ghExtend := config.ConfigTool{
		ToolConfig: config.ToolConfig{
			Name:    "gh",
			Mode:    "extend",
			Allowed: []string{"pr"},
			Subcommands: map[string][]string{
				"pr": {"create"},
			},
		},
	}
	gc := &config.GlobalConfig{Builtins: allBuiltinsEnabled()}
	e := New(gc, []config.ConfigTool{ghExtend})

	tool, ok := e.reg.Get("gh")
	if !ok {
		t.Fatal("gh not registered")
	}
	env := tool.NonInteractiveEnv()
	if env["GH_PROMPT_DISABLED"] != "1" {
		t.Errorf("expected GH_PROMPT_DISABLED=1, got %v", env)
	}
}
