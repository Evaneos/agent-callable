package spec

import "fmt"

// ShellCToolSpec validates bash/sh invocations that use the -c flag.
// Only the -c form is supported; bare script execution (bash script.sh) is blocked.
// The inner expression is validated by an injected shell expression validator,
// which recursively parses the AST — including cd-aware writable dir resolution.
type ShellCToolSpec struct {
	name         string
	validateFunc func(expr string) error
}

// NewShellCWrapper creates a ShellCToolSpec for a shell interpreter (bash, sh).
func NewShellCWrapper(name string) *ShellCToolSpec {
	return &ShellCToolSpec{name: name}
}

func (t *ShellCToolSpec) Name() string                         { return t.name }
func (t *ShellCToolSpec) NonInteractiveEnv() map[string]string { return nil }

// SetValidateFunc injects the shell expression validator.
func (t *ShellCToolSpec) SetValidateFunc(fn func(expr string) error) {
	t.validateFunc = fn
}

// safeShellFlags are bash/sh flags that don't open unsafe execution paths.
var safeShellFlags = map[string]bool{
	"-e": true, // exit on error
	"-u": true, // treat unset vars as errors
	"-x": true, // trace execution
	"-v": true, // verbose
	"-n": true, // syntax-check only (dry run)
}

// Check validates bash/sh args. Only `-c <expr>` (with optional safe flags) is allowed.
// Any other form — script file, interactive shell, no -c — is blocked.
func (t *ShellCToolSpec) Check(args []string, _ RuntimeCtx) Result {
	if t.validateFunc == nil {
		return Deny(fmt.Sprintf("%s: shell wrapper not initialized", t.name))
	}

	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			i++
			break
		}
		if arg == "-c" {
			if i+1 >= len(args) {
				return Deny(fmt.Sprintf("%s: -c requires an expression", t.name))
			}
			expr := args[i+1]
			if expr == "" {
				return Deny(fmt.Sprintf("%s: empty -c expression", t.name))
			}
			if err := t.validateFunc(expr); err != nil {
				return Deny(fmt.Sprintf("%s -c: %v", t.name, err))
			}
			return Allow()
		}
		if arg == "-o" {
			i += 2 // skip -o <option>
			continue
		}
		if safeShellFlags[arg] {
			i++
			continue
		}
		// Handle combined single-char flags like -eu (= -e -u) or -eux.
		// If -c appears within a combined flag cluster, it is not supported
		// (e.g. -ec would be ambiguous). Require -c as a standalone flag.
		if len(arg) > 2 && arg[0] == '-' && arg[1] != '-' {
			allSafe := true
			for _, ch := range arg[1:] {
				if !safeShellFlags["-"+string(ch)] {
					allSafe = false
					break
				}
			}
			if allSafe {
				i++
				continue
			}
		}
		return Deny(fmt.Sprintf("%s: only '-c <expr>' form is supported (got %q)", t.name, arg))
	}

	return Deny(fmt.Sprintf("%s: -c flag required", t.name))
}
