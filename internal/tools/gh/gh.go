package gh

import (
	"strings"

	"github.com/evaneos/agent-callable/internal/spec"
)

type Tool struct{}

func New() *Tool { return &Tool{} }

func (t *Tool) Name() string { return "gh" }

func (t *Tool) NonInteractiveEnv() map[string]string {
	return map[string]string{
		"GH_PROMPT_DISABLED": "1",
		"GH_PAGER":           "cat",
	}
}

// ghSpec handles the standard allowlist + subcommand validation.
// Commands not listed here (and not handled as special cases below)
// are denied by default.
var ghSpec = spec.NewConfigTool(spec.ConfigToolOpts{
	Name: "gh",
	Allowed: []string{
		"version", "status", "help", "search",
		"auth", "config", "repo", "pr", "issue", "release",
		"run", "workflow", "extension", "label", "cache", "project",
	},
	FlagsWithValue: []string{
		"-R", "--repo", "-q", "--jq", "-t", "--template", "--hostname",
	},
	Subcommands: map[string][]string{
		"auth":      {"status", "token"},
		"config":    {"list", "get"},
		"repo":      {"view", "list", "clone"},
		"pr":        {"view", "list", "status", "checks", "diff", "checkout"},
		"issue":     {"view", "list", "status"},
		"release":   {"view", "list", "download"},
		"run":       {"view", "list", "watch", "download"},
		"workflow":  {"view", "list"},
		"extension": {"list"},
		"label":     {"list"},
		"cache":     {"list"},
		"project":   {"list", "view"},
	},
})

func (t *Tool) Check(args []string, ctx spec.RuntimeCtx) spec.Result {
	if res, ok := spec.CheckPreamble("gh", args); !ok {
		return res
	}

	cmd := spec.FirstNonFlag(args, ghSpec.FlagsWithValueMap())

	// api needs custom validation (write-method detection).
	if cmd == "api" {
		if containsWriteMethod(args) {
			return spec.Deny("gh api with write method not allowed (use GET by default)")
		}
		return spec.Allow()
	}

	return ghSpec.Check(args, ctx)
}

// containsWriteMethod detects write HTTP methods in gh api args.
func containsWriteMethod(args []string) bool {
	for i, a := range args {
		if a == "--" {
			break
		}
		if (a == "--method" || a == "-X") && i+1 < len(args) {
			m := strings.ToUpper(args[i+1])
			if m != "GET" && m != "HEAD" {
				return true
			}
		}
		if strings.HasPrefix(a, "--method=") {
			m := strings.ToUpper(a[len("--method="):])
			if m != "GET" && m != "HEAD" {
				return true
			}
		}
		if strings.HasPrefix(a, "-X") && len(a) > 2 {
			m := strings.ToUpper(a[2:])
			if m != "GET" && m != "HEAD" {
				return true
			}
		}
		if a == "--input" || strings.HasPrefix(a, "--input=") ||
			a == "-f" || a == "--raw-field" ||
			a == "-F" || a == "--field" {
			return true
		}
	}
	return false
}
