package config

import (
	"testing"

	"github.com/evaneos/agent-callable/internal/shell"
	"github.com/evaneos/agent-callable/internal/spec"
	"github.com/evaneos/agent-callable/internal/spectest"
)

// loadDefaultTool parses the embedded TOML defaults and returns a ToolSpec by name.
func loadDefaultTool(t *testing.T, name string) spec.ToolSpec {
	t.Helper()
	return loadDefaultToolWithDirs(t, name, nil)
}

// loadDefaultToolWithDirs parses the embedded TOML defaults and returns a ToolSpec by name,
// injecting the given writableDirs for write_target checking.
func loadDefaultToolWithDirs(t *testing.T, name string, writableDirs []string) spec.ToolSpec {
	t.Helper()
	for _, content := range shell.DefaultConfigs {
		configs, err := ParseTOML(content)
		if err != nil {
			t.Fatalf("parsing embedded TOML: %v", err)
		}
		for _, c := range configs {
			if c.Name == name {
				return c.ToToolSpec(writableDirs)
			}
		}
	}
	t.Fatalf("tool %q not found in embedded defaults", name)
	return nil
}

// --- jq (allow_all in text-processing.toml) ---

func TestDefaultJqAllowsAnyArgs(t *testing.T) {
	tool := loadDefaultTool(t, "jq")
	cases := [][]string{
		{"."},
		{"-r", ".name", "file.json"},
		{"--arg", "k", "v", ".[$k]"},
	}
	spectest.AssertAllowedBatch(t, tool, cases)
}

// --- write_flags tools (yq, sed, gofmt, ruff, eslint, prettier) ---
// Table-driven tests for the ReadOnly/WriteAllowed/WriteBlocked triplet.

type writeFlagTestCase struct {
	tool         string
	readOnly     [][]string
	writeAllowed [][]string
	writeBlocked [][]string
}

var writeFlagTests = []writeFlagTestCase{
	{
		tool: "yq",
		readOnly: [][]string{
			{"."},
			{"-r", ".name", "file.json"},
			{"--arg", "k", "v", ".[$k]"},
			{"-P", "yaml", ".key", "/etc/file.yaml"},
		},
		writeAllowed: [][]string{
			{"-i", ".key = \"value\"", "/tmp/file.yaml"},
			{"--inplace", ".key = \"value\"", "/tmp/file.yaml"},
		},
		writeBlocked: [][]string{
			{"-i", ".key = \"value\"", "/etc/file.yaml"},
			{"--inplace", ".key = \"value\"", "/etc/file.yaml"},
		},
	},
	{
		tool: "sed",
		readOnly: [][]string{
			{"-e", "s/foo/bar/", "/etc/hosts"},
			{"-f", "script.sed", "/etc/hosts"},
			{"s/foo/bar/", "/etc/hosts"},
			{"-n", "1p", "/etc/hosts"},
		},
		writeAllowed: [][]string{
			{"-i", "-e", "s/foo/bar/", "/tmp/file"},
			{"-i.bak", "-e", "s/foo/bar/", "/tmp/file"},
			{"--in-place", "-e", "s/foo/bar/", "/tmp/file"},
			{"-i", "s/foo/bar/", "/tmp/file"},
		},
		writeBlocked: [][]string{
			{"-i", "-e", "s/foo/bar/", "/etc/hosts"},
			{"-i.bak", "-e", "s/foo/bar/", "/etc/hosts"},
			{"--in-place", "-e", "s/foo/bar/", "/etc/hosts"},
			{"-i", "s/foo/bar/", "/etc/hosts"},
		},
	},
	{
		tool: "gofmt",
		readOnly: [][]string{
			{"-l", "."},
			{"-d", "main.go"},
			{"main.go"},
		},
		writeAllowed: [][]string{
			{"-w", "/tmp/main.go"},
		},
		writeBlocked: [][]string{
			{"-w", "/etc/main.go"},
		},
	},
	{
		tool: "ruff",
		readOnly: [][]string{
			{"check", "."},
			{"format", "--check", "."},
			{"format", "--diff", "/etc/file.py"},
			{"--version"},
			{"check", "/etc/file.py"},
		},
		writeAllowed: [][]string{
			{"check", "--fix", "/tmp/file.py"},
			{"--fix-only", "/tmp/file.py"},
		},
		writeBlocked: [][]string{
			{"check", "--fix", "/etc/file.py"},
			{"--fix-only", "/etc/file.py"},
			{"--fix=true", "/etc/file.py"},
		},
	},
	{
		tool: "eslint",
		readOnly: [][]string{
			{"src/"},
			{"-c", ".eslintrc", "src/"},
			{"--format", "json", "src/"},
			{"--version"},
			{"/etc/file.ts"},
		},
		writeAllowed: [][]string{
			{"--fix", "/tmp/file.ts"},
			{"--fix", "-c", ".eslintrc", "/tmp/file.ts"},
		},
		writeBlocked: [][]string{
			{"--fix", "/etc/file.ts"},
			{"--fix", "-c", ".eslintrc", "/etc/file.ts"},
		},
	},
	{
		tool: "prettier",
		readOnly: [][]string{
			{"--check", "src/"},
			{"--list-different", "src/"},
			{"/etc/file.ts"},
			{"--config", ".prettierrc", "src/"},
		},
		writeAllowed: [][]string{
			{"--write", "/tmp/file.ts"},
			{"-w", "/tmp/file.ts"},
		},
		writeBlocked: [][]string{
			{"--write", "/etc/file.ts"},
			{"-w", "/etc/file.ts"},
		},
	},
}

