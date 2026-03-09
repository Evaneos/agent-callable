package pulumi

import (
	"fmt"

	"github.com/evaneos/agent-callable/internal/spec"
)

// Pulumi runs the program (preview/up/refresh) -> potentially local side effects.
// By default only "information" commands that do not run the program are allowed.
type Tool struct{}

func New() *Tool { return &Tool{} }

func (t *Tool) Name() string { return "pulumi" }

func (t *Tool) NonInteractiveEnv() map[string]string {
	return map[string]string{
		"PULUMI_SKIP_UPDATE_CHECK": "1",
	}
}

// pulumi global flags that consume the next separate argument.
var pulumiGlobalFlagsWithValue = map[string]bool{
	"-C": true, "--cwd": true,
	"-s": true, "--stack": true,
	"--color": true,
	"-v":      true, "--verbose": true,
}

func (t *Tool) Check(args []string, _ spec.RuntimeCtx) spec.Result {
	if res, ok := spec.CheckPreamble("pulumi", args); !ok {
		return res
	}

	cmd := spec.FirstNonFlag(args, pulumiGlobalFlagsWithValue)
	if cmd == "" {
		return spec.Deny("pulumi subcommand not found")
	}

	// Explicitly block commands that run the program / modify state.
	switch cmd {
	case "up", "refresh", "destroy", "cancel", "import", "state", "new", "login", "logout":
		return spec.Deny(fmt.Sprintf("pulumi %q not allowed (executes/modifies)", cmd))
	}

	// Block flags that can expose secrets (even on read-only commands).
	if spec.ContainsFlag(args, "--show-secrets") {
		return spec.Deny("pulumi flag not allowed: --show-secrets")
	}

	switch cmd {
	case "version", "about", "whoami", "graph":
		return spec.Allow()
	case "preview":
		if spec.ContainsFlag(args, "--non-interactive") {
			return spec.Allow()
		}
		return spec.Result{Decision: spec.DecisionAllow, NonInteractiveArgs: []string{"--non-interactive"}}
	case "logs":
		return spec.Allow()
	case "stack":
		sub := spec.NthNonFlag(args, 2, pulumiGlobalFlagsWithValue)
		switch sub {
		case "ls", "history", "output":
			return spec.Allow()
		case "tag":
			subsub := spec.NthNonFlag(args, 3, pulumiGlobalFlagsWithValue)
			if subsub == "ls" || subsub == "get" {
				return spec.Allow()
			}
			return spec.Deny(fmt.Sprintf("pulumi stack tag %q not allowed", subsub))
		default:
			return spec.Deny(fmt.Sprintf("pulumi stack %q not allowed", sub))
		}
	case "config":
		sub := spec.NthNonFlag(args, 2, pulumiGlobalFlagsWithValue)
		if sub == "get" {
			return spec.Allow()
		}
		return spec.Deny(fmt.Sprintf("pulumi config %q not allowed", sub))
	case "plugin":
		sub := spec.NthNonFlag(args, 2, pulumiGlobalFlagsWithValue)
		if sub == "ls" {
			return spec.Allow()
		}
		return spec.Deny(fmt.Sprintf("pulumi plugin %q not allowed", sub))
	default:
		return spec.Deny(fmt.Sprintf("pulumi command %q not allowed", cmd))
	}
}
