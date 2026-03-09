package kubectl

import (
	"strings"
	"testing"

	"github.com/evaneos/agent-callable/internal/spec"
	"github.com/evaneos/agent-callable/internal/spectest"
)

func TestBasicAllowlist(t *testing.T) {
	tool := New()
	allowed := [][]string{
		{"get", "pods"},
		{"describe", "deployment", "nginx"},
		{"logs", "my-pod", "-f", "--tail=100"},
		{"top", "nodes"},
		{"explain", "pods"},
		{"api-resources"},
		{"api-versions"},
		{"cluster-info"},
		{"version", "--client"},
		{"events", "-n", "kube-system"},
		{"wait", "--for=condition=Ready", "pod/nginx"},
		{"diff", "-f", "file.yaml"},
		{"config", "view"},
		{"config", "get-contexts"},
		{"config", "get-clusters"},
		{"config", "get-users"},
		{"config", "current-context"},
		{"config", "use-context", "production"},
		{"auth", "can-i", "get", "pods"},
		{"auth", "whoami"},
		{"rollout", "status", "deployment/nginx"},
		{"rollout", "history", "deployment/nginx"},
		{"krew", "list"},
		{"krew", "search"},
		{"krew", "search", "ctx"},
		{"krew", "info", "ctx"},
	}
	spectest.AssertAllowedBatch(t, tool, allowed)
}

func TestBlockedDangerousVerbs(t *testing.T) {
	tool := New()
	blocked := [][]string{
		{"delete", "pod", "x"},
		{"apply", "-f", "x.yaml"},
		{"exec", "pod/x", "--", "sh"},
		{"port-forward", "pod/x", "8080:80"},
		{"proxy"},
		{"debug", "pod/x"},
		{"alpha"},
		{"plugin", "list"},
		{"krew", "install", "ctx"},
		{"krew", "update"},
		{"krew", "upgrade"},
		{"krew", "uninstall", "ctx"},
		{"help"},
		{"completion", "bash"},
	}
	spectest.AssertBlockedBatch(t, tool, blocked)
}

func TestSecretsMetadataAllowed(t *testing.T) {
	tool := New()
	tests := [][]string{
		{"get", "secrets"},
		{"get", "secret"},
		{"get", "secrets", "-A"},
		{"get", "secrets", "-n", "kube-system"},
		{"describe", "secret", "my-secret"},
		{"get", "secrets", "-o", "name"},
		{"get", "secret", "my-secret", "-o", "name"},
		{"get", "secrets", "-o", "wide"},
		{"--context=prod", "get", "secrets"},
	}
	spectest.AssertAllowedBatch(t, tool, tests)
}

func TestSecretsValuesBlocked(t *testing.T) {
	tool := New()
	tests := [][]string{
		{"get", "secrets", "-o", "yaml"},
		{"get", "secrets", "-o", "json"},
		{"get", "secret", "my-secret", "-o", "yaml"},
		{"get", "secrets", "-ojson"},
		{"get", "secrets", "-oyaml"},
		{"get", "secrets", "--output=yaml"},
		{"get", "secret/my-secret", "-o", "jsonpath={.data}"},
		{"get", "secrets", "-o", "go-template={{.data}}"},
		{"get", "secrets", "-o", "custom-columns=DATA:.data"},
		{"get", "--raw", "/api/v1/secrets"},
		{"get", "--raw=/api/v1/namespaces/default/secrets"},
		{"get", "pods,secrets", "-o", "yaml"},
		{"get", "secrets", "-o=yaml"},
		{"get", "secret/my-secret", "--output", "json"},
	}
	spectest.AssertBlockedBatch(t, tool, tests)
}

func TestControlCharsBlocked(t *testing.T) {
	tool := New()
	spectest.AssertBlocked(t, tool, []string{"get\x00delete", "pods"})
	spectest.AssertBlocked(t, tool, []string{"get", "pods\x7f"})
}