func TestDefaultWriteFlagTools(t *testing.T) {
	for _, tc := range writeFlagTests {
		t.Run(tc.tool+"/ReadOnly", func(t *testing.T) {
			tool := loadDefaultTool(t, tc.tool)
			spectest.AssertAllowedBatch(t, tool, tc.readOnly)
		})
		t.Run(tc.tool+"/WriteAllowed", func(t *testing.T) {
			tool := loadDefaultToolWithDirs(t, tc.tool, []string{"/tmp"})
			spectest.AssertAllowedBatch(t, tool, tc.writeAllowed)
		})
		t.Run(tc.tool+"/WriteBlocked", func(t *testing.T) {
			tool := loadDefaultToolWithDirs(t, tc.tool, []string{"/tmp"})
			spectest.AssertBlockedBatch(t, tool, tc.writeBlocked)
		})
	}
}

// --- base64 (allow_all in utilities.toml) ---

func TestDefaultBase64AllowsAnyArgs(t *testing.T) {
	tool := loadDefaultTool(t, "base64")
	spectest.AssertAllowed(t, tool, []string{"-d", "aGVsbG8="})
}

// --- envsubst (allow_all in utilities.toml) ---

func TestDefaultEnvsubstAllowsAnyArgs(t *testing.T) {
	tool := loadDefaultTool(t, "envsubst")
	spectest.AssertAllowed(t, tool, []string{})
}

// --- kubectx (allow_all in kubernetes.toml) ---

func TestDefaultKubectxAllowed(t *testing.T) {
	tool := loadDefaultTool(t, "kubectx")
	cases := [][]string{
		{},
		{"prod-context"},
		{"-"},
		{"--current"},
		{"--help"},
		{"-d", "context"},
	}
	spectest.AssertAllowedBatch(t, tool, cases)
}

// --- krew ---

func TestDefaultKrewAllowlist(t *testing.T) {
	tool := loadDefaultTool(t, "krew")
	allowed := [][]string{
		{"list"},
		{"search", "ctx"},
		{"info", "readonly"},
		{"version"},
	}
	spectest.AssertAllowedBatch(t, tool, allowed)
}

func TestDefaultKrewBlocksWrites(t *testing.T) {
	tool := loadDefaultTool(t, "krew")
	blocked := [][]string{
		{"install", "ctx"},
		{"uninstall", "ctx"},
		{"update"},
		{"upgrade"},
	}
	spectest.AssertBlockedBatch(t, tool, blocked)
}

