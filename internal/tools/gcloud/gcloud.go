package gcloud

import (
	"fmt"
	"strings"

	"github.com/evaneos/agent-callable/internal/spec"
)

// gcloud is extremely broad. We keep a conservative allowlist based on
// read-only verbs (list/describe/get/show/read/logs) and a few explicitly
// allowed roots (version/info/config list/auth list).
type Tool struct{}

func New() *Tool { return &Tool{} }

func (t *Tool) Name() string { return "gcloud" }

func (t *Tool) NonInteractiveEnv() map[string]string { return nil }

// gcloud global flags that consume the next separate argument.
var gcloudGlobalFlagsWithValue = map[string]bool{
	"--project": true, "--account": true,
	"--configuration": true, "--format": true,
	"--verbosity": true, "--log-http": false,
	"--impersonate-service-account": true,
	"--billing-project":             true,
	"--flags-file":                  true,
}

func (t *Tool) Check(args []string, _ spec.RuntimeCtx) spec.Result {
	if res, ok := spec.CheckPreamble("gcloud", args); !ok {
		return res
	}

	// Block common write verbs when they appear as a non-flag token.
	if spec.ContainsAnyNonFlag(args, gcloudGlobalFlagsWithValue, "create", "delete", "update", "set", "unset", "enable", "disable", "deploy", "run", "start", "stop", "restart", "rollback", "apply", "add-iam-policy-binding", "remove-iam-policy-binding", "reset", "move", "insert", "import", "export", "patch", "remove", "resize", "suspend", "resume") {
		return spec.Deny("potentially destructive gcloud command (write verb detected)")
	}

	cmd := spec.NthNonFlag(args, 1, gcloudGlobalFlagsWithValue)

	// Allowed roots.
	switch cmd {
	case "version", "info":
		return spec.Allow()
	case "config":
		sub := spec.NthNonFlag(args, 2, gcloudGlobalFlagsWithValue)
		if sub == "list" || sub == "get-value" || sub == "get" {
			return spec.Allow()
		}
		return spec.Deny(fmt.Sprintf("gcloud config %q not allowed", sub))
	case "auth":
		sub := spec.NthNonFlag(args, 2, gcloudGlobalFlagsWithValue)
		if sub == "list" {
			return spec.Allow()
		}
		return spec.Deny(fmt.Sprintf("gcloud auth %q not allowed", sub))
	}

	// Scan all positional tokens for a known read-only verb.
	tokens := spec.AllNonFlags(args, gcloudGlobalFlagsWithValue)
	for i := len(tokens) - 1; i >= 0; i-- {
		if isVerbAllowed(tokens[i]) {
			return spec.Allow()
		}
	}
	return spec.Deny(fmt.Sprintf("no read-only verb detected in: gcloud %s", strings.Join(tokens, " ")))
}

func isVerbAllowed(v string) bool {
	switch v {
	case "list", "describe", "get", "get-value", "get-credentials",
		"show", "read", "logs":
		return true
	default:
		return false
	}
}
