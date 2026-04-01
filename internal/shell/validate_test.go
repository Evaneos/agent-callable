package shell

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/evaneos/agent-callable/internal/spec"
	"mvdan.cc/sh/v3/syntax"
)

func testRegistry() *spec.Registry {
	r := spec.NewRegistry()
	// Register some fake tools for testing.
	r.Register(&fakeToolSpec{name: "kubectl"})
	r.Register(&fakeToolSpec{name: "grep"})
	r.Register(&fakeToolSpec{name: "curl"})
	r.Register(&fakeToolSpec{name: "jq"})
	r.Register(&fakeToolSpec{name: "sed"})
	r.Register(&fakeToolSpec{name: "awk"})
	r.Register(&fakeToolSpec{name: "cat"})
	r.Register(&fakeToolSpec{name: "head"})
	r.Register(&fakeToolSpec{name: "sort"})
	r.Register(&fakeToolSpec{name: "wc"})
	r.Register(&fakeToolSpec{name: "xargs"})
	return r
}

type fakeToolSpec struct {
	name string
}

func (f *fakeToolSpec) Name() string { return f.name }
func (f *fakeToolSpec) Check(_ []string, _ spec.RuntimeCtx) spec.Result {
	return spec.Allow()
}
func (f *fakeToolSpec) NonInteractiveEnv() map[string]string { return nil }

var defaultOpts = ValidateOpts{WritableDirs: []string{"/tmp"}}

func TestValidateAllowed(t *testing.T) {
	reg := testRegistry()

	tests := []struct {
		name string
		expr string
	}{
		{"simple command", "kubectl get pods"},
		{"pipe", "kubectl get pods | grep Running"},
		{"for loop", "for ns in prod staging; do kubectl get pods -n $ns; echo '---'; done"},
		{"while loop", "kubectl get pods | while read line; do echo $line; done"},
		{"if/else", "if kubectl get pods; then echo ok; else echo fail; fi"},
		{"subshell", "(kubectl get pods)"},
		{"and operator", "kubectl get pods && echo done"},
		{"or operator", "kubectl get pods || echo failed"},
		{"command substitution", "echo $(kubectl get pods)"},
		{"local function", "helper() { echo hello; }; helper"},
		{"absolute path", "/usr/bin/kubectl get pods"},
		{"builtins only", "echo hello; true; false; pwd"},
		{"pure assignment", "FOO=bar"},
		{"assignment with export", "export FOO=bar"},
		{"redirect to /dev/null", "kubectl get pods > /dev/null"},
		{"redirect to /tmp", "kubectl get pods > /tmp/out.txt"},
		{"append to /tmp", "kubectl get pods >> /tmp/out.txt"},
		{"redirect stderr to /dev/null", "kubectl get pods 2>/dev/null"},
		{"fd duplication 2>&1", "kubectl get pods 2>&1"},
		{"fd dup in pipeline", "kubectl get pods 2>&1 | head -5"},
		{"complex pipeline", "kubectl get pods -A | grep -v Running | awk '{print $1}' | sort | head -5"},
		{"backtick substitution", "echo `kubectl get pods`"},
		{"nested substitution", "echo $(echo $(kubectl get pods))"},
		{"test builtin", "test -f /tmp/foo && echo exists"},
		{"bracket test", "[ -f /tmp/foo ] && echo exists"},
		{"read builtin", "echo hello | read VAR"},
		{"set builtin", "set -e"},
		{"shift builtin", "shift"},
		{"colon builtin", ": noop"},
		{"agent-callable nested", "agent-callable kubectl get pods"},
		{"double-quoted arg", `echo "hello world"`},
		{"single-quoted arg", "echo 'hello world'"},
		{"quoted label selector", `kubectl get pods -l "app=nginx"`},
		{"mixed quotes", `echo "hello" 'world'`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Validate(tt.expr, reg, defaultOpts)
			if err != nil {
				t.Errorf("expected allowed, got error: %v", err)
			}
		})
	}
}