// --- kubectl-crossplane ---

func TestDefaultCrossplaneAllowlist(t *testing.T) {
	tool := loadDefaultTool(t, "kubectl-crossplane")
	allowed := [][]string{
		{"version"},
		{"trace", "xr", "my-xr"},
	}
	spectest.AssertAllowedBatch(t, tool, allowed)
}

func TestDefaultCrossplaneBlocksOthers(t *testing.T) {
	tool := loadDefaultTool(t, "kubectl-crossplane")
	spectest.AssertBlocked(t, tool, []string{"install"})
}

// --- kustomize ---

func TestDefaultKustomizeAllowlist(t *testing.T) {
	tool := loadDefaultTool(t, "kustomize")
	allowed := [][]string{
		{"version"},
		{"build", "./k8s"},
		{"cfg", "tree", "./k8s"},
		{"cfg", "cat", "./k8s"},
		{"cfg", "grep", "pattern"},
	}
	spectest.AssertAllowedBatch(t, tool, allowed)
}

func TestDefaultKustomizeBlocksEdits(t *testing.T) {
	tool := loadDefaultTool(t, "kustomize")
	blocked := [][]string{
		{"edit", "set", "image", "x=y"},
		{"create"},
		{"add", "resource", "x.yaml"},
	}
	spectest.AssertBlockedBatch(t, tool, blocked)
}

// --- docker-compose ---

func TestDefaultDockerComposeAllowlist(t *testing.T) {
	tool := loadDefaultTool(t, "docker-compose")
	allowed := [][]string{
		{"version"},
		{"config"},
		{"ps"},
		{"logs"},
		{"images"},
		{"top"},
		{"port", "web", "80"},
		{"events"},
		{"ls"},
		{"convert"},
		// Global flags with value argument
		{"-f", "docker-compose.prod.yml", "ps"},
		{"--file", "/path/to/dc.yml", "logs"},
		{"-p", "myproject", "config"},
		{"--project-name", "myproject", "images"},
		{"-f", "dc1.yml", "-f", "dc2.yml", "ps"},
		{"--env-file", ".env.prod", "config"},
	}
	spectest.AssertAllowedBatch(t, tool, allowed)
}

func TestDefaultDockerComposeBlocksWrites(t *testing.T) {
	tool := loadDefaultTool(t, "docker-compose")
	blocked := [][]string{
		{"up", "-d"},
		{"down"},
		{"exec", "svc", "sh"},
		{"build"},
		{"-f", "dc.yml", "up", "-d"},
		{"-p", "myproject", "down"},
	}
	spectest.AssertBlockedBatch(t, tool, blocked)
}

func TestDefaultDockerComposeEdgeCases(t *testing.T) {
	tool := loadDefaultTool(t, "docker-compose")

	allowed := [][]string{
		{"config", "--services"},
		{"config", "--volumes"},
		{"config", "--format", "json"},
		{"ps", "-a"},
		{"ps", "--format", "json"},
		{"logs", "-f", "web"},
		{"logs", "--tail", "100", "web", "db"},
		{"images", "--format", "table"},
		{"top", "web"},
		{"port", "web", "8080"},
		{"events", "--json"},
		{"ls", "--all"},
		{"convert", "--format", "json"},
		{"-f", "dc1.yml", "-f", "dc2.yml", "-f", "dc3.yml", "ps"},
		{"-f", "dc.yml", "-p", "proj", "--env-file", ".env", "config"},
	}
	spectest.AssertAllowedBatch(t, tool, allowed)

	blocked := [][]string{
		{"up"},
		{"up", "-d", "--build"},
		{"down", "--volumes", "--rmi", "all"},
		{"start", "web"},
		{"stop", "web"},
		{"restart", "web"},
		{"rm", "-f", "web"},
		{"run", "web", "npm", "test"},
		{"exec", "-T", "web", "sh", "-c", "echo"},
		{"kill", "web"},
		{"build", "web"},
		{"push", "web"},
		{"pull", "web"},
		{"create"},
		{"pause", "web"},
		{"unpause", "web"},
		{"scale", "web=3"},
		{"-f", "dc.yml", "up"},
		{"-p", "proj", "--env-file", ".env", "down"},
	}
	spectest.AssertBlockedBatch(t, tool, blocked)
}

