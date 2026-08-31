package gh

import (
	"testing"

	"github.com/evaneos/agent-callable/internal/spectest"
)

func TestGHAllowlistBasic(t *testing.T) {
	tool := New()
	allowed := [][]string{
		{"version"},
		{"status"},
		{"auth", "status"},
		{"auth", "token"},
		{"repo", "view"},
		{"repo", "list"},
		{"repo", "clone", "OWNER/REPO"},
		{"pr", "list"},
		{"pr", "view", "123"},
		{"pr", "checkout", "123"},
		{"issue", "list"},
		{"release", "list"},
		{"run", "list"},
		{"run", "download", "123"},
		{"workflow", "list"},
		{"search", "repos", "foo"},
		{"api", "/repos/OWNER/REPO/pulls"},
		{"api", "/repos/OWNER/REPO/pulls", "--method", "GET"},
		{"api", "-XGET", "/repos/OWNER/REPO"},
		{"config", "list"},
		{"config", "get", "editor"},
		{"extension", "list"},
		{"label", "list"},
		{"cache", "list"},
		// Global flags with value argument
		{"-R", "owner/repo", "pr", "list"},
		{"--repo", "owner/repo", "issue", "view", "42"},
		{"-R", "owner/repo", "run", "list"},
		{"--jq", ".items", "api", "/repos/X/Y"},
		{"-t", "{{.title}}", "pr", "list"},
		// project read-only
		{"project", "list"},
		{"project", "view", "1"},
		// help
		{"help"},
		{"help", "pr"},
	}
	spectest.AssertAllowedBatch(t, tool, allowed)
}

func TestGHBlocksGenericAndWriteCommands(t *testing.T) {
	tool := New()
	blocked := [][]string{
		{"auth", "login"},
		{"pr", "create"},
		{"repo", "create"},
		{"release", "create"},
		{"workflow", "run"},
		{"run", "rerun", "123"},
		{"api", "/repos/X/Y", "--method", "POST"},
		{"api", "/repos/X/Y", "-X", "DELETE"},
		{"api", "/repos/X/Y", "-XPATCH"},
		{"api", "/repos/X/Y", "--method=PUT"},
		{"api", "/repos/X/Y", "-f", "key=value"},
		{"config", "set", "editor", "vim"},
		{"extension", "install", "foo"},
		{"label", "create", "bug"},
		{"cache", "delete", "--all"},
		// Global flags don't bypass sub-command checks
		{"-R", "owner/repo", "pr", "create"},
		{"--repo", "owner/repo", "repo", "create"},
		// project write
		{"project", "create"},
		{"project", "edit", "1"},
		{"project", "delete", "1"},
	}
	spectest.AssertBlockedBatch(t, tool, blocked)
}

