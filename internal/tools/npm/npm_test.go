package npm

import (
	"testing"

	"github.com/evaneos/agent-callable/internal/spectest"
)

func TestNpmAllowlist(t *testing.T) {
	tool := New()
	allowed := [][]string{
		{"ls"},
		{"view", "react"},
		{"info", "react"},
		{"show", "react"},
		{"outdated"},
		{"audit"},
		{"install", "--ignore-scripts"},
		{"ci", "--ignore-scripts"},
		{"version"},
		{"why", "react"},
		{"explain", "react"},
		{"search", "express"},
		{"diff"},
		{"root"},
		{"prefix"},
		{"bin"},
		{"pkg", "get", "name"},
		// help
		{"help"},
		{"help", "install"},
		// fund / config read
		{"fund"},
		{"config", "list"},
		{"config", "get", "registry"},
		// npm run safe scripts
		{"run", "test"},
		{"run", "lint"},
		{"run", "typecheck"},
		{"run", "type-check"},
		{"run", "check"},
		{"run", "format"},
		{"run", "build"},
		{"run", "dev"},
		{"run", "start"},
	}
	spectest.AssertAllowedBatch(t, tool, allowed)
}

func TestNpmBlocklist(t *testing.T) {
	tool := New()
	blocked := [][]string{
		{"test"},
		{"exec", "node", "-e", "1"},
		{"publish"},
		{"pack"},
		{"link"},
		{"login"},
		{"logout"},
		{"adduser"},
		// install/ci without --ignore-scripts
		{"install"},
		{"ci"},
		{"install", "react"},
		{"ci", "--production"},
		{"pkg", "set", "name=foo"},
		// config write
		{"config", "set", "registry", "https://evil.com"},
		{"config", "delete", "registry"},
		// npm run unsafe scripts
		{"run", "deploy"},
		{"run", "postinstall"},
		{"run", "preinstall"},
		{"run"},
		// unknown commands
		{"whatever"},
	}
	spectest.AssertBlockedBatch(t, tool, blocked)
}

func TestNpmReadOnlyNoNonInteractiveArgs(t *testing.T) {
	tool := New()
	cases := [][]string{
		{"ls"},
		{"version"},
		{"audit"},
		{"outdated"},
		{"help"},
		{"run", "test"},
		{"install", "--ignore-scripts"},
	}
	spectest.AssertNoNonInteractiveArgsBatch(t, tool, cases)
}

func TestNpmEmptyArgs(t *testing.T) {
	tool := New()
	spectest.AssertBlocked(t, tool, []string{})
}