func TestDefaultDockerComposeEmptyAndBareArgs(t *testing.T) {
	tool := loadDefaultTool(t, "docker-compose")
	spectest.AssertBlocked(t, tool, []string{})
	spectest.AssertBlocked(t, tool, []string{"-f", "docker-compose.yml"})
}

// --- helm ---

func TestDefaultHelmAllowlist(t *testing.T) {
	tool := loadDefaultTool(t, "helm")
	allowed := [][]string{
		{"version"},
		{"list", "-A"},
		{"status", "myrel"},
		{"history", "myrel"},
		{"get", "values", "myrel"},
		{"show", "values", "./chart"},
		{"template", "myrel", "./chart"},
		{"lint", "./chart"},
		{"verify", "./chart-0.1.0.tgz"},
		{"repo", "list"},
		{"search", "repo", "nginx"},
		// Global flags with value argument
		{"-n", "production", "list"},
		{"--namespace", "production", "status", "myrel"},
		{"--kube-context", "staging", "list", "-A"},
		{"--kubeconfig", "/path/to/config", "get", "values", "myrel"},
		{"-n", "prod", "--kube-context", "staging", "history", "myrel"},
	}
	spectest.AssertAllowedBatch(t, tool, allowed)
}

func TestDefaultHelmBlocksWrites(t *testing.T) {
	tool := loadDefaultTool(t, "helm")
	blocked := [][]string{
		{"install", "x", "./chart"},
		{"upgrade", "x", "./chart"},
		{"uninstall", "x"},
		{"rollback", "x", "1"},
		{"repo", "add", "x", "https://example.com"},
		{"plugin", "install", "x"},
		{"-n", "production", "install", "x", "./chart"},
		{"--kube-context", "staging", "upgrade", "x", "./chart"},
	}
	spectest.AssertBlockedBatch(t, tool, blocked)
}

func TestDefaultHelmEdgeCases(t *testing.T) {
	tool := loadDefaultTool(t, "helm")

	allowed := [][]string{
		{"env"},
		{"show", "all", "./chart"},
		{"show", "crds", "./chart"},
		{"show", "readme", "./chart"},
		{"get", "all", "myrel"},
		{"get", "hooks", "myrel"},
		{"get", "notes", "myrel"},
		{"get", "manifest", "myrel"},
		{"search", "hub", "nginx"},
		{"template", "myrel", "./chart", "--set", "key=value"},
		{"lint", "./chart", "--strict"},
		{"list", "--all-namespaces"},
		{"list", "--filter", "myrel"},
		{"status", "myrel", "--revision", "3"},
		{"history", "myrel", "--max", "10"},
		{"-n", "prod", "--kube-context", "staging", "--kubeconfig", "/path", "list"},
	}
	spectest.AssertAllowedBatch(t, tool, allowed)

	blocked := [][]string{
		{"repo", "add", "bitnami", "https://charts.bitnami.com"},
		{"repo", "remove", "bitnami"},
		{"repo", "update"},
		{"dependency", "update", "./chart"},
		{"dependency", "build", "./chart"},
		{"plugin", "install", "https://example.com/plugin"},
		{"plugin", "update", "myplugin"},
		{"plugin", "uninstall", "myplugin"},
		{"delete", "myrel"},
		{"push", "chart.tgz", "oci://registry.example.com"},
		{"pull", "bitnami/nginx"},
		{"get", "secrets", "myrel"},
		{"show", "custom", "./chart"},
		{"search", "custom", "nginx"},
		{"test", "myrel"},
		{"package", "./chart"},
		{"create", "mychart"},
		{"-n", "prod", "delete", "myrel"},
		{"--kube-context", "staging", "repo", "add", "x", "url"},
	}
	spectest.AssertBlockedBatch(t, tool, blocked)
}