func TestGHEdgeCases(t *testing.T) {
	tool := New()

	// === ALLOWED edge cases ===
	allowed := [][]string{
		// pr sub-commands
		{"pr", "diff", "123"},
		{"pr", "checks", "123"},
		{"pr", "status"},
		// issue sub-commands
		{"issue", "status"},
		{"issue", "view", "42", "--comments"},
		// release sub-commands
		{"release", "view", "v1.0"},
		{"release", "download", "v1.0", "-p", "*.tar.gz"},
		// run sub-commands
		{"run", "view", "123"},
		{"run", "watch", "123"},
		// workflow sub-commands
		{"workflow", "view", "ci.yml"},
		// api with HEAD method (read-only)
		{"api", "/repos/X/Y", "--method", "HEAD"},
		{"api", "/repos/X/Y", "-XHEAD"},
		{"api", "/repos/X/Y", "--method=HEAD"},
		{"api", "/repos/X/Y", "--method", "GET"},
		// api without method (defaults to GET)
		{"api", "/repos/X/Y/pulls"},
		// api graphql: query operation via -f/-F, not a REST write
		{"api", "graphql", "-f", "query=query { viewer { login } }"},
		{"api", "graphql", "-F", "query=query { viewer { login } }"},
		{"api", "graphql", "--raw-field", "query=query { viewer { login } }"},
		{"api", "graphql", "--raw-field=query=query { viewer { login } }"},
		{"api", "graphql", "-f", "owner=foo", "-f", "query=query { viewer { login } }"},
		{"api", "graphql", "-f", "query=  query { viewer { login } }"},
		{"api", "graphql", "-f", "query=Query { viewer { login } }"}, // case-insensitive "query" keyword, not "mutation"
		// "Mutation" as part of a longer identifier isn't the operation keyword
		{"api", "graphql", "-f", "query=query { addComment { clientMutationId } }"},
		// explicit method matching what graphql actually sends
		{"api", "graphql", "-X", "POST", "-f", "query=query { viewer { login } }"},
		// --preview value doesn't get mistaken for the endpoint positional
		{"api", "--preview", "nebula-preview", "/repos/X/Y"},
		// last -X wins (matches real gh/HTTP client behavior)
		{"api", "/repos/X/Y", "-X", "DELETE", "-X", "GET"},
		// search variants
		{"search", "issues", "is:open"},
		{"search", "prs", "author:me"},
		{"search", "commits", "fix"},
		{"search", "code", "func main"},
		// global flags with various positions
		{"-R", "owner/repo", "--jq", ".items", "pr", "list"},
		{"--hostname", "github.example.com", "repo", "list"},
	}
	spectest.AssertAllowedBatch(t, tool, allowed)

	// === BLOCKED edge cases ===
	blocked := [][]string{
		// api with write-implying flags, including forms glued to the flag
		// itself (pflag accepts these; a scan for the bare flag token misses them)
		{"api", "/repos/X/Y", "--field=key=value"},
		{"api", "/repos/X/Y", "--raw-field=key=value"},
		{"api", "/repos/X/Y", "-fkey=value"},
		{"api", "/repos/X/Y", "-Fkey=value"},
		{"api", "/repos/X/Y", "--field", "key=value"},
		{"api", "/repos/X/Y", "-F", "key=value"},
		{"api", "/repos/X/Y", "--input=file.json"},
		{"api", "/repos/X/Y", "--input", "file.json"},
		{"api", "/repos/X/Y", "--raw-field", "key=value"},
		{"api", "/repos/X/Y", "-f", "body=text"},
		// api with write methods
		{"api", "/repos/X/Y", "-X", "POST"},
		{"api", "/repos/X/Y", "-X", "PUT"},
		{"api", "/repos/X/Y", "-X", "PATCH"},
		{"api", "/repos/X/Y", "-XPOST"},
		{"api", "/repos/X/Y", "--method=DELETE"},
		// api graphql: mutation operation, --input, or unreadable/missing query
		{"api", "graphql", "-f", "query=mutation { addComment(input: {}) { clientMutationId } }"},
		{"api", "graphql", "-F", "query=Mutation { addComment(input: {}) { clientMutationId } }"},
		{"api", "graphql", "--raw-field=query=mutation { addComment(input: {}) { clientMutationId } }"},
		{"api", "graphql", "-f", "query=@query.graphql"},
		{"api", "graphql", "--input", "body.json"},
		{"api", "graphql", "-f", "owner=foo"},
		{"api", "graphql"},
		// mutation keyword hidden behind a leading comment, fragment, or comma
		{"api", "graphql", "-f", "query=# just a comment\nmutation { addComment(input: {}) { clientMutationId } }"},
		{"api", "graphql", "-f", "query=fragment F on X{a} mutation{addComment(input:{}){clientMutationId}}"},
		{"api", "graphql", "-f", "query=,mutation{addComment(input:{}){clientMutationId}}"},
		// operationName can select a different (mutating) operation than the
		// one the "query" field's text was reviewed against
		{"api", "graphql", "-f", "query=query A{viewer{login}} mutation B{addComment(input:{}){clientMutationId}}", "-f", "operationName=B"},
		{"api", "graphql", "-f", "query=query{viewer{login}}", "-f", "operationName=anything"},
		// -X/--preview value "graphql" must not be mistaken for the endpoint:
		// the real endpoint here is a REST path with a write method/field
		{"api", "-X", "graphql", "/repos/X/Y", "-X", "DELETE", "-f", "query=query{viewer{login}}"},
		{"api", "--preview", "graphql", "/repos/X/Y/issues/comments/1", "-X", "DELETE"},
		// explicit non-GET/POST method on the real graphql endpoint
		{"api", "graphql", "-X", "DELETE", "-f", "query=query { viewer { login } }"},
		// pr write operations
		{"pr", "merge", "123"},
		{"pr", "close", "123"},
		{"pr", "comment", "123", "--body", "text"},
		{"pr", "edit", "123", "--title", "new"},
		{"pr", "review", "123", "--approve"},
		{"pr", "reopen", "123"},
		{"pr", "ready", "123"},
		// issue write operations
		{"issue", "create", "--title", "bug"},
		{"issue", "close", "42"},
		{"issue", "comment", "42"},
		{"issue", "edit", "42"},
		{"issue", "reopen", "42"},
		{"issue", "transfer", "42", "other/repo"},
		// repo write operations
		{"repo", "fork"},
		{"repo", "delete", "owner/repo"},
		{"repo", "archive", "owner/repo"},
		{"repo", "rename", "old", "new"},
		{"repo", "edit"},
		// release write
		{"release", "create", "v2.0"},
		{"release", "edit", "v1.0"},
		{"release", "delete", "v1.0"},
		{"release", "upload", "v1.0", "file.tar.gz"},
		// run/workflow write
		{"run", "cancel", "123"},
		{"run", "rerun", "123"},
		{"run", "delete", "123"},
		{"workflow", "run", "ci.yml"},
		{"workflow", "enable", "ci.yml"},
		{"workflow", "disable", "ci.yml"},
		// extension write
		{"extension", "install", "owner/ext"},
		{"extension", "create", "my-ext"},
		{"extension", "upgrade", "ext"},
		{"extension", "remove", "ext"},
		// label write
		{"label", "create", "enhancement"},
		{"label", "edit", "bug"},
		{"label", "delete", "bug"},
		// cache write
		{"cache", "delete", "key"},
		// config write
		{"config", "set", "pager", "less"},
		// auth write
		{"auth", "login"},
		{"auth", "logout"},
		{"auth", "refresh"},
		{"auth", "setup-git"},
		// codespace/gist/secret (not in allowlist)
		{"codespace", "create"},
		{"gist", "create", "file.txt"},
		{"secret", "set", "TOKEN"},
		// project write
		{"project", "close", "1"},
		{"project", "field-create", "Status"},
		{"project", "item-add", "1"},
		// ssh-key
		{"ssh-key", "add", "key.pub"},
		// gpg-key
		{"gpg-key", "add", "key.gpg"},
	}
	spectest.AssertBlockedBatch(t, tool, blocked)
}

func TestGHEmptyAndBareArgs(t *testing.T) {
	tool := New()
	spectest.AssertBlocked(t, tool, []string{})
	spectest.AssertBlocked(t, tool, []string{"-R", "owner/repo"})
}