func TestValidateBlocked(t *testing.T) {
	reg := testRegistry()

	tests := []struct {
		name string
		expr string
		want string // substring of error message
	}{
		{"unknown command", "unknown_cmd args", "not allowed"},
		{"dynamic command", "$CMD args", "dynamic command"},
		{"eval", "eval echo bad", "dangerous"},
		{"exec", "exec /bin/sh", "dangerous"},
		{"source", "source script.sh", "dangerous"},
		{"dot source", ". script.sh", "dangerous"},
		{"command bypass", "command kubectl get pods", "dangerous"},
		{"builtin bypass", "builtin echo hello", "dangerous"},
		{"trap", "trap 'echo trapped' EXIT", "dangerous"},
		{"bad in pipe", "kubectl get pods | unknown_cmd", "not allowed"},
		{"syntax error", "if then done", "syntax"},
		{"empty expression", "", "empty"},
		{"whitespace only", "   ", "empty"},
		{"redirect to home", "kubectl get pods > ~/important.txt", "blocked"},
		{"redirect dynamic", "kubectl get pods > $FILE", "dynamic"},
		{"redirect to /etc", "kubectl get pods > /etc/passwd", "blocked"},
		{"bad cmd in for", "for x in a b; do unknown_cmd $x; done", "not allowed"},
		{"bad cmd in if", "if unknown_cmd; then echo ok; fi", "not allowed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Validate(tt.expr, reg, defaultOpts)
			if err == nil {
				t.Error("expected error, got nil")
				return
			}
			if tt.want != "" {
				if got := err.Error(); !strings.Contains(got, tt.want) {
					t.Errorf("expected error containing %q, got %q", tt.want, got)
				}
			}
		})
	}
}