func TestDefaultHelmEmptyAndBareArgs(t *testing.T) {
	tool := loadDefaultTool(t, "helm")
	spectest.AssertBlocked(t, tool, []string{})
	spectest.AssertBlocked(t, tool, []string{"-n", "production"})
}

// --- flux ---

func TestDefaultFluxAllowlistBasic(t *testing.T) {
	tool := loadDefaultTool(t, "flux")
	allowed := [][]string{
		{"version"},
		{"get", "all"},
		{"get", "kustomizations"},
		{"get", "helmreleases"},
		{"logs"},
		{"logs", "--kind", "Kustomization", "--name", "x"},
		{"tree", "kustomization", "my-app"},
		{"trace", "deployment", "my-app"},
		{"events"},
		{"events", "--for", "Kustomization/my-app"},
		// Global flags with value argument
		{"-n", "flux-system", "get", "kustomizations"},
		{"--namespace", "flux-system", "get", "all"},
		{"--context", "staging", "get", "helmreleases"},
		{"--kubeconfig", "/path/to/config", "logs"},
		{"-n", "flux-system", "--context", "prod", "tree", "kustomization", "my-app"},
	}
	spectest.AssertAllowedBatch(t, tool, allowed)
}

func TestDefaultFluxBlocksWrites(t *testing.T) {
	tool := loadDefaultTool(t, "flux")
	blocked := [][]string{
		{"reconcile", "kustomization", "x"},
		{"suspend", "kustomization", "x"},
		{"resume", "kustomization", "x"},
		{"create", "source", "git"},
		{"-n", "flux-system", "reconcile", "kustomization", "x"},
		{"--context", "prod", "suspend", "kustomization", "x"},
	}
	spectest.AssertBlockedBatch(t, tool, blocked)
}

func TestDefaultFluxEdgeCases(t *testing.T) {
	tool := loadDefaultTool(t, "flux")

	allowed := [][]string{
		{"get", "helmcharts"},
		{"get", "helmrepositories"},
		{"get", "gitrepositories"},
		{"get", "buckets"},
		{"get", "ocirepositories"},
		{"get", "sources"},
		{"get", "alerts"},
		{"get", "providers"},
		{"get", "receivers"},
		{"get", "images"},
		{"get", "imagepolicies"},
		{"get", "imagerepositories"},
		{"get", "imageupdateautomations"},
		{"get", "events"},
		{"logs", "--all-namespaces"},
		{"logs", "--kind", "HelmRelease", "--name", "myrelease"},
		{"logs", "--level", "error"},
		{"tree", "helmrelease", "my-app"},
		{"trace", "service", "my-svc"},
		{"events", "--for", "HelmRelease/my-release"},
		{"events", "--types", "Warning"},
		{"-n", "flux-system", "--context", "prod", "--kubeconfig", "/path", "get", "all"},
	}
	spectest.AssertAllowedBatch(t, tool, allowed)

	blocked := [][]string{
		{"reconcile", "helmrelease", "myrelease"},
		{"reconcile", "source", "git", "mysource"},
		{"suspend", "helmrelease", "myrelease"},
		{"resume", "helmrelease", "myrelease"},
		{"create", "kustomization", "myks"},
		{"create", "helmrelease", "myhr"},
		{"create", "source", "helm", "myrepo"},
		{"delete", "kustomization", "myks"},
		{"delete", "source", "git", "mysource"},
		{"export", "kustomization", "myks"},
		{"export", "helmrelease", "myhr"},
		{"install"},
		{"uninstall"},
		{"bootstrap", "github"},
		{"bootstrap", "gitlab"},
		{"check", "--pre"},
		{"get"},
		{"get", "unknownresource"},
		{"-n", "flux-system", "reconcile", "kustomization", "x"},
		{"--context", "prod", "delete", "kustomization", "x"},
	}
	spectest.AssertBlockedBatch(t, tool, blocked)
}

func TestDefaultFluxEmptyAndBareArgs(t *testing.T) {
	tool := loadDefaultTool(t, "flux")
	spectest.AssertBlocked(t, tool, []string{})
	spectest.AssertBlocked(t, tool, []string{"-n", "flux-system"})
}

