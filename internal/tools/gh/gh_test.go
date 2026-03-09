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
		// Note: "gh api graphql -f query=..." is blocked by -f (acceptable false positive).
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
		// api with write-implying flags
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
