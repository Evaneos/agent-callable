package git

import (
	"testing"

	"github.com/evaneos/agent-callable/internal/spectest"
)

func TestGitAllowlistBasic(t *testing.T) {
	tool := New()

	// In a non-git directory, we should not be blocked just for "alias".
	// (best-effort: no .git/config => no aliases detected)
	allowed := [][]string{
		{"status"},
		{"diff"},
		{"log", "--oneline", "-n", "5"},
		{"show", "HEAD"},
		{"rev-parse", "HEAD"},
		{"ls-files"},
		{"cat-file", "-p", "HEAD"},
		{"remote", "-v"},
		{"fetch", "--all"},
		{"grep", "TODO"},
		{"blame", "README.md"},
		{"reflog"},
		{"tag", "-l"},
		{"stash", "list"},
		{"checkout", "main"},
		{"switch", "main"},
		{"clone", "https://example.com/repo.git"},
		{"worktree", "list"},
		{"worktree", "add", "../feature-branch", "feature-branch"},
		// Plumbing / investigation
		{"ls-tree", "HEAD"},
		{"rev-list", "--count", "HEAD"},
		{"merge-base", "main", "feature"},
		{"diff-tree", "--no-commit-id", "-r", "HEAD"},
		{"name-rev", "HEAD"},
		{"cherry", "main"},
		{"range-diff", "main...feature"},
		{"count-objects", "-v"},
		{"verify-commit", "HEAD"},
		{"verify-tag", "v1.0"},
		// Config read-only
		{"config", "--list"},
		{"config", "-l"},
		{"config", "--get", "user.email"},
		{"config", "--get-all", "remote.origin.url"},
		{"config", "--get-regexp", "remote.*"},
		{"config", "user.email"},
		// Global flags with value argument
		{"-C", "/some/path", "log", "--oneline", "-20"},
		{"-C", "/some/path", "status"},
		{"--git-dir=/some/.git", "log"},
		{"--git-dir", "/some/.git", "log"},
		{"--work-tree", "/some/path", "status"},
		{"--work-tree=/some/path", "status"},
		{"-C", "/some/path", "-C", "../other", "diff"},
		// Local writes (non-destructive)
		{"add", "file.go"},
		{"add", "-A"},
		{"add", "-p", "file.go"},
		{"commit", "-m", "fix: something"},
		{"commit", "-m", "fix: something", "--no-verify"},
		{"-C", "/some/path", "commit", "-m", "msg"},
		{"mv", "old.go", "new.go"},
		{"revert", "HEAD"},
		{"revert", "--no-commit", "HEAD~3..HEAD"},
		{"cherry-pick", "abc123"},
		{"cherry-pick", "--no-commit", "abc123"},
		{"rm", "--cached", "file.go"},
		{"rm", "--cached", "-r", "dir/"},
		{"rm", "file.go"},
		{"rm", "-r", "dir/"},
		{"stash", "show"},
		{"stash", "show", "stash@{0}"},
		{"stash", "apply"},
		{"stash", "apply", "stash@{1}"},
		// help
		{"help"},
		{"help", "commit"},
		{"--help"},
	}
	spectest.AssertAllowedBatch(t, tool, allowed)
}

func TestGitBlocksDangerousFlagsAndWrites(t *testing.T) {
	tool := New()
	blocked := [][]string{
		{"-c", "alias.status=!rm -rf /", "status"},
		{"--config-env=GIT_CONFIG_PARAMETERS", "status"},
		{"commit", "--amend", "-m", "x"},
		{"push"},
		{"pull"},
		{"merge", "main"},
		{"rebase", "main"},
		{"checkout", "-f", "main"},
		{"switch", "--force", "main"},
		{"reset", "--hard"},
		{"clean", "-fdx"},
		{"branch", "-D", "main"},
		{"remote", "add", "origin", "x"},
		{"tag", "v1.2.3"},
		{"stash", "push", "-m", "x"},
		{"worktree", "remove", "feature-branch"},
		{"worktree", "move", "old", "new"},
		{"worktree", "prune"},
		{"worktree", "add", "-f", "../force-branch"},
		// Config write
		{"config", "user.email", "foo@bar.com"},
		{"config", "--global", "user.email", "foo@bar.com"},
		{"config", "--add", "remote.origin.url", "x"},
		{"config", "--unset", "user.email"},
		{"config", "--edit"},
		// -C doesn't bypass sub-command checks
		{"-C", "/some/path", "push"},
		{"-C", "/some/path", "commit", "--amend", "-m", "x"},
		// Destructive stash operations
		{"stash", "drop"},
		{"stash", "pop"},
		{"stash", "clear"},
		// rm with force
		{"rm", "-f", "file.go"},
		{"rm", "--force", "file.go"},
	}
	spectest.AssertBlockedBatch(t, tool, blocked)
}