// --- sops (secrets tool: filestatus + encrypt only, decrypt blocked) ---

func TestDefaultSopsAllowlistBasic(t *testing.T) {
	tool := loadDefaultTool(t, "sops")
	allowed := [][]string{
		{"filestatus", "secrets.enc.yaml"},
		{"filestatus", "config.json"},
		{"encrypt", "secrets.yaml"},
		{"encrypt", "--kms", "arn:aws:kms:us-east-1:123:key/abc", "secrets.yaml"},
		{"encrypt", "--gcp-kms", "projects/p/locations/l/keyRings/r/cryptoKeys/k", "secrets.yaml"},
		{"encrypt", "--age", "age1abc", "secrets.yaml"},
		{"encrypt", "--pgp", "FBC7B9E2A4F9289AC0C1D4843D16CEE4A27381B4", "secrets.yaml"},
		{"encrypt", "--config", "/path/to/.sops.yaml", "secrets.yaml"},
		{"encrypt", "--input-type", "json", "--output-type", "yaml", "secrets.json"},
	}
	spectest.AssertAllowedBatch(t, tool, allowed)
}

func TestDefaultSopsBlocksSecretOps(t *testing.T) {
	tool := loadDefaultTool(t, "sops")
	blocked := [][]string{
		{"decrypt", "secrets.enc.yaml"},
		{"edit", "secrets.enc.yaml"},
		{"rotate", "secrets.enc.yaml"},
		{"set", "secrets.enc.yaml", "[\"key\"]", "\"value\""},
		{"unset", "secrets.enc.yaml", "[\"key\"]"},
		{"publish", "secrets.enc.yaml"},
		{"exec-env", "secrets.enc.yaml", "env"},
		{"exec-file", "secrets.enc.yaml", "cat {}"},
		{"updatekeys", "secrets.enc.yaml"},
	}
	spectest.AssertBlockedBatch(t, tool, blocked)
}

func TestDefaultSopsEncryptInPlaceAllowed(t *testing.T) {
	tool := loadDefaultToolWithDirs(t, "sops", []string{"/tmp"})
	allowed := [][]string{
		{"-i", "encrypt", "/tmp/secrets.yaml"},
		{"encrypt", "--in-place", "/tmp/secrets.yaml"},
	}
	spectest.AssertAllowedBatch(t, tool, allowed)
}

func TestDefaultSopsEncryptInPlaceBlocked(t *testing.T) {
	tool := loadDefaultToolWithDirs(t, "sops", []string{"/tmp"})
	blocked := [][]string{
		{"-i", "encrypt", "/etc/secrets.yaml"},
		{"encrypt", "--in-place", "/etc/secrets.yaml"},
	}
	spectest.AssertBlockedBatch(t, tool, blocked)
}

func TestDefaultSopsEmptyAndBareArgs(t *testing.T) {
	tool := loadDefaultTool(t, "sops")
	spectest.AssertBlocked(t, tool, []string{})
	spectest.AssertBlocked(t, tool, []string{"--input-type", "json"})
}

// --- go (subcommand allowlist in go.toml) ---

func TestDefaultGoAllowlist(t *testing.T) {
	tool := loadDefaultTool(t, "go")
	allowed := [][]string{
		{"test", "./..."},
		{"build", "./cmd/myapp"},
		{"vet", "./..."},
		{"fmt", "./..."},
		{"mod", "tidy"},
		{"version"},
		{"env"},
		{"list", "-m", "all"},
		{"generate", "./..."},
		{"doc", "fmt"},
		{"tool"},
	}
	spectest.AssertAllowedBatch(t, tool, allowed)
}

func TestDefaultGoBlocksWrites(t *testing.T) {
	tool := loadDefaultTool(t, "go")
	blocked := [][]string{
		{"run", "main.go"},
		{"install", "golang.org/x/tools/...@latest"},
		{"get", "github.com/foo/bar"},
	}
	spectest.AssertBlockedBatch(t, tool, blocked)
}

