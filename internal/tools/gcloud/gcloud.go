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

	cmd := spec.NthNonFlag(args, 1, gcloudGlobalFlagsWithValue)

	// Block common write verbs when they appear as a non-flag token.
	if spec.ContainsAnyNonFlag(args, gcloudGlobalFlagsWithValue, "create", "delete", "update", "set", "unset", "enable", "disable", "deploy", "start", "stop", "restart", "rollback", "apply", "add-iam-policy-binding", "remove-iam-policy-binding", "set-iam-policy", "reset", "move", "insert", "import", "export", "patch", "remove", "resize", "suspend", "resume") {
		return spec.Deny("potentially destructive gcloud command (write verb detected)")
	}
	// "run" is a destructive verb (e.g. `composer environments run ENV -- <cli>`
	// executes an arbitrary command inside the environment) everywhere except
	// as the root command (`gcloud run <resource> <verb>`, optionally behind
	// the "alpha"/"beta" release-track prefix), where it's the Cloud Run
	// product name rather than a verb.
	runRoot := cmd
	if cmd == "alpha" || cmd == "beta" {
		runRoot = spec.NthNonFlag(args, 2, gcloudGlobalFlagsWithValue)
	}
	if runRoot != "run" && spec.ContainsAnyNonFlag(args, gcloudGlobalFlagsWithValue, "run") {
		return spec.Deny("potentially destructive gcloud command (write verb detected)")
	}
	if runRoot == "run" {
		return checkCloudRun(args, cmd)
	}

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
		if sub == "application-default" {
			// print-access-token/print-identity-token only print a token
			// (read-only); login/revoke/set/etc. write local ADC credentials.
			adcSub := spec.NthNonFlag(args, 3, gcloudGlobalFlagsWithValue)
			if adcSub == "print-access-token" || adcSub == "print-identity-token" {
				return spec.Allow()
			}
			return spec.Deny(fmt.Sprintf("gcloud auth application-default %q not allowed", adcSub))
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

// checkCloudRun validates `gcloud [alpha|beta] run <resource> <verb>` and
// the one nested shape `run jobs executions <verb>`. The verb is checked at
// its known position rather than by scanning every token: an unrecognized
// flag with a separate value (not in gcloudGlobalFlagsWithValue) leaves that
// value in the token stream, and a scan-anywhere check would treat a value
// that happens to read "list" as the verb.
func checkCloudRun(args []string, cmd string) spec.Result {
	tokens := spec.AllNonFlags(args, gcloudGlobalFlagsWithValue)
	runIdx := 0
	if cmd == "alpha" || cmd == "beta" {
		runIdx = 1
	}
	for _, i := range []int{runIdx + 2, runIdx + 3} {
		if i < len(tokens) && isVerbAllowed(tokens[i]) {
			return spec.Allow()
		}
	}
	return spec.Deny(fmt.Sprintf("no read-only verb detected in: gcloud %s", strings.Join(tokens, " ")))
}

// isVerbAllowed reports whether a positional token is a known read-only gcloud verb.
// Recognized forms:
//   - explicit verbs: list, describe, get, show, read, logs, tail, wait, search
//   - hyphenated getters: get-value, get-credentials, get-iam-policy,
//     get-ancestors, get-effective-firewalls, ... (any "get-*")
//   - hyphenated listers: list-testable-permissions, list-grantable-roles,
//     list-usable-subnets, ... (any "list-*")
//   - hyphenated describers: any "describe-*"
//   - search subcommands: search-all-resources, search-all-iam-policies
//     (any "search-*")
//   - test-iam-permissions (permission probe, read-only)
func isVerbAllowed(v string) bool {
	switch v {
	case "list", "describe", "get", "show", "read", "logs",
		"tail", "wait", "search", "test-iam-permissions":
		return true
	}
	if strings.HasPrefix(v, "get-") ||
		strings.HasPrefix(v, "list-") ||
		strings.HasPrefix(v, "describe-") ||
		strings.HasPrefix(v, "search-") {
		return true
	}
	return false
}
