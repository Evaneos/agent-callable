package git

import (
	"fmt"
	"strings"

	"github.com/evaneos/agent-callable/internal/spec"
)

// Deny-by-default. Filters out accidental side effects (push, PR, apply, etc.)
// from an LLM agent. Best-effort filter, not a security boundary.
type Tool struct{}

func New() *Tool { return &Tool{} }

func (t *Tool) Name() string { return "git" }

func (t *Tool) NonInteractiveEnv() map[string]string {
	return map[string]string{
		"GIT_TERMINAL_PROMPT": "0",
		"GIT_PAGER":           "cat",
	}
}

// gitGlobalFlagsWithValue lists git global flags that consume the next
// separate argument (e.g. "-C <path>", "--git-dir <path>").
var gitGlobalFlagsWithValue = map[string]bool{
	"-C":          true,
	"--git-dir":   true,
	"--work-tree": true,
	"--namespace": true,
}

func (t *Tool) Check(args []string, _ spec.RuntimeCtx) spec.Result {
	if res, ok := spec.CheckPreamble("git", args); !ok {
		return res
	}

	// Deny the most dangerous global flags (config/exec injection).
	// Only check arguments BEFORE the subcommand (git global flags),
	// because "-c" is also a legitimate subcommand flag (e.g. git switch -c).
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			break
		}
		if !strings.HasPrefix(a, "-") {
			break // Reached the subcommand, stop scanning.
		}
		if a == "-c" || strings.HasPrefix(a, "-c=") {
			return spec.Deny("git flag not allowed: -c (config/alias injection)")
		}
		if a == "--config-env" || strings.HasPrefix(a, "--config-env=") {
			return spec.Deny("git flag not allowed: --config-env (config injection)")
		}
		if strings.HasPrefix(a, "--exec-path") {
			return spec.Deny("git flag not allowed: --exec-path")
		}
		if gitGlobalFlagsWithValue[a] {
			i++ // skip the value argument
		}
	}

	if spec.ContainsFlag(args, "--help") {
		return spec.Allow()
	}

	cmd := spec.FirstNonFlag(args, gitGlobalFlagsWithValue)
	if cmd == "" {
		return spec.Deny("git subcommand not found")
	}
	switch cmd {
	case "help":
		return spec.Allow()
	case // Inspection / read-only
		"status", "diff", "show", "log", "rev-parse", "ls-files", "cat-file", "describe",
		"blame", "grep", "reflog", "shortlog", "show-ref", "for-each-ref",
		// Plumbing / read-only investigation
		"ls-tree", "rev-list", "merge-base", "diff-tree", "name-rev",
		"cherry", "range-diff", "count-objects", "verify-commit", "verify-tag":
		return spec.Allow()
	case "fetch", "ls-remote":
		return spec.Allow()
	case "clone":
		if spec.ContainsAny(args, "-f", "--force") {
			return spec.Deny("git clone with force not allowed")
		}
		return spec.Allow()
	case "checkout":
		if spec.ContainsAny(args, "-f", "--force", "-B") {
			return spec.Deny("git checkout with force/-B not allowed")
		}
		return spec.Allow()
	case "switch":
		if spec.ContainsAny(args, "-f", "--force", "-C") {
			return spec.Deny("git switch with force/-C not allowed")
		}
		return spec.Allow()
	case "add", "mv":
		return spec.Allow()
	case "commit", "revert", "cherry-pick":
		if spec.ContainsAny(args, "--amend") {
			return spec.Deny("git commit --amend not allowed (history rewrite)")
		}
		return spec.Allow()
	case "rm":
		if spec.ContainsAny(args, "-f", "--force") {
			return spec.Deny("git rm with force not allowed")
		}
		return spec.Allow()
	case "config":
		if spec.ContainsAny(args, "--list", "-l", "--get", "--get-all", "--get-regexp", "--get-urlmatch") {
			return spec.Allow()
		}
		if spec.ContainsAny(args, "--add", "--unset", "--unset-all", "--replace-all",
			"--rename-section", "--remove-section", "--edit", "-e",
			"--global", "--system") {
			return spec.Deny("git config in write mode not allowed")
		}
		nf := spec.CountNonFlags(args, gitGlobalFlagsWithValue)
		if nf == 2 {
			return spec.Allow()
		}
		return spec.Deny("git config in write mode not allowed (use --get or --list)")
	case "branch":
		if spec.ContainsAny(args, "-D") {
			return spec.Deny("git branch -D (force delete) not allowed")
		}
		if spec.ContainsAny(args, "-f", "--force", "-M") {
			return spec.Deny("git branch with force/-M not allowed")
		}
		return spec.Allow()
	case "remote":
		sub := spec.NthNonFlag(args, 2, gitGlobalFlagsWithValue)
		if sub == "" {
			return spec.Allow()
		}
		if sub == "show" {
			return spec.Allow()
		}
		return spec.Deny(fmt.Sprintf("git remote %q not allowed", sub))
	case "tag":
		if spec.ContainsAny(args, "-l", "--list") || len(args) == 1 {
			return spec.Allow()
		}
		return spec.Deny("git tag modification not allowed")
	case "worktree":
		sub := spec.NthNonFlag(args, 2, gitGlobalFlagsWithValue)
		if sub == "" || sub == "list" {
			return spec.Allow()
		}
		if sub == "add" {
			if spec.ContainsAny(args, "-f", "--force") {
				return spec.Deny("git worktree add with force not allowed")
			}
			return spec.Allow()
		}
		return spec.Deny(fmt.Sprintf("git worktree %q not allowed", sub))
	case "stash":
		sub := spec.NthNonFlag(args, 2, gitGlobalFlagsWithValue)
		if sub == "list" || sub == "show" || sub == "apply" {
			return spec.Allow()
		}
		if sub == "" {
			return spec.Deny("bare git stash not allowed (equivalent to stash push)")
		}
		return spec.Deny(fmt.Sprintf("git stash %q not allowed (risk of stash loss)", sub))
	default:
		return spec.Deny(fmt.Sprintf("git subcommand %q not allowed", cmd))
	}
}