// --- uv (subcommand allowlist in python.toml) ---

func TestDefaultUvAllowlist(t *testing.T) {
	tool := loadDefaultTool(t, "uv")
	allowed := [][]string{
		{"sync"},
		{"lock"},
		{"venv"},
		{"version"},
		{"python", "list"},
		{"python", "find"},
		{"python", "pin", "3.12"},
		{"pip", "list"},
		{"pip", "show", "requests"},
		{"pip", "check"},
		{"pip", "tree"},
		{"pip", "compile", "requirements.in"},
		// uv run safe commands
		{"run", "pytest"},
		{"run", "mypy", "."},
		{"run", "pyright"},
		{"run", "ty", "check"},
		{"run", "ruff", "check", "."},
		{"run", "black", "--check", "."},
		{"run", "isort", "--check-only", "."},
		{"run", "pylint", "src/"},
		{"run", "flake8", "."},
		{"run", "bandit", "-r", "src/"},
		{"run", "pip-audit"},
	}
	spectest.AssertAllowedBatch(t, tool, allowed)
}

func TestDefaultUvBlocksWrites(t *testing.T) {
	tool := loadDefaultTool(t, "uv")
	blocked := [][]string{
		{"pip", "install", "requests"},
		{"pip", "uninstall", "requests"},
		{"tool", "install", "ruff"},
		{"publish"},
		// uv run arbitrary commands
		{"run", "python", "script.py"},
		{"run", "bash", "-c", "echo"},
		{"run", "rm", "-rf", "/"},
		{"run"},
	}
	spectest.AssertBlockedBatch(t, tool, blocked)
}

// --- ty (allow_all in python.toml) ---

func TestDefaultTyAllowsAnyArgs(t *testing.T) {
	tool := loadDefaultTool(t, "ty")
	cases := [][]string{
		{"check"},
		{"--version"},
	}
	spectest.AssertAllowedBatch(t, tool, cases)
}

// --- make (allow_all in dev-tools.toml) ---

func TestDefaultMakeAllowsAnyArgs(t *testing.T) {
	tool := loadDefaultTool(t, "make")
	cases := [][]string{
		{"test"},
		{"build"},
		{"-j4", "all"},
	}
	spectest.AssertAllowedBatch(t, tool, cases)
}

// --- cp (write_target=last in filesystem-write.toml) ---

func TestDefaultCpWriteTargetAllowed(t *testing.T) {
	tool := loadDefaultToolWithDirs(t, "cp", []string{"/tmp"})
	cases := [][]string{
		{"/etc/hosts", "/tmp/hosts"},
		{"-r", "/src", "/tmp/dest"},
		{"-r", "-v", "/src", "/tmp/dest/file"},
		{"--", "-weird-file", "/tmp/dst"},
	}
	spectest.AssertAllowedBatch(t, tool, cases)
}

func TestDefaultCpWriteTargetBlocked(t *testing.T) {
	tool := loadDefaultToolWithDirs(t, "cp", []string{"/tmp"})
	cases := [][]string{
		{"/etc/hosts", "/usr/local/bin/hosts"},
		{"-r", "/src", "/etc/dest"},
	}
	spectest.AssertBlockedBatch(t, tool, cases)
}

// --- mv (write_target=last in filesystem-write.toml) ---

func TestDefaultMvWriteTargetAllowed(t *testing.T) {
	tool := loadDefaultToolWithDirs(t, "mv", []string{"/tmp"})
	spectest.AssertAllowed(t, tool, []string{"/tmp/a", "/tmp/b"})
}

func TestDefaultMvWriteTargetBlocked(t *testing.T) {
	tool := loadDefaultToolWithDirs(t, "mv", []string{"/tmp"})
	spectest.AssertBlocked(t, tool, []string{"/tmp/a", "/etc/b"})
}

// --- ln (write_target=last in filesystem-write.toml) ---

func TestDefaultLnWriteTargetAllowed(t *testing.T) {
	tool := loadDefaultToolWithDirs(t, "ln", []string{"/tmp"})
	spectest.AssertAllowed(t, tool, []string{"-s", "/some/target", "/tmp/link"})
}

