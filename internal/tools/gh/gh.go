package gh

import (
	"regexp"
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

// ghApiFlagsWithValue lists every `gh api` flag that consumes a separate
// argument. Endpoint detection (NthNonFlag position 2) must skip all of
// them, or a flag's own value (e.g. "graphql" passed to -X/--preview) gets
// mistaken for the endpoint positional and the request is checked against
// the wrong branch entirely.
var ghApiFlagsWithValue = map[string]bool{
	"-X": true, "--method": true,
	"-H": true, "--header": true,
	"-p": true, "--preview": true,
	"--cache": true,
	"--input": true,
	"-f":      true, "--raw-field": true,
	"-F": true, "--field": true,
	"-q": true, "--jq": true,
	"-t": true, "--template": true,
	"--hostname": true,
}

func (t *Tool) Check(args []string, ctx spec.RuntimeCtx) spec.Result {
	if res, ok := spec.CheckPreamble("gh", args); !ok {
		return res
	}

	cmd := spec.FirstNonFlag(args, ghSpec.FlagsWithValueMap())

	// api needs custom validation (write-method detection).
	if cmd == "api" {
		endpoint := spec.NthNonFlag(args, 2, ghApiFlagsWithValue)
		if endpoint == "graphql" {
			// GraphQL always goes over POST and passes its query document as
			// a -f/-F field (there's no other way), so checkGraphQL validates
			// it by inspecting the operation type of the document instead.
			return checkGraphQL(args)
		}
		if containsWriteMethod(args) {
			return spec.Deny("gh api with write method not allowed (use GET by default)")
		}
		return spec.Allow()
	}

	return ghSpec.Check(args, ctx)
}

// mutationOrSubscriptionRe matches the GraphQL "mutation" or "subscription"
// keyword as a whole word, case-insensitively, anywhere in the document —
// not just at the start, since a document can lead with a comment, a
// fragment definition, or an ignored comma before its operation keyword.
// Word-boundary matching means "clientMutationId" doesn't match.
var mutationOrSubscriptionRe = regexp.MustCompile(`(?i)\b(mutation|subscription)\b`)

// checkGraphQL validates `gh api graphql` calls by inspecting the GraphQL
// document passed via -f/--raw-field or -F/--field query=<document>.
func checkGraphQL(args []string) spec.Result {
	if spec.ContainsFlag(args, "--input") {
		return spec.Deny("gh api graphql --input not allowed (query body not readable statically)")
	}
	if m, ok := explicitMethod(args); ok && m != "GET" && m != "POST" {
		return spec.Deny("gh api graphql: unexpected --method/-X (graphql always POSTs)")
	}

	var query string
	var queryFound, hasOperationName bool
	graphqlFields(args, func(key, value string) {
		switch key {
		case "operationName":
			hasOperationName = true
		case "query":
			if !queryFound && !strings.HasPrefix(value, "@") {
				query = value
				queryFound = true
			}
		}
	})

	// operationName picks which of a document's several named operations
	// actually runs, so a reviewed "query" field alone doesn't prove which
	// one executes.
	if hasOperationName {
		return spec.Deny("gh api graphql: operationName not allowed (may select a different operation than the one reviewed)")
	}
	if !queryFound {
		return spec.Deny("gh api graphql: query field not found or not readable statically (e.g. @file)")
	}
	if mutationOrSubscriptionRe.MatchString(query) {
		return spec.Deny("gh api graphql mutation/subscription not allowed")
	}
	return spec.Allow()
}

// graphqlFields calls fn for every key=value field passed via
// -f/--raw-field or -F/--field, stopping at "--".
func graphqlFields(args []string, fn func(key, value string)) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			return
		}
		var kv string
		switch {
		case a == "-f" || a == "--raw-field" || a == "-F" || a == "--field":
			if i+1 >= len(args) {
				return
			}
			kv = args[i+1]
			i++
		case strings.HasPrefix(a, "--raw-field="):
			kv = a[len("--raw-field="):]
		case strings.HasPrefix(a, "--field="):
			kv = a[len("--field="):]
		default:
			continue
		}
		key, value, found := strings.Cut(kv, "=")
		if !found {
			continue
		}
		fn(key, value)
	}
}

// explicitMethod returns the uppercased HTTP method from the last
// --method/-X flag in args (gh, like real HTTP clients, lets a later flag
// override an earlier one), and whether one was present at all.
func explicitMethod(args []string) (string, bool) {
	method, found := "", false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			break
		}
		switch {
		case (a == "--method" || a == "-X") && i+1 < len(args):
			method, found = strings.ToUpper(args[i+1]), true
			i++
		case strings.HasPrefix(a, "--method="):
			method, found = strings.ToUpper(a[len("--method="):]), true
		case strings.HasPrefix(a, "-X") && len(a) > 2:
			method, found = strings.ToUpper(a[2:]), true
		}
	}
	return method, found
}

// hasFieldOrInputFlag reports whether args pass a REST body via -f/-F or
// --input — for a plain REST call this always implies a write (gh itself
// switches to POST once one is present). Covers every form pflag accepts:
// separate value ("-f name=x"), "=" form ("--field=name=x"), and a short
// flag's value glued directly to it ("-fname=x").
func hasFieldOrInputFlag(args []string) bool {
	for _, a := range args {
		if a == "--" {
			break
		}
		switch {
		case a == "--input", strings.HasPrefix(a, "--input="),
			a == "-f", a == "-F", a == "--raw-field", a == "--field",
			strings.HasPrefix(a, "--raw-field="), strings.HasPrefix(a, "--field="):
			return true
		case (strings.HasPrefix(a, "-f") || strings.HasPrefix(a, "-F")) && len(a) > 2:
			return true
		}
	}
	return false
}

// containsWriteMethod detects write HTTP methods in gh api args.
func containsWriteMethod(args []string) bool {
	if m, ok := explicitMethod(args); ok && m != "GET" && m != "HEAD" {
		return true
	}
	return hasFieldOrInputFlag(args)
}
