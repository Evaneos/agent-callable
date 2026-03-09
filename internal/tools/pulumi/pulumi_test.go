package pulumi

import (
	"testing"

	"github.com/evaneos/agent-callable/internal/spectest"
)

func TestPulumiAllowlistBasic(t *testing.T) {
	tool := New()
	allowed := [][]string{
		{"version"},
		{"about"},
		{"whoami"},
		{"preview", "--non-interactive"},
		{"stack", "ls"},
		{"stack", "history"},
		{"stack", "output"},
		{"stack", "tag", "ls"},
		{"stack", "tag", "get", "env"},
		{"logs"},
		{"graph"},
		{"config", "get", "foo"},
		{"plugin", "ls"},
		// Global flags with value argument
		{"-s", "prod", "stack", "ls"},
		{"--stack", "prod", "stack", "output"},
		{"-C", "/some/path", "version"},
		{"--cwd", "/some/path", "stack", "history"},
		{"-s", "dev", "-C", "/project", "config", "get", "foo"},
	}
	spectest.AssertAllowedBatch(t, tool, allowed)
}

func TestPulumiBlocksProgramExecutionAndSecrets(t *testing.T) {
	tool := New()
	blocked := [][]string{
		{"up"},
		{"destroy"},
		{"refresh"},
		// {"preview"} is now allowed (--non-interactive injected via NonInteractiveArgs)
		{"stack", "select", "x"},
		{"config", "set", "x", "y"},
		{"stack", "ls", "--show-secrets"},
		{"config", "get", "foo", "--show-secrets"},
		{"stack", "tag", "set", "env", "prod"},
		{"stack", "export"},
		{"stack", "output", "--show-secrets"},
		// Global flags don't bypass sub-command checks
		{"-s", "prod", "up"},
		{"-C", "/path", "destroy"},
	}
	spectest.AssertBlockedBatch(t, tool, blocked)
}

func TestPulumiEdgeCases(t *testing.T) {
	tool := New()

	// === ALLOWED edge cases ===
	allowed := [][]string{
		// preview with non-interactive
		{"preview", "--non-interactive", "--diff"},
		{"preview", "--non-interactive", "--json"},
		// stack read variants
		{"stack", "ls", "--all"},
		{"stack", "ls", "--json"},
		{"stack", "history", "--page-size", "10"},
		{"stack", "output", "--json"},
		{"stack", "output", "mykey"},
		// stack tag read
		{"stack", "tag", "ls"},
		{"stack", "tag", "get", "environment"},
		// config get
		{"config", "get", "mykey", "--json"},
		// plugin ls
		{"plugin", "ls", "--json"},
		// about/whoami
		{"about", "--json"},
		{"whoami", "--verbose"},
		// preview without --non-interactive (injected via NonInteractiveArgs)
		{"preview"},
		{"preview", "--diff"},
		{"preview", "--diff", "--stack", "mystack"},
		// Global flags interleaved
		{"-s", "prod", "-C", "/path", "stack", "ls"},
		{"--stack", "dev", "--color", "never", "version"},
	}
	spectest.AssertAllowedBatch(t, tool, allowed)

	// === BLOCKED edge cases ===
	blocked := [][]string{
		// Destructive commands
		{"up", "--yes"},
		{"up", "--non-interactive"},
		{"destroy", "--yes"},
		{"refresh", "--yes"},
		{"cancel"},
		// import/state (state manipulation)
		{"import", "aws:s3/bucket:Bucket", "mybucket"},
		{"state", "delete", "urn:pulumi:stack::project::type::name"},
		{"state", "unprotect", "urn"},
		// new/login/logout
		{"new", "typescript"},
		{"login"},
		{"logout"},
		// --show-secrets on various commands
		{"stack", "output", "--show-secrets"},
		{"stack", "ls", "--show-secrets"},
		{"config", "get", "secret-key", "--show-secrets"},
		{"stack", "history", "--show-secrets"},
		// stack write operations
		{"stack", "select", "prod"},
		{"stack", "init", "new-stack"},
		{"stack", "rm", "old-stack"},
		{"stack", "rename", "new-name"},
		{"stack", "export"},
		{"stack", "import"},
		// stack tag write
		{"stack", "tag", "set", "env", "prod"},
		{"stack", "tag", "rm", "env"},
		// config write
		{"config", "set", "key", "value"},
		{"config", "set-all"},
		{"config", "rm", "key"},
		{"config", "refresh"},
		// plugin write
		{"plugin", "install", "resource", "aws"},
		{"plugin", "rm", "resource", "aws"},
		// Unknown commands
		{"watch"},
		{"console"},
		{"org"},
		// Global flags don't bypass
		{"-s", "prod", "destroy", "--yes"},
		{"-C", "/path", "--stack", "dev", "up"},
	}
	spectest.AssertBlockedBatch(t, tool, blocked)
}

func TestPulumiPreviewInjectsNonInteractive(t *testing.T) {
	tool := New()
	// Without --non-interactive: NonInteractiveArgs should contain it.
	spectest.AssertNonInteractiveArgs(t, tool, []string{"preview", "--diff"}, []string{"--non-interactive"})
	// With --non-interactive already present: NonInteractiveArgs should be nil.
	spectest.AssertNonInteractiveArgs(t, tool, []string{"preview", "--non-interactive", "--diff"}, nil)
}

func TestPulumiReadOnlyNoNonInteractiveArgs(t *testing.T) {
	tool := New()
	cases := [][]string{
		{"version"},
		{"about"},
		{"whoami"},
		{"logs"},
		{"graph"},
		{"stack", "ls"},
		{"config", "get", "foo"},
		{"plugin", "ls"},
	}
	spectest.AssertNoNonInteractiveArgsBatch(t, tool, cases)
}

func TestPulumiPreviewWithGlobalFlags(t *testing.T) {
	tool := New()
	// preview with global flags should still inject --non-interactive.
	spectest.AssertNonInteractiveArgs(t, tool, []string{"-s", "prod", "preview", "--diff"}, []string{"--non-interactive"})
}

func TestPulumiEmptyAndBareArgs(t *testing.T) {
	tool := New()
	spectest.AssertBlocked(t, tool, []string{})
	spectest.AssertBlocked(t, tool, []string{"-s", "prod"})
}