func TestDefaultLnWriteTargetBlocked(t *testing.T) {
	tool := loadDefaultToolWithDirs(t, "ln", []string{"/tmp"})
	spectest.AssertBlocked(t, tool, []string{"-s", "/tmp/target", "/etc/link"})
}

// --- mkdir (write_target=all in filesystem-write.toml) ---

func TestDefaultMkdirWriteTargetAllowed(t *testing.T) {
	tool := loadDefaultToolWithDirs(t, "mkdir", []string{"/tmp"})
	cases := [][]string{
		{"/tmp/a", "/tmp/b"},
		{"-p", "/tmp/a/b/c"},
	}
	spectest.AssertAllowedBatch(t, tool, cases)
}

func TestDefaultMkdirWriteTargetBlocked(t *testing.T) {
	tool := loadDefaultToolWithDirs(t, "mkdir", []string{"/tmp"})
	cases := [][]string{
		{"/etc/bad"},
		{"/tmp/ok", "/etc/bad"},
		{"-p", "/usr/local/new"},
	}
	spectest.AssertBlockedBatch(t, tool, cases)
}

// --- touch (write_target=all in filesystem-write.toml) ---

func TestDefaultTouchWriteTargetAllowed(t *testing.T) {
	tool := loadDefaultToolWithDirs(t, "touch", []string{"/tmp"})
	spectest.AssertAllowed(t, tool, []string{"/tmp/file"})
}

func TestDefaultTouchWriteTargetBlocked(t *testing.T) {
	tool := loadDefaultToolWithDirs(t, "touch", []string{"/tmp"})
	spectest.AssertBlocked(t, tool, []string{"/etc/file"})
}

// --- tee (write_target=all in filesystem-write.toml) ---

func TestDefaultTeeWriteTargetAllowed(t *testing.T) {
	tool := loadDefaultToolWithDirs(t, "tee", []string{"/tmp"})
	cases := [][]string{
		{"/tmp/out"},
		{"/dev/null"},
		{"-a", "/tmp/out"},
	}
	spectest.AssertAllowedBatch(t, tool, cases)
}

func TestDefaultTeeWriteTargetBlocked(t *testing.T) {
	tool := loadDefaultToolWithDirs(t, "tee", []string{"/tmp"})
	spectest.AssertBlocked(t, tool, []string{"/etc/passwd"})
}

// --- chmod (write_target = last) ---

func TestDefaultChmodWriteTarget(t *testing.T) {
	tool := loadDefaultToolWithDirs(t, "chmod", []string{"/tmp"})
	spectest.AssertAllowed(t, tool, []string{"+x", "/tmp/script.sh"})
	spectest.AssertBlocked(t, tool, []string{"777", "/etc/passwd"})
}

// --- tsc (allow_all in typescript.toml) ---

func TestDefaultTscAllowsAnyArgs(t *testing.T) {
	tool := loadDefaultTool(t, "tsc")
	cases := [][]string{
		{"--version"},
		{"--noEmit"},
		{"--project", "tsconfig.json"},
		{"--build"},
		{},
	}
	spectest.AssertAllowedBatch(t, tool, cases)
}

// --- npx (restricted allowlist in typescript.toml) ---

func TestDefaultNpxAllowsSafeTools(t *testing.T) {
	tool := loadDefaultTool(t, "npx")
	allowed := [][]string{
		{"tsc", "--noEmit"},
		{"eslint", "."},
		{"prettier", "--check", "."},
		{"jest"},
		{"vitest"},
		{"tsx", "script.ts"},
	}
	spectest.AssertAllowedBatch(t, tool, allowed)
}

func TestDefaultNpxBlocksArbitrary(t *testing.T) {
	tool := loadDefaultTool(t, "npx")
	blocked := [][]string{
		{"rimraf", "dist"},
		{"cowsay", "hello"},
		{"ts-node", "script.ts"},
	}
	spectest.AssertBlockedBatch(t, tool, blocked)
}