// Edge cases: unusual syntaxes an LLM might accidentally generate.
func TestGitEdgeCases(t *testing.T) {
	tool := New()

	// === ALLOWED edge cases ===
	allowed := [][]string{
		// branch list modes
		{"branch"},
		{"branch", "-a"},
		{"branch", "-r"},
		{"branch", "--list"},
		{"branch", "-v"},
		{"branch", "--contains", "HEAD"},
		{"branch", "-d", "feature-branch"},
		{"branch", "--delete", "feature-branch"},
		// checkout/switch normal usage
		{"checkout", "-b", "new-branch"},
		{"checkout", "--", "file.go"},
		{"switch", "-c", "new-branch"},
		{"switch", "--create", "new-branch"},
		// commit variants (not --amend)
		{"commit", "-m", "msg", "--allow-empty"},
		{"commit", "--fixup", "HEAD~1"},
		{"commit", "-a", "-m", "msg"},
		// stash with flags
		{"stash", "show", "-p"},
		{"stash", "show", "--stat"},
		{"stash", "apply", "--index"},
		// log with complex formatting
		{"log", "--pretty=format:%H %s", "--since=2024-01-01"},
		{"log", "--graph", "--all", "--oneline"},
		// diff with paths after --
		{"diff", "HEAD~1", "--", "src/"},
		// config edge cases
		{"config", "--get-urlmatch", "http", "https://example.com"},
		// remote show with flags
		{"remote", "show", "origin"},
		{"remote", "-v"},
		// clone with options
		{"clone", "--depth", "1", "https://example.com/repo"},
		{"clone", "--single-branch", "--branch", "main", "url"},
		// tag list modes
		{"tag"},
		{"tag", "-l", "v1.*"},
		{"tag", "--list"},
		// worktree add without force
		{"worktree", "add", "-b", "feature", "../feature"},
		// global flags interleaved
		{"-C", "/path", "--git-dir", "/path/.git", "status"},
		{"--namespace", "ns", "-C", "/path", "log"},
	}
	spectest.AssertAllowedBatch(t, tool, allowed)

	// === BLOCKED edge cases ===
	blocked := [][]string{
		// checkout -B (force-create branch)
		{"checkout", "-B", "main"},
		{"checkout", "-B", "feature", "origin/feature"},
		// switch -C (force-create branch)
		{"switch", "-C", "main"},
		{"switch", "-C", "feature", "origin/feature"},
		// branch with force/rename
		{"branch", "-f", "main", "HEAD~1"},
		{"branch", "--force", "main", "HEAD~1"},
		{"branch", "-M", "old-name", "new-name"},
		// bare stash (equals stash push)
		{"stash"},
		// stash save (deprecated push alias)
		{"stash", "save", "wip message"},
		// stash push explicitly
		{"stash", "push"},
		{"stash", "push", "-m", "wip"},
		// stash branch (creates a branch from stash)
		{"stash", "branch", "new-branch"},
		// commit --amend with global flag
		{"-C", "/path", "commit", "--amend"},
		// rebase (not allowed at all)
		{"rebase", "--onto", "main"},
		{"rebase", "-i", "HEAD~3"},
		// reset (not allowed)
		{"reset", "HEAD~1"},
		{"reset", "--soft", "HEAD~1"},
		{"reset", "--mixed", "HEAD~1"},
		// rm with force
		{"rm", "-f", "file.go"},
		{"rm", "--force", "-r", "dir/"},
		// push variants
		{"push", "--force"},
		{"push", "origin", "main"},
		{"push", "-u", "origin", "feature"},
		// tag creation (no -l/--list)
		{"tag", "v1.0.0"},
		{"tag", "-a", "v1.0.0", "-m", "release"},
		{"tag", "-d", "v1.0.0"},
		// remote mutations
		{"remote", "add", "upstream", "url"},
		{"remote", "remove", "origin"},
		{"remote", "rename", "origin", "old-origin"},
		{"remote", "set-url", "origin", "new-url"},
		// config write with 3 non-flags
		{"config", "user.email", "foo@bar.com"},
		{"config", "--global", "user.email", "foo@bar.com"},
		{"config", "--system", "core.autocrlf", "true"},
		// merge/pull (not allowed)
		{"merge", "--no-ff", "feature"},
		{"pull", "--rebase"},
		// submodule (not in allowlist)
		{"submodule", "update", "--init"},
		// bisect (not in allowlist)
		{"bisect", "start"},
		// format-patch (not in allowlist)
		{"format-patch", "HEAD~3"},
		// am (apply mailbox - not allowed)
		{"am", "patch.mbox"},
		// gc (garbage collect)
		{"gc"},
		// filter-branch (rewrite history)
		{"filter-branch"},
		// worktree with force
		{"worktree", "add", "--force", "../forced"},
		{"worktree", "remove", "../old"},
		{"worktree", "prune"},
	}
	spectest.AssertBlockedBatch(t, tool, blocked)
}

func TestGitEmptyAndBareArgs(t *testing.T) {
	tool := New()
	spectest.AssertBlocked(t, tool, []string{})
	spectest.AssertBlocked(t, tool, []string{"--git-dir", "/path"})
	spectest.AssertBlocked(t, tool, []string{"--"})
}