func TestValidateResultToolNames(t *testing.T) {
	reg := testRegistry()

	result, err := Validate("kubectl get pods | grep Running", reg, defaultOpts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	toolSet := make(map[string]bool)
	for _, name := range result.ToolNames {
		toolSet[name] = true
	}

	if !toolSet["kubectl"] {
		t.Error("expected kubectl in tool names")
	}
	if !toolSet["grep"] {
		t.Error("expected grep in tool names")
	}
}

func TestValidateWritableDirs(t *testing.T) {
	reg := testRegistry()

	// With custom writable dirs
	opts := ValidateOpts{WritableDirs: []string{"/tmp", "/var/log"}}

	allowed := []string{
		"echo hello > /tmp/out.txt",
		"echo hello > /var/log/test.log",
		"echo hello > /dev/null",
	}
	for _, expr := range allowed {
		_, err := Validate(expr, reg, opts)
		if err != nil {
			t.Errorf("expected allowed for %q, got: %v", expr, err)
		}
	}

	blocked := []string{
		"echo hello > /home/user/file.txt",
		"echo hello > /etc/passwd",
	}
	for _, expr := range blocked {
		_, err := Validate(expr, reg, opts)
		if err == nil {
			t.Errorf("expected blocked for %q", expr)
		}
	}
}

func TestCheckFuncReceivesQuotedArgs(t *testing.T) {
	reg := testRegistry()
	var captured []string

	opts := ValidateOpts{
		WritableDirs: []string{"/tmp"},
		CheckFunc: func(name string, args []string) error {
			captured = append(captured, args...)
			return nil
		},
	}

	tests := []struct {
		name string
		expr string
		want []string
	}{
		{"bare args", "kubectl get pods", []string{"get", "pods"}},
		{"dq arg", `kubectl get secrets -o "yaml"`, []string{"get", "secrets", "-o", "yaml"}},
		{"sq arg", "kubectl get secrets -o 'json'", []string{"get", "secrets", "-o", "json"}},
		{"label selector", `kubectl get pods -l "app=nginx"`, []string{"get", "pods", "-l", "app=nginx"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			captured = nil
			_, err := Validate(tt.expr, reg, opts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(captured) != len(tt.want) {
				t.Fatalf("got %v, want %v", captured, tt.want)
			}
			for i := range captured {
				if captured[i] != tt.want[i] {
					t.Errorf("arg[%d] = %q, want %q", i, captured[i], tt.want[i])
				}
			}
		})
	}
}

func TestValidateAgentCallableNested(t *testing.T) {
	reg := spec.NewRegistry()
	reg.Register(&fakeToolSpec{name: "kubectl"})

	_, err := Validate("agent-callable kubectl get pods", reg, defaultOpts)
	if err != nil {
		t.Errorf("expected allowed for nested agent-callable, got error: %v", err)
	}
}

func TestCheckFuncErrorPropagation(t *testing.T) {
	reg := spec.NewRegistry()
	reg.Register(&fakeToolSpec{name: "kubectl"})

	sentinel := errors.New("kubectl blocked by policy")
	opts := ValidateOpts{
		WritableDirs: []string{"/tmp"},
		CheckFunc: func(name string, args []string) error {
			if name == "kubectl" {
				return sentinel
			}
			return nil
		},
	}

	_, err := Validate("kubectl get pods", reg, opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != sentinel {
		t.Errorf("expected sentinel error, got: %v", err)
	}
}

func TestCheckFuncCalledWithArgs(t *testing.T) {
	reg := spec.NewRegistry()
	reg.Register(&fakeToolSpec{name: "kubectl"})

	var capturedName string
	var capturedArgs []string

	opts := ValidateOpts{
		WritableDirs: []string{"/tmp"},
		CheckFunc: func(name string, args []string) error {
			capturedName = name
			capturedArgs = args
			return nil
		},
	}

	_, err := Validate("kubectl get pods -n test", reg, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedName != "kubectl" {
		t.Errorf("CheckFunc name = %q, want %q", capturedName, "kubectl")
	}

	wantArgs := []string{"get", "pods", "-n", "test"}
	if len(capturedArgs) != len(wantArgs) {
		t.Fatalf("CheckFunc args = %v, want %v", capturedArgs, wantArgs)
	}
	for i := range wantArgs {
		if capturedArgs[i] != wantArgs[i] {
			t.Errorf("CheckFunc args[%d] = %q, want %q", i, capturedArgs[i], wantArgs[i])
		}
	}
}

func TestValidateLocalFuncAllowed(t *testing.T) {
	reg := spec.NewRegistry()

	_, err := Validate("myfunc() { echo hello; }; myfunc", reg, defaultOpts)
	if err != nil {
		t.Errorf("expected allowed for local function definition and call, got error: %v", err)
	}
}

func TestWordLitQuotes(t *testing.T) {
	parse := func(expr string) *syntax.Word {
		parser := syntax.NewParser()
		prog, err := parser.Parse(strings.NewReader(expr), "")
		if err != nil {
			t.Fatalf("parse %q: %v", expr, err)
		}
		// Return second word (first arg) of first simple command.
		var word *syntax.Word
		syntax.Walk(prog, func(node syntax.Node) bool {
			if call, ok := node.(*syntax.CallExpr); ok && len(call.Args) > 1 && word == nil {
				word = call.Args[1]
				return false
			}
			return true
		})
		return word
	}

	tests := []struct {
		name string
		expr string
		want string
	}{
		{"bare", "echo hello", "hello"},
		{"double-quoted", `echo "hello"`, "hello"},
		{"single-quoted", "echo 'hello'", "hello"},
		{"dq with spaces", `echo "hello world"`, "hello world"},
		{"sq with spaces", "echo 'hello world'", "hello world"},
		{"mixed lit+dq", `echo foo"bar"`, "foobar"},
		{"variable expansion", `echo "$HOME"`, ""},
		{"cmd substitution", "echo \"$(whoami)\"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := parse(tt.expr)
			if w == nil {
				t.Fatal("no argument word found")
			}
			got := wordLit(w)
			if got != tt.want {
				t.Errorf("wordLit = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- Wrapper integration tests (--sh mode) ---

// wireWrappers simulates what Engine does: it wires a checkFunc into every
// WrapperToolSpec in the registry and returns ValidateOpts with a matching
// CheckFunc. Call this after registering all fake tools and wrappers.
func wireWrappers(reg *spec.Registry) ValidateOpts {
	wrapperFn := func(name string, args []string) ([]string, error) {
		t, ok := reg.Get(name)
		if !ok {
			return nil, fmt.Errorf("command %q not allowed", name)
		}
		res := t.Check(args, spec.RuntimeCtx{})
		if res.Decision != spec.DecisionAllow {
			return nil, fmt.Errorf("%s", res.Reason)
		}
		return res.NonInteractiveArgs, nil
	}

	for _, name := range reg.Names() {
		t, ok := reg.Get(name)
		if !ok {
			continue
		}
		if w, ok := t.(interface {
			SetCheckFunc(func(string, []string) ([]string, error))
		}); ok {
			w.SetCheckFunc(wrapperFn)
		}
	}

	return ValidateOpts{
		WritableDirs: []string{"/tmp"},
		CheckFunc: func(name string, args []string) error {
			_, err := wrapperFn(name, args)
			return err
		},
	}
}

func wrapperShellRegistry() (*spec.Registry, ValidateOpts) {
	reg := spec.NewRegistry()
	for _, name := range []string{"git", "echo", "head", "cat"} {
		reg.Register(&fakeToolSpec{name: name})
	}
	reg.Register(spec.NewWrapper("timeout", spec.ExtractAfterFlagsAndN(1, map[string]bool{
		"-k": true, "--kill-after": true,
		"-s": true, "--signal": true,
	})))
	reg.Register(spec.NewWrapper("nice", spec.ExtractAfterFlags(map[string]bool{
		"-n": true, "--adjustment": true,
	})))
	return reg, wireWrappers(reg)
}

func TestShellWrapperPipeline(t *testing.T) {
	reg, opts := wrapperShellRegistry()
	_, err := Validate("timeout 5 git log | head", reg, opts)
	if err != nil {
		t.Fatalf("expected allowed, got: %v", err)
	}
}

func TestShellWrapperSequence(t *testing.T) {
	reg, opts := wrapperShellRegistry()
	_, err := Validate("timeout 5 git log; echo done", reg, opts)
	if err != nil {
		t.Fatalf("expected allowed, got: %v", err)
	}
}

func TestShellWrapperAndChain(t *testing.T) {
	reg, opts := wrapperShellRegistry()
	_, err := Validate("timeout 5 git log && echo ok", reg, opts)
	if err != nil {
		t.Fatalf("expected allowed, got: %v", err)
	}
}

func TestShellWrapperOrChain(t *testing.T) {
	reg, opts := wrapperShellRegistry()
	_, err := Validate("timeout 5 git log || echo fail", reg, opts)
	if err != nil {
		t.Fatalf("expected allowed, got: %v", err)
	}
}

func TestShellWrapperPipelineDenied(t *testing.T) {
	reg, opts := wrapperShellRegistry()
	_, err := Validate("timeout 5 unknown-cmd | cat", reg, opts)
	if err == nil {
		t.Fatal("expected blocked for timeout wrapping unknown cmd in pipeline")
	}
}

func TestShellWrapperAndChainDenied(t *testing.T) {
	reg, opts := wrapperShellRegistry()
	_, err := Validate("git log && timeout 5 unknown-cmd", reg, opts)
	if err == nil {
		t.Fatal("expected blocked for unknown-cmd in && chain")
	}
}

func TestShellWrapperRedirectAllowed(t *testing.T) {
	reg, opts := wrapperShellRegistry()
	_, err := Validate("timeout 5 git log > /tmp/out", reg, opts)
	if err != nil {
		t.Fatalf("expected allowed, got: %v", err)
	}
}

func TestShellWrapperRedirectDenied(t *testing.T) {
	reg, opts := wrapperShellRegistry()
	_, err := Validate("timeout 5 git log > /etc/out", reg, opts)
	if err == nil {
		t.Fatal("expected blocked for redirect outside writable dirs")
	}
}

func TestShellWrapperNestedInPipeline(t *testing.T) {
	reg, opts := wrapperShellRegistry()
	_, err := Validate("nice timeout 5 git log | head -5", reg, opts)
	if err != nil {
		t.Fatalf("expected allowed for nested wrappers in pipeline, got: %v", err)
	}
}

func TestShellWrapperDynamic(t *testing.T) {
	reg, opts := wrapperShellRegistry()
	_, err := Validate("timeout 5 $CMD", reg, opts)
	if err == nil {
		t.Fatal("expected blocked for dynamic command in wrapper")
	}
}

func xargsShellRegistry() (*spec.Registry, ValidateOpts) {
	reg := spec.NewRegistry()
	for _, name := range []string{"grep", "cat", "head", "find"} {
		reg.Register(&fakeToolSpec{name: name})
	}
	reg.Register(spec.NewWrapper("xargs", spec.ExtractAfterFlags(map[string]bool{
		"-n": true, "--max-args": true,
		"-P": true, "--max-procs": true,
		"-s": true, "--max-chars": true,
		"-a": true, "--arg-file": true,
		"-I": true, "--replace": true,
		"-L": true, "--max-lines": true,
		"-d": true, "--delimiter": true,
		"--process-slot-var": true,
	})))
	return reg, wireWrappers(reg)
}

func TestXargsAllowed(t *testing.T) {
	reg, opts := xargsShellRegistry()
	tests := []struct {
		name string
		expr string
	}{
		{"simple pipe grep", "find . | xargs grep pattern"},
		{"simple pipe cat", "find . -name '*.go' | xargs cat"},
		{"with -I replace", "find . | xargs -I {} cat {}"},
		{"with -I embedded", "find . | xargs -I{} grep {} file"},
		{"with -n flag", "find . | xargs -n 10 grep pattern"},
		{"with -P flag", "find . | xargs -P 4 grep pattern"},
		{"with -a flag", "xargs -a /tmp/list.txt grep foo"},
		{"with --max-args", "find . | xargs --max-args=5 grep foo"},
		{"with --replace", "find . | xargs --replace={} cat {}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Validate(tt.expr, reg, opts)
			if err != nil {
				t.Errorf("expected allowed, got: %v", err)
			}
		})
	}
}

func TestXargsBlocked(t *testing.T) {
	reg, opts := xargsShellRegistry()
	tests := []struct {
		name string
		expr string
	}{
		{"rm not in registry", "find . | xargs rm"},
		{"bash not in registry", "echo file | xargs bash"},
		{"sh not in registry", "find . | xargs sh -c 'evil'"},
		{"dynamic command", "find . | xargs $CMD"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Validate(tt.expr, reg, opts)
			if err == nil {
				t.Errorf("expected blocked for %q", tt.expr)
			}
		})
	}
}

// --- bash/sh -c integration tests ---

// bashShellRegistry creates a registry with bash/sh wired for recursive shell validation.
func bashShellRegistry() (*spec.Registry, ValidateOpts) {
	reg := spec.NewRegistry()
	for _, name := range []string{"git", "npm", "npx", "tsc", "echo", "head", "grep", "cat"} {
		reg.Register(&fakeToolSpec{name: name})
	}

	bashWrapper := spec.NewShellCWrapper("bash")
	shWrapper := spec.NewShellCWrapper("sh")
	reg.Register(bashWrapper)
	reg.Register(shWrapper)

	// CheckFunc validates via registry (used for inner commands in recursive calls).
	checkFn := func(name string, args []string) error {
		t, ok := reg.Get(name)
		if !ok {
			return fmt.Errorf("command %q not allowed", name)
		}
		res := t.Check(args, spec.RuntimeCtx{})
		if res.Decision != spec.DecisionAllow {
			return fmt.Errorf("%s", res.Reason)
		}
		return nil
	}

	opts := ValidateOpts{
		WritableDirs: []string{"/tmp"},
		CheckFunc:    checkFn,
	}

	// Inject recursive shell validator into bash/sh wrappers.
	shellValidateFn := func(expr string) error {
		_, err := Validate(expr, reg, opts)
		return err
	}
	bashWrapper.SetValidateFunc(shellValidateFn)
	shWrapper.SetValidateFunc(shellValidateFn)

	return reg, opts
}

func TestBashCAllowed(t *testing.T) {
	reg, opts := bashShellRegistry()
	tests := []struct {
		name string
		expr string
	}{
		{"simple allowed command", `bash -c "git status"`},
		{"pipeline", `bash -c "git log | head -5"`},
		{"cd + command", `bash -c "cd /tmp && git status"`},
		{"cd + pipe + redirect", `bash -c "cd /tmp && git log > out.txt"`},
		{"stderr redirect", `bash -c "git status 2>&1"`},
		{"stderr to /dev/null", `bash -c "git status 2>/dev/null"`},
		{"sh variant", `sh -c "git status"`},
		{"safe flag -e", `bash -e -c "git status"`},
		{"safe flag -u", `bash -u -c "git status"`},
		{"safe flags combined", `bash -eu -c "git status"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Validate(tt.expr, reg, opts)
			if err != nil {
				t.Errorf("expected allowed, got: %v", err)
			}
		})
	}
}

func TestBashCBlocked(t *testing.T) {
	reg, opts := bashShellRegistry()
	tests := []struct {
		name string
		expr string
	}{
		{"inner command not in registry", `bash -c "rm -rf /"`},
		{"unknown inner command in chain", `bash -c "git log && curl -X DELETE http://example.com"`},
		{"bash without -c", `bash script.sh`},
		{"bash no args", `bash`},
		{"redirect outside writable dirs", `bash -c "git log > /etc/out.txt"`},
		{"sh blocked inner", `sh -c "rm -rf /"`},
		{"nested bash inner blocked", `bash -c "bash -c \"rm -rf /\""`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Validate(tt.expr, reg, opts)
			if err == nil {
				t.Errorf("expected blocked for %q", tt.expr)
			}
		})
	}
}

func TestBashCCdTracking(t *testing.T) {
	reg, opts := bashShellRegistry()

	// Relative redirect after cd: resolves to /tmp/out.txt → allowed.
	_, err := Validate(`bash -c "cd /tmp && git log > out.txt"`, reg, opts)
	if err != nil {
		t.Errorf("expected allowed (cd /tmp → out.txt resolves to /tmp/out.txt), got: %v", err)
	}

	// Relative redirect without cd: resolves against process CWD → blocked.
	_, err = Validate(`bash -c "git log > out.txt"`, reg, opts)
	if err == nil {
		t.Error("expected blocked (out.txt with no cd has no writable resolution)")
	}

	// Redirect to absolute writable path — always allowed regardless of cd.
	_, err = Validate(`bash -c "git log > /tmp/out.txt"`, reg, opts)
	if err != nil {
		t.Errorf("expected allowed (absolute /tmp/out.txt), got: %v", err)
	}
}

func TestBashCTypicalClaudePattern(t *testing.T) {
	reg, opts := bashShellRegistry()
	// Typical pattern from audit log: cd into a project dir, run a TS check.
	expr := `bash -c "cd /home/grs/work/cloud-infrastructure/product/cloudsql/staging && npx tsc --noEmit 2>&1 | head -20"`
	_, err := Validate(expr, reg, opts)
	if err != nil {
		t.Errorf("expected allowed for typical Claude cd+npx pattern, got: %v", err)
	}
}

// --- Combinatorial bash/sh tests ---

func TestBashCAllowedCombinatorial(t *testing.T) {
	reg, opts := bashShellRegistry()
	tests := []struct {
		name string
		expr string
	}{
		// Single commands
		{"git log oneline", `bash -c "git log --oneline -10"`},
		{"git diff stat", `bash -c "git diff --stat"`},
		{"npx tsc noEmit", `bash -c "npx tsc --noEmit"`},
		{"tsc direct", `bash -c "tsc --noEmit"`},

		// Pipelines
		{"double pipe", `bash -c "git log | head -5"`},
		{"triple pipe", `bash -c "git log | grep feat | head -10"`},
		{"pipe stderr redir", `bash -c "git log 2>&1 | head -5"`},
		{"pipe to cat", `bash -c "git status | cat"`},
		{"pipe stderr devnull", `bash -c "git log 2>/dev/null | head -5"`},
		{"stderr before pipe", `bash -c "npx tsc --noEmit 2>&1 | grep error | head -20"`},

		// Sequential operators
		{"semicolon", `bash -c "git fetch; git status"`},
		{"and chain", `bash -c "git fetch && git status"`},
		{"or operator", `bash -c "git status || echo failed"`},
		{"and-or", `bash -c "git fetch && git status || echo failed"`},
		{"triple and", `bash -c "git fetch && git status && git log | head -5"`},
		{"semicolons", `bash -c "git fetch; git status; git log | head -5"`},

		// cd patterns
		{"cd absolute + cmd", `bash -c "cd /tmp && git status"`},
		{"cd + pipe", `bash -c "cd /tmp && git log | head -5"`},
		{"cd + stderr pipe", `bash -c "cd /tmp && npx tsc --noEmit 2>&1 | head -20"`},
		{"cd + redirect /tmp", `bash -c "cd /tmp && git log > output.txt"`},
		{"cd + append /tmp", `bash -c "cd /tmp && git log >> output.txt"`},
		{"double cd linear", `bash -c "cd /tmp && echo here && cd /tmp && git status"`},
		{"cd /home path", `bash -c "cd /home/grs/work && git status"`},
		{"cd + semicolons", `bash -c "cd /tmp; git status; git log | head -3"`},

		// Redirects
		{"stdout devnull", `bash -c "git log > /dev/null"`},
		{"stderr devnull", `bash -c "git status 2>/dev/null"`},
		{"both devnull", `bash -c "git status > /dev/null 2>&1"`},
		{"absolute /tmp", `bash -c "git log > /tmp/output.txt"`},
		{"absolute /tmp append", `bash -c "git log >> /tmp/out.txt"`},
		{"fd dup 2>&1", `bash -c "git status 2>&1"`},
		{"fd dup in pipe", `bash -c "git status 2>&1 | cat"`},

		// Loops
		{"for loop echo", `bash -c "for ns in prod staging; do echo $ns; done"`},
		{"for loop git", `bash -c "for f in a b; do git status; done"`},
		{"while read", `bash -c "git log --oneline | while read line; do echo $line; done"`},

		// Conditionals
		{"if then", `bash -c "if git status; then echo ok; fi"`},
		{"if then else", `bash -c "if git status; then echo ok; else echo fail; fi"`},
		{"test builtin", `bash -c "[ -f /tmp/foo ] && git status"`},

		// env prefix
		{"env prefix cmd", `bash -c "GIT_PAGER=cat git log"`},
		{"env prefix pipe", `bash -c "CI=1 npx tsc --noEmit 2>&1 | head -10"`},

		// Safe flag combinations
		{"flag -e", `bash -e -c "git status"`},
		{"flag -u", `bash -u -c "git status"`},
		{"flag -x", `bash -x -c "git status"`},
		{"flag -eu combined", `bash -eu -c "git status"`},
		{"flag -eux combined", `bash -eux -c "git status"`},
		{"flag -e -u separate", `bash -e -u -c "git status"`},
		{"sh -e flag", `sh -e -c "git status"`},
		{"sh -eu combined", `sh -eu -c "git status"`},

		// sh variant
		{"sh simple", `sh -c "git status"`},
		{"sh pipeline", `sh -c "git log | head -5"`},
		{"sh cd + cmd", `sh -c "cd /tmp && git status"`},
		{"sh stderr pipe", `sh -c "npx tsc --noEmit 2>&1 | head -10"`},

		// Subshell and substitution
		{"subshell", `bash -c "(git status)"`},
		{"subshell pipeline", `bash -c "(git log | head -5)"`},
		{"command substitution", `bash -c "echo $(git log --oneline -1)"`},

		// Multi-line scripts (literal newlines in -c expression)
		{"multiline simple", "bash -c \"\ncd /tmp\ngit status\n\""},
		{"multiline pipeline", "bash -c \"\ngit fetch\ngit log | head -5\n\""},
		{"multiline cd redirect", "bash -c \"\ncd /tmp\ngit log > output.txt\n\""},
		{"multiline three steps", "bash -c \"\ncd /tmp\ngit fetch\ngit status\ngit log | head -3\n\""},
		{"multiline semicolons", "bash -c \"cd /tmp\ngit status; git log | head -5\""},
		{"multiline with loop", "bash -c \"\nfor ns in prod staging; do\n  echo $ns\n  git status\ndone\n\""},

		// Real-world Claude patterns from audit log
		{"audit pattern 1", `bash -c "cd /home/grs/work/cloud-infrastructure/product/cloudsql/staging && npx tsc --noEmit 2>&1 | head -20"`},
		{"audit pattern 2", `bash -c "cd /home/grs/work/cloud-infrastructure/product/cloudsql/staging && npx tsc --noEmit 2>&1 | grep -v '^$' | head -20"`},
		{"audit pattern 3", `bash -c "cd /home/grs/work/cloud-infrastructure/product/cloudsql/staging && npm install && npx tsc --noEmit 2>&1 | head -30"`},
		{"audit pattern 4", `bash -c "cd /home/grs/work/some-repo && tsc --noEmit 2>/dev/null && echo ok"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Validate(tt.expr, reg, opts)
			if err != nil {
				t.Errorf("expected allowed, got: %v", err)
			}
		})
	}
}

func TestBashCBlockedCombinatorial(t *testing.T) {
	reg, opts := bashShellRegistry()
	tests := []struct {
		name string
		expr string
	}{
		// Commands not in registry
		{"rm", `bash -c "rm -rf /"`},
		{"curl delete", `bash -c "curl -X DELETE http://api.example.com"`},
		{"wget", `bash -c "wget http://example.com -O /tmp/x"`},
		{"unknown in and-chain", `bash -c "git status && rm -rf /tmp/x"`},
		{"unknown in pipe", `bash -c "git log | rm -rf /"`},
		{"unknown in loop", `bash -c "for f in a b; do rm $f; done"`},
		{"unknown in if-then", `bash -c "if true; then rm -f /tmp/x; fi"`},

		// Dangerous builtins
		{"eval bypass", `bash -c "eval 'git push'"`},
		{"exec bypass", `bash -c "exec git push"`},
		{"source bypass", `bash -c "source /etc/profile"`},
		{"dot bypass", `bash -c ". /etc/profile"`},

		// Dynamic command names
		{"var as cmd", `bash -c "$CMD git status"`},
		{"subst as cmd", `bash -c "$(cat /etc/passwd)"`},
		{"var in pipe", `bash -c "git log | $FILTER"`},

		// bash without -c
		{"script file", `bash script.sh`},
		{"absolute script", `bash /home/grs/scripts/deploy.sh`},
		{"no args", `bash`},
		{"flag only", `bash -e`},

		// Redirects outside writable_dirs
		{"to /etc", `bash -c "git log > /etc/out.txt"`},
		{"to /usr/local", `bash -c "git log > /usr/local/bin/exploit"`},
		{"to /root", `bash -c "git log > /root/.bashrc"`},
		{"relative no cd", `bash -c "git log > out.txt"`},
		{"to /var", `bash -c "git log > /var/log/syslog"`},
		{"cd + redirect outside", `bash -c "cd /tmp && git log > /etc/out.txt"`},

		// Nested bash with blocked inner
		{"nested blocked cmd", `bash -c "bash -c 'rm -rf /'"`},
		{"nested blocked redir", `bash -c "bash -c \"git log > /etc/out.txt\""`},

		// sh blocked
		{"sh rm", `sh -c "rm -rf /"`},
		{"sh eval", `sh -c "eval 'something'"`},
		{"sh no -c", `sh script.sh`},

		// Multiline with blocked content
		{"multiline blocked cmd", "bash -c \"\ngit status\nrm -rf /\n\""},
		{"multiline blocked redir", "bash -c \"\ncd /tmp\ngit log > /etc/out.txt\n\""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Validate(tt.expr, reg, opts)
			if err == nil {
				t.Errorf("expected blocked for %q", tt.expr)
			}
		})
	}
}

func TestBashCCdTrackingCombinatorial(t *testing.T) {
	reg, opts := bashShellRegistry()

	// Allowed: redirect resolves to writable dir via cd
	allowedCd := []struct {
		name string
		expr string
	}{
		{"cd /tmp redirect", `bash -c "cd /tmp && git log > out.txt"`},
		{"cd /tmp append", `bash -c "cd /tmp && git log >> out.txt"`},
		{"cd /tmp semicolon redirect", `bash -c "cd /tmp; git log > out.txt"`},
		{"cd /tmp multiline", "bash -c \"\ncd /tmp\ngit log > out.txt\n\""},
		{"absolute /tmp always", `bash -c "git log > /tmp/out.txt"`},
		{"absolute /tmp no cd needed", `bash -c "cd /srv && git log > /tmp/out.txt"`},
	}
	for _, tt := range allowedCd {
		t.Run("allowed/"+tt.name, func(t *testing.T) {
			_, err := Validate(tt.expr, reg, opts)
			if err != nil {
				t.Errorf("expected allowed, got: %v", err)
			}
		})
	}

	// Blocked: redirect cannot be resolved to a writable dir
	blockedCd := []struct {
		name string
		expr string
	}{
		{"relative no cd", `bash -c "git log > out.txt"`},
		{"relative cd to non-writable", `bash -c "cd /srv && git log > out.txt"`},
		{"absolute non-writable", `bash -c "git log > /etc/out.txt"`},
		{"cd /etc redirect relative", `bash -c "cd /etc && git log > out.txt"`},
	}
	for _, tt := range blockedCd {
		t.Run("blocked/"+tt.name, func(t *testing.T) {
			_, err := Validate(tt.expr, reg, opts)
			if err == nil {
				t.Errorf("expected blocked for %q", tt.expr)
			}
		})
	}
}

func TestBashCCdTildeAndRelative(t *testing.T) {
	reg, opts := bashShellRegistry()

	// Tilde expansion: cd ~ / cd ~/path resolve to home dir.
	// home is not in writable_dirs (["/tmp"]) → relative redirect is blocked.
	// This confirms tilde expansion ran (path resolves to /home/grs/out.txt, not "out.txt").
	tildeBlocked := []struct {
		name string
		expr string
	}{
		{"cd ~ redirect home-relative", `bash -c "cd ~ && git log > out.txt"`},
		{"cd ~/work redirect home-relative", `bash -c "cd ~/work && git log > out.txt"`},
		{"cd ~/work/repo redirect home-relative", `bash -c "cd ~/work/myrepo && git log > out.txt"`},
	}
	for _, tt := range tildeBlocked {
		t.Run("tilde_blocked/"+tt.name, func(t *testing.T) {
			_, err := Validate(tt.expr, reg, opts)
			if err == nil {
				t.Errorf("expected blocked (home not in writable_dirs) for %q", tt.expr)
			}
		})
	}

	// After cd ~/..., absolute /tmp redirects are still fine.
	tildeAbsAllowed := []struct {
		name string
		expr string
	}{
		{"cd ~ then absolute /tmp", `bash -c "cd ~ && git log > /tmp/out.txt"`},
		{"cd ~/work then absolute /tmp", `bash -c "cd ~/work && git log > /tmp/out.txt"`},
	}
	for _, tt := range tildeAbsAllowed {
		t.Run("tilde_abs_allowed/"+tt.name, func(t *testing.T) {
			_, err := Validate(tt.expr, reg, opts)
			if err != nil {
				t.Errorf("expected allowed, got: %v", err)
			}
		})
	}

	// Relative cd from a known dir — combines previous cd with relative step.
	relAllowed := []struct {
		name string
		expr string
	}{
		{"cd /tmp then subdir redirect", `bash -c "cd /tmp && cd subdir && git log > out.txt"`},
	}
	for _, tt := range relAllowed {
		t.Run("rel_allowed/"+tt.name, func(t *testing.T) {
			_, err := Validate(tt.expr, reg, opts)
			if err != nil {
				t.Errorf("expected allowed, got: %v", err)
			}
		})
	}
}

func TestBashCCdUncertain(t *testing.T) {
	reg, opts := bashShellRegistry()

	// After cd $VAR, any relative redirect should be blocked.
	uncertainBlocked := []struct {
		name string
		expr string
	}{
		{"cd var then relative redirect", `bash -c "cd $WORKDIR && git log > out.txt"`},
		{"cd var then cmd redirect", `bash -c "cd $DIR; git log > result.txt"`},
	}
	for _, tt := range uncertainBlocked {
		t.Run("uncertain_blocked/"+tt.name, func(t *testing.T) {
			_, err := Validate(tt.expr, reg, opts)
			if err == nil {
				t.Errorf("expected blocked (uncertain cwd) for %q", tt.expr)
			}
		})
	}

	// After cd $VAR, absolute redirects are still fine.
	uncertainAllowedAbsolute := []struct {
		name string
		expr string
	}{
		{"cd var then absolute /tmp redirect", `bash -c "cd $WORKDIR && git log > /tmp/out.txt"`},
	}
	for _, tt := range uncertainAllowedAbsolute {
		t.Run("uncertain_allowed/"+tt.name, func(t *testing.T) {
			_, err := Validate(tt.expr, reg, opts)
			if err != nil {
				t.Errorf("expected allowed (absolute path), got: %v", err)
			}
		})
	}
}