func TestCaseSensitivityBlocked(t *testing.T) {
	tool := New()
	// kubectl is case-sensitive: we block anything that doesn't exactly match the allowlist.
	spectest.AssertBlocked(t, tool, []string{"GET", "pods"})
	spectest.AssertBlocked(t, tool, []string{"Get", "pods"})
	spectest.AssertBlocked(t, tool, []string{"CONFIG", "view"})
	spectest.AssertBlocked(t, tool, []string{"config", "VIEW"})
}

func TestKubectlEdgeCases(t *testing.T) {
	tool := New()
	// only flags, no positional -> allowed (harmless, kubectl shows help)
	spectest.AssertAllowed(t, tool, []string{"-n", "default"})
	// config without subcommand -> blocked (requires subcommand)
	spectest.AssertBlocked(t, tool, []string{"config"})
	// config with invalid subcommand -> blocked
	spectest.AssertBlocked(t, tool, []string{"config", "delete-context", "x"})
	// auth with invalid subcommand -> blocked
	spectest.AssertBlocked(t, tool, []string{"auth", "exec"})
	// unknown command not in any list -> blocked
	spectest.AssertBlocked(t, tool, []string{"bogus"})
}

func TestDoubleDashGuard(t *testing.T) {
	tool := New()
	// -- before command: command after -- is still evaluated
	spectest.AssertBlocked(t, tool, []string{"--", "delete", "pods"})
	spectest.AssertBlocked(t, tool, []string{"--namespace=default", "--", "delete", "pods"})
	// -- after a safe command: harmless, tokens after -- are just args
	spectest.AssertAllowed(t, tool, []string{"get", "pods", "--", "delete", "pods"})
	spectest.AssertAllowed(t, tool, []string{"get", "--", "delete"})
}

func TestKustomizeAllowed(t *testing.T) {
	tool := New()
	allowed := [][]string{
		{"kustomize", "build", "."},
		{"kustomize", "build", "./overlays/prod"},
		{"kustomize", "cfg", "tree"},
	}
	spectest.AssertAllowedBatch(t, tool, allowed)
}

func TestKustomizeBlockedFlags(t *testing.T) {
	tool := New()
	blocked := [][]string{
		{"kustomize", "build", "--enable-helm", "."},
		{"kustomize", "build", "--enable-alpha-plugins", "."},
		{"kustomize", "build", "--network", "."},
		{"kustomize", "build", "--load-restrictor=None", "."},
	}
	spectest.AssertBlockedBatch(t, tool, blocked)
}

func TestHintPropagation(t *testing.T) {
	tool := New()
	tests := []struct {
		args     []string
		wantHint string
	}{
		{[]string{"get", "secrets", "-o", "yaml"}, "-o name"},
		{[]string{"get", "--raw", "/api/v1/secrets"}, "--raw"},
		{[]string{"kustomize", "build", "--enable-helm", "."}, "--enable-helm"},
		{[]string{"config"}, "view"},
		{[]string{"config", "set-context", "evil"}, "view"},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			res := tool.Check(tt.args, spec.RuntimeCtx{})
			if res.Decision != spec.DecisionDeny {
				t.Fatal("expected blocked")
			}
			if !strings.Contains(res.Reason, tt.wantHint) {
				t.Errorf("reason %q should contain %q", res.Reason, tt.wantHint)
			}
		})
	}
}

func TestNoHintFallback(t *testing.T) {
	tool := New()
	// Unknown commands should get generic reason, not empty
	res := tool.Check([]string{"delete", "pods"}, spec.RuntimeCtx{})
	if res.Decision != spec.DecisionDeny {
		t.Fatal("expected blocked")
	}
	if res.Reason != "kubectl command not allowed" {
		t.Errorf("expected generic reason, got %q", res.Reason)
	}
}

func TestUnicodeHomoglyphBlocked(t *testing.T) {
	tool := New()
	// Homoglyphs for "delete"/"exec" must be unknown -> blocked.
	spectest.AssertBlocked(t, tool, []string{"d\u0435lete", "pods"})  // Cyrillic 'е'
	spectest.AssertBlocked(t, tool, []string{"ехес", "pod"})          // Cyrillic
	spectest.AssertBlocked(t, tool, []string{"del\u200bete", "pods"}) // zero-width
}
