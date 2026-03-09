package shell

import (
	"fmt"
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

			if !isAllowed(name, localFuncs, reg) {
				if dangerousBuiltins[name] {
					walkErr = fmt.Errorf("dangerous builtin %q blocked", name)
				} else {
					walkErr = fmt.Errorf("command %q not allowed", name)
				}
				return false
			}

			if _, ok := reg.Get(name); ok {
				if opts.CheckFunc != nil {
					cmdArgs := make([]string, 0, len(n.Args)-1)
					for _, a := range n.Args[1:] {
						cmdArgs = append(cmdArgs, wordLit(a))
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
			if err := checkRedirect(n, opts); err != nil {
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
func checkRedirect(redir *syntax.Redirect, opts ValidateOpts) error {
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

	if !spec.IsUnderWritableDir(path, opts.WritableDirs) {
		return fmt.Errorf("redirection to %q blocked (outside allowed directories)", path)
	}

	return nil
}
