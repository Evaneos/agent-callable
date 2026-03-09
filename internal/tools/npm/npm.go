package npm

import (
	"fmt"

	"github.com/evaneos/agent-callable/internal/spec"
)

// npm has a huge "exec" surface via scripts/lifecycle hooks.
// Only an allowed subset is exposed, and install only if --ignore-scripts is present.
//
// npm run is restricted to common safe scripts (lint, test, build, etc.).
var allowedNpmScript = map[string]bool{
	"test":       true,
	"lint":       true,
	"typecheck":  true,
	"type-check": true,
	"check":      true,
	"format":     true,
	"build":      true,
	"dev":        true,
	"start":      true,
}

type Tool struct{}

func New() *Tool { return &Tool{} }

func (t *Tool) Name() string { return "npm" }

func (t *Tool) NonInteractiveEnv() map[string]string {
	return map[string]string{
		"NPM_CONFIG_FUND":            "false",
		"NPM_CONFIG_AUDIT":           "false",
		"NPM_CONFIG_UPDATE_NOTIFIER": "false",
	}
}

func (t *Tool) Check(args []string, _ spec.RuntimeCtx) spec.Result {
	if res, ok := spec.CheckPreamble("npm", args); !ok {
		return res
	}

	cmd := spec.FirstNonFlag(args, nil)
	if cmd == "" {
		return spec.Deny("npm subcommand not found")
	}

	// Explicitly block execution / publish surfaces.
	switch cmd {
	case "exec", "publish", "pack", "link", "login", "logout", "adduser":
		return spec.Deny(fmt.Sprintf("npm command %q not allowed", cmd))
	}

	switch cmd {
	case "help":
		return spec.Allow()
	case "version":
		return spec.Allow()
	case "ls", "list", "view", "info", "show", "outdated", "audit":
		return spec.Allow()
	case "why", "explain":
		return spec.Allow()
	case "search":
		return spec.Allow()
	case "diff":
		return spec.Allow()
	case "root", "prefix", "bin":
		return spec.Allow()
	case "fund":
		return spec.Allow()
	case "run":
		script := spec.NthNonFlag(args, 2, nil)
		if allowedNpmScript[script] {
			return spec.Allow()
		}
		return spec.Deny(fmt.Sprintf("npm run %q not allowed", script))
	case "config":
		sub := spec.NthNonFlag(args, 2, nil)
		if sub == "list" || sub == "get" {
			return spec.Allow()
		}
		return spec.Deny(fmt.Sprintf("npm config %q not allowed", sub))
	case "pkg":
		sub := spec.NthNonFlag(args, 2, nil)
		if sub == "get" {
			return spec.Allow()
		}
		return spec.Deny(fmt.Sprintf("npm pkg %q not allowed", sub))
	case "ci", "install":
		if !spec.ContainsFlag(args, "--ignore-scripts") {
			return spec.Deny(fmt.Sprintf("npm %s requires --ignore-scripts", cmd))
		}
		return spec.Allow()
	default:
		return spec.Deny(fmt.Sprintf("npm command %q not allowed", cmd))
	}
}
