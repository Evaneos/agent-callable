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
