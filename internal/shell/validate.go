package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/evaneos/agent-callable/internal/spec"
	"mvdan.cc/sh/v3/syntax"
)

// Result contains the tools found during validation.
type Result struct {
	ToolNames []string // tool names from the registry found in the expression
}

// ValidateOpts configures validation behavior.
type ValidateOpts struct {
	WritableDirs []string                               // directories where > and >> are allowed
	CheckFunc    func(name string, args []string) error // optional per-command argument check
	AllowOnAny   []string                               // flags universally allowed on any command
}

// Validate parses a shell expression and checks that all commands are allowed.
func Validate(expr string, reg *spec.Registry, opts ValidateOpts) (*Result, error) {
	if strings.TrimSpace(expr) == "" {
		return nil, fmt.Errorf("empty expression")
	}

	parser := syntax.NewParser()
	prog, err := parser.Parse(strings.NewReader(expr), "")
	if err != nil {
		return nil, fmt.Errorf("shell syntax error: %w", err)
	}

	localFuncs := make(map[string]bool)
	toolSet := make(map[string]bool)
	var walkErr error
	var currentDir string        // tracks the last known cd destination for relative path resolution
	var currentDirUncertain bool // true when cd was to a dynamic/unresolvable path

	// First pass: collect function declarations.
	syntax.Walk(prog, func(node syntax.Node) bool {
		if fd, ok := node.(*syntax.FuncDecl); ok {
			localFuncs[fd.Name.Value] = true
		}
		return true
	})

	// Second pass: validate commands and redirections.
	syntax.Walk(prog, func(node syntax.Node) bool {
		if walkErr != nil {
			return false
		}

		switch n := node.(type) {
		case *syntax.CallExpr:
			if len(n.Args) == 0 {
				// Pure assignments (e.g. FOO=bar) — allowed.
				return true
			}
			name := wordLit(n.Args[0])
			if name == "" {
				walkErr = fmt.Errorf("dynamic command blocked (variable/substitution as command name)")
				return false
			}
			name = filepath.Base(name) // handle /usr/bin/foo → foo

			if name == "agent-callable" {
				return true
			}

			// Track cd to resolve relative paths in subsequent redirections.
			if name == "cd" {
				if len(n.Args) < 2 {
					// bare cd → home directory
					if home, err := os.UserHomeDir(); err == nil {
						currentDir = home
						currentDirUncertain = false
					} else {
						currentDirUncertain = true
					}
				} else {
					arg := wordLit(n.Args[1])
					if arg == "" {
						// dynamic cd arg (e.g. cd $VAR) — can't track
						currentDirUncertain = true
					} else {
						currentDirUncertain = false
						switch {
						case arg == "~":
							if home, err := os.UserHomeDir(); err == nil {
								currentDir = home
							} else {
								currentDirUncertain = true
							}
						case strings.HasPrefix(arg, "~/"):
							if home, err := os.UserHomeDir(); err == nil {
								currentDir = filepath.Join(home, arg[2:])
							} else {
								currentDirUncertain = true
							}
						case filepath.IsAbs(arg):
							currentDir = arg
						case currentDir != "":
							currentDir = filepath.Join(currentDir, arg)
						default:
							// relative cd from unknown cwd
							currentDirUncertain = true
						}
					}
				}
			}

			if !isAllowed(name, localFuncs, reg) {
				// allow_on_any: universally safe flags on unregistered commands.
				if shellAllowedOnAny(n.Args, opts.AllowOnAny) {
					return true
				}
				if dangerousBuiltins[name] {
					walkErr = fmt.Errorf("dangerous builtin %q blocked", name)
				} else {
					walkErr = fmt.Errorf("command %q not allowed", name)
				}
				return false
			}

			if _, ok := reg.Get(name); ok {
				// allow_on_any short-circuit for registered tools:
				// skip CheckFunc when all args are universally safe.
				if shellAllowedOnAny(n.Args, opts.AllowOnAny) {
					if !toolSet[name] {
						toolSet[name] = true
					}
					return true
				}
				if opts.CheckFunc != nil {
					cmdArgs := make([]string, 0, len(n.Args)-1)
					for _, a := range n.Args[1:] {
						cmdArgs = append(cmdArgs, wordSource(a))
					}
					if err := opts.CheckFunc(name, cmdArgs); err != nil {
						walkErr = err
						return false
					}
				}
				if !toolSet[name] {
					toolSet[name] = true
				}
			}

		case *syntax.Redirect:
			if err := checkRedirect(n, opts, currentDir, currentDirUncertain); err != nil {
				walkErr = err
				return false
			}
		}

		return true
	})

	if walkErr != nil {
		return nil, walkErr
	}

	names := make([]string, 0, len(toolSet))
	for n := range toolSet {
		names = append(names, n)
	}
	return &Result{ToolNames: names}, nil
}

// wordSource returns a best-effort string representation of a word.
// Unlike wordLit, it replaces variable/command substitutions with "__"
// instead of returning "". This preserves argument structure (e.g. flag
// detection like -c) while handling dynamic content gracefully.
func wordSource(w *syntax.Word) string {
	if w == nil {
		return ""
	}
	var sb strings.Builder
	for _, part := range w.Parts {
		switch p := part.(type) {
		case *syntax.Lit:
			sb.WriteString(p.Value)
		case *syntax.SglQuoted:
			sb.WriteString(p.Value)
		case *syntax.DblQuoted:
			for _, qp := range p.Parts {
				if lit, ok := qp.(*syntax.Lit); ok {
					sb.WriteString(lit.Value)
				} else {
					sb.WriteString("__")
				}
			}
		default:
			sb.WriteString("__")
		}
	}
	return sb.String()
}

// wordLit returns the literal string value of a word, or "" if the word
// contains any expansion (variable, command substitution, etc.).
// It resolves single-quoted ('foo') and double-quoted ("foo") strings
// as long as they contain only literal text (no $var or $(cmd)).
func wordLit(w *syntax.Word) string {
	if w == nil {
		return ""
	}
	var sb strings.Builder
	for _, part := range w.Parts {
		switch p := part.(type) {
		case *syntax.Lit:
			sb.WriteString(p.Value)
		case *syntax.SglQuoted:
			sb.WriteString(p.Value)
		case *syntax.DblQuoted:
			for _, qp := range p.Parts {
				lit, ok := qp.(*syntax.Lit)
				if !ok {
					return ""
				}
				sb.WriteString(lit.Value)
			}
		default:
			return ""
		}
	}
	return sb.String()
}

// isAllowed checks if a command name is allowed (builtin, local func, or in registry).
func isAllowed(name string, localFuncs map[string]bool, reg *spec.Registry) bool {
	if dangerousBuiltins[name] {
		return false
	}
	if safeBuiltins[name] {
		return true
	}
	if localFuncs[name] {
		return true
	}
	_, ok := reg.Get(name)
	return ok
}

// checkRedirect validates a redirect node.
// currentDir is the last known cd destination; uncertain is true when the
// working directory could not be tracked (e.g. after `cd $VAR`).
func checkRedirect(redir *syntax.Redirect, opts ValidateOpts, currentDir string, uncertain bool) error {
	op := redir.Op
	// Only check file-write redirections (> and >>).
	// DplOut (>&) is fd duplication (e.g. 2>&1), not a file write.
	if op != syntax.RdrOut && op != syntax.AppOut {
		return nil
	}

	target := redir.Word
	if target == nil {
		// Heredoc or no target — skip.
		return nil
	}

	path := wordLit(target)
	if path == "" {
		return fmt.Errorf("dynamic redirection blocked (variable/substitution as target)")
	}

	if !filepath.IsAbs(path) {
		if uncertain {
			return fmt.Errorf("redirection to %q blocked (working directory uncertain after dynamic cd)", path)
		}
		// Resolve relative paths against the cd-tracked directory so that
		// e.g. `cd /tmp && cmd > out.txt` correctly resolves to /tmp/out.txt.
		if currentDir != "" {
			path = filepath.Join(currentDir, path)
		}
	}

	if !spec.IsUnderWritableDir(path, opts.WritableDirs) {
		return fmt.Errorf("redirection to %q blocked (outside allowed directories)", path)
	}

	return nil
}

// shellAllowedOnAny checks if all literal args of a CallExpr are in the AllowOnAny list.
// Returns false if any arg is non-literal (variable, substitution), if there are no args
// beyond the command name, or if AllowOnAny is empty.
func shellAllowedOnAny(args []*syntax.Word, allowList []string) bool {
	if len(allowList) == 0 || len(args) <= 1 {
		return false
	}
	allowed := make(map[string]bool, len(allowList))
	for _, a := range allowList {
		allowed[a] = true
	}
	for _, w := range args[1:] {
		lit := wordLit(w)
		if lit == "" || !allowed[lit] {
			return false
		}
	}
	return true
}
