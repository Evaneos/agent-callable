package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/evaneos/agent-callable/internal/spec"
)

// --- LoadAll pipeline tests ---

func TestLoadAllFromDisk(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGENT_CALLABLE_CONFIG_DIR", dir)
	toolsDir := filepath.Join(dir, "tools.d")
	if err := os.MkdirAll(toolsDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, toolsDir, "tools.toml", `[grep]
allowed = ["*"]

[helm]
allowed = ["version", "list"]
`)

	tools, gc, errs := LoadAll()
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	if gc == nil {
		t.Fatal("expected non-nil GlobalConfig")
	}
}

func TestLoadAllFromEmbedded(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGENT_CALLABLE_CONFIG_DIR", dir)

	embedded := map[string]string{
		"test.toml": `[jq]
allowed = ["*"]

[curl]
allowed = ["*"]
`,
	}

	tools, _, errs := LoadAll(embedded)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools from embedded, got %d", len(tools))
	}
	names := toolNames(tools)
	if !names["jq"] || !names["curl"] {
		t.Errorf("expected jq and curl, got %v", names)
	}
}

func TestLoadAllDiskBeatsEmbedded(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGENT_CALLABLE_CONFIG_DIR", dir)
	toolsDir := filepath.Join(dir, "tools.d")
	if err := os.MkdirAll(toolsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Disk: grep with restricted allowed.
	writeFile(t, toolsDir, "grep.toml", `[grep]
allowed = ["--count"]
`)
	// Embedded: grep with wildcard.
	embedded := map[string]string{
		"grep.toml": `[grep]
allowed = ["*"]
`,
	}

	tools, _, errs := LoadAll(embedded)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].AllowAll {
		t.Error("disk config should win: expected AllowAll=false (restricted), not wildcard from embedded")
	}
}

func TestLoadAllGlobalConfigPropagates(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGENT_CALLABLE_CONFIG_DIR", dir)
	writeFile(t, dir, "config.toml", `writable_dirs = ["/tmp", "/var/tmp"]

[builtins]
git = false
gh = true
`)
	toolsDir := filepath.Join(dir, "tools.d")
	if err := os.MkdirAll(toolsDir, 0755); err != nil {
		t.Fatal(err)
	}

	_, gc, errs := LoadAll()
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(gc.WritableDirs) != 2 {
		t.Fatalf("expected 2 writable_dirs, got %v", gc.WritableDirs)
	}
	if gc.WritableDirs[0] != "/tmp" || gc.WritableDirs[1] != "/var/tmp" {
		t.Errorf("unexpected writable_dirs: %v", gc.WritableDirs)
	}
	if gc.Builtins == nil {
		t.Fatal("expected builtins map")
	}
	if gc.Builtins["git"] != false {
		t.Error("expected git = false")
	}
	if gc.Builtins["gh"] != true {
		t.Error("expected gh = true")
	}
}

func TestLoadAllBadTOMLOnDisk(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGENT_CALLABLE_CONFIG_DIR", dir)
	toolsDir := filepath.Join(dir, "tools.d")
	if err := os.MkdirAll(toolsDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, toolsDir, "valid.toml", `[grep]
allowed = ["*"]
`)
	writeFile(t, toolsDir, "bad.toml", `not valid toml [[[`)

	tools, _, errs := LoadAll()
	if len(errs) == 0 {
		t.Fatal("expected at least one parse error")
	}
	// Valid file should still be loaded.
	if len(tools) != 1 || tools[0].Name != "grep" {
		t.Errorf("expected grep to be loaded despite error, got %v", toolNames(tools))
	}
}

func TestLoadAllBadEmbeddedTOML(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGENT_CALLABLE_CONFIG_DIR", dir)

	embedded := map[string]string{
		"valid.toml": `[jq]
allowed = ["*"]
`,
		"bad.toml": `[[[[invalid`,
	}

	tools, _, errs := LoadAll(embedded)
	if len(errs) == 0 {
		t.Fatal("expected at least one error for bad embedded TOML")
	}
	// jq should still be loaded (if bad.toml was processed after valid.toml).
	// We can't guarantee order of map iteration, so just check errs is non-empty.
	_ = tools
}

// --- Full E2E: TOML → LoadAll → ToToolSpec → Check ---

func TestE2EAllowedSubcommand(t *testing.T) {
	_, tool := setupE2ETool(t, `[helm]
allowed = ["version", "list", "status"]
`, nil)

	result := tool.Check([]string{"list"}, runtimeCtx())
	if result.Decision != spec.DecisionAllow {
		t.Errorf("expected allow for 'helm list', got deny: %s", result.Reason)
	}
}

func TestE2EBlockedSubcommand(t *testing.T) {
	_, tool := setupE2ETool(t, `[helm]
allowed = ["version", "list", "status"]
`, nil)

	result := tool.Check([]string{"install", "myrelease", "mychart"}, runtimeCtx())
	if result.Decision == spec.DecisionAllow {
		t.Error("expected deny for 'helm install' (not in allowed)")
	}
}

func TestE2EWildcardAllowed(t *testing.T) {
	_, toolSpec := setupE2ETool(t, `[grep]
allowed = ["*"]
`, nil)

	result := toolSpec.Check([]string{"--count", "pattern", "file.txt"}, runtimeCtx())
	if result.Decision != spec.DecisionAllow {
		t.Errorf("expected allow for wildcard grep, got: %s", result.Reason)
	}
}

func TestE2EWriteTargetAllowedInWritableDir(t *testing.T) {
	tmpDst := t.TempDir()
	_, toolSpec := setupE2ETool(t, `[cp]
allowed = ["*"]
write_target = "last"
`, []string{tmpDst})

	result := toolSpec.Check([]string{"src.txt", filepath.Join(tmpDst, "dst.txt")}, runtimeCtx())
	if result.Decision != spec.DecisionAllow {
		t.Errorf("expected allow for cp to writable dir, got: %s", result.Reason)
	}
}

func TestE2EWriteTargetBlockedOutsideWritableDir(t *testing.T) {
	tmpDst := t.TempDir()
	_, toolSpec := setupE2ETool(t, `[cp]
allowed = ["*"]
write_target = "last"
`, []string{tmpDst})

	result := toolSpec.Check([]string{"src.txt", "/etc/shadow"}, runtimeCtx())
	if result.Decision == spec.DecisionAllow {
		t.Error("expected deny for cp to /etc/shadow (outside writable dirs)")
	}
}

func TestE2EEmptyWritableDirsBlocksAllWrites(t *testing.T) {
	_, toolSpec := setupE2ETool(t, `[cp]
allowed = ["*"]
write_target = "last"
`, []string{} /* empty */)

	result := toolSpec.Check([]string{"src.txt", "/tmp/dst.txt"}, runtimeCtx())
	if result.Decision == spec.DecisionAllow {
		t.Error("expected deny with empty writable_dirs, even for /tmp")
	}
}

const sedTOML = `[sed]
allowed = ["*"]
write_flags = ["-i", "--in-place"]
write_target = "last"
flags_with_value = ["-e", "-f", "--expression", "--file"]
`

func TestE2EWriteFlagsReadOnlyAllowed(t *testing.T) {
	_, toolSpec := setupE2ETool(t, sedTOML, []string{t.TempDir()})

	// Without write flag → read-only → allowed even for /etc.
	result := toolSpec.Check([]string{"s/foo/bar/", "/etc/passwd"}, runtimeCtx())
	if result.Decision != spec.DecisionAllow {
		t.Errorf("expected allow for read-only sed, got: %s", result.Reason)
	}
}

func TestE2EWriteFlagsInPlaceBlocked(t *testing.T) {
	writable := t.TempDir()
	_, toolSpec := setupE2ETool(t, sedTOML, []string{writable})

	// With -i flag, target is /etc/passwd (last arg) → blocked.
	result := toolSpec.Check([]string{"-i", "s/foo/bar/", "/etc/passwd"}, runtimeCtx())
	if result.Decision == spec.DecisionAllow {
		t.Error("expected deny for sed -i targeting /etc")
	}
}

func TestE2EWriteFlagsInPlaceAllowedInWritableDir(t *testing.T) {
	writable := t.TempDir()
	_, toolSpec := setupE2ETool(t, sedTOML, []string{writable})

	// With -i flag, target is a file in writable dir (last arg) → allowed.
	result := toolSpec.Check([]string{"-i", "s/foo/bar/", filepath.Join(writable, "file.txt")}, runtimeCtx())
	if result.Decision != spec.DecisionAllow {
		t.Errorf("expected allow for sed -i to writable dir, got: %s", result.Reason)
	}
}

// --- Rust tools ---

const cargoTOML = `[cargo]
allowed = ["build", "check", "test", "clippy", "fmt", "doc", "bench", "clean", "update", "metadata", "tree", "version"]
`

func TestE2ECargoAllowedSubcommands(t *testing.T) {
	for _, sub := range []string{"build", "check", "test", "clippy", "fmt", "doc", "bench", "clean", "update", "metadata", "tree", "version"} {
		t.Run(sub, func(t *testing.T) {
			_, tool := setupE2ETool(t, cargoTOML, nil)
			result := tool.Check([]string{sub}, runtimeCtx())
			if result.Decision != spec.DecisionAllow {
				t.Errorf("expected allow for 'cargo %s', got deny: %s", sub, result.Reason)
			}
		})
	}
}

func TestE2ECargoBlockedSubcommands(t *testing.T) {
	for _, sub := range []string{"run", "install", "publish", "login", "owner", "yank"} {
		t.Run(sub, func(t *testing.T) {
			_, tool := setupE2ETool(t, cargoTOML, nil)
			result := tool.Check([]string{sub}, runtimeCtx())
			if result.Decision == spec.DecisionAllow {
				t.Errorf("expected deny for 'cargo %s'", sub)
			}
		})
	}
}

const rustcTOML = `[rustc]
allowed = ["*"]
`

func TestE2ERustcAllowed(t *testing.T) {
	_, tool := setupE2ETool(t, rustcTOML, nil)
	result := tool.Check([]string{"--version"}, runtimeCtx())
	if result.Decision != spec.DecisionAllow {
		t.Errorf("expected allow for 'rustc --version', got deny: %s", result.Reason)
	}
}

func TestE2ERustcArbitraryArgs(t *testing.T) {
	_, tool := setupE2ETool(t, rustcTOML, nil)
	result := tool.Check([]string{"main.rs", "-o", "main"}, runtimeCtx())
	if result.Decision != spec.DecisionAllow {
		t.Errorf("expected allow for 'rustc main.rs -o main', got deny: %s", result.Reason)
	}
}

// --- Concourse fly ---

const flyTOML = `[fly]
allowed = ["builds", "bs", "containers", "cs", "completion", "format-pipeline", "fp", "get-pipeline", "gp", "jobs", "js", "pipelines", "ps", "resource-versions", "rvs", "resources", "rs", "status", "targets", "ts", "teams", "userinfo", "validate-pipeline", "vp", "version", "volumes", "vs", "watch", "w", "workers", "ws"]
flags_with_value = ["-t", "--target", "-p", "--pipeline", "-j", "--job", "-b", "--build", "-n", "--team-name", "-c", "--config"]
`

func TestE2EFlyAllowedSubcommands(t *testing.T) {
	for _, sub := range []string{
		"builds", "bs", "containers", "cs", "completion",
		"format-pipeline", "fp", "get-pipeline", "gp",
		"jobs", "js", "pipelines", "ps",
		"resource-versions", "rvs", "resources", "rs",
		"status", "targets", "ts", "teams", "userinfo",
		"validate-pipeline", "vp", "version",
		"volumes", "vs", "watch", "w", "workers", "ws",
	} {
		t.Run(sub, func(t *testing.T) {
			_, tool := setupE2ETool(t, flyTOML, nil)
			result := tool.Check([]string{"-t", "my-target", sub}, runtimeCtx())
			if result.Decision != spec.DecisionAllow {
				t.Errorf("expected allow for 'fly %s', got deny: %s", sub, result.Reason)
			}
		})
	}
}

func TestE2EFlyBlockedSubcommands(t *testing.T) {
	for _, sub := range []string{
		"set-pipeline", "sp", "destroy-pipeline", "dp",
		"trigger-job", "tj", "abort-build", "ab",
		"pause-pipeline", "pp", "unpause-pipeline", "up",
		"set-team", "st", "destroy-team",
		"login", "logout", "sync",
		"intercept", "hijack", "execute", "curl",
		"expose-pipeline", "ep", "hide-pipeline", "hp",
		"archive-pipeline", "ap", "rename-pipeline", "rp",
		"pin-resource", "pr", "unpin-resource", "ur",
		"check-resource", "cr",
		"prune-worker", "pw", "land-worker", "lw",
	} {
		t.Run(sub, func(t *testing.T) {
			_, tool := setupE2ETool(t, flyTOML, nil)
			result := tool.Check([]string{"-t", "my-target", sub}, runtimeCtx())
			if result.Decision == spec.DecisionAllow {
				t.Errorf("expected deny for 'fly %s'", sub)
			}
		})
	}
}

// --- Config resolution tests ---

func TestConfigBaseDirFallbackToXDG(t *testing.T) {
	xdgDir := t.TempDir()
	t.Setenv("AGENT_CALLABLE_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", xdgDir)
	got := ConfigBaseDir()
	want := filepath.Join(xdgDir, "agent-callable")
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestXDGBaseDirFallbackToHome(t *testing.T) {
	t.Setenv("AGENT_CALLABLE_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir available")
	}
	got := ConfigBaseDir()
	want := filepath.Join(home, ".config", "agent-callable")
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestLoadGlobalConfigEmptyWritableDirs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGENT_CALLABLE_CONFIG_DIR", dir)
	writeFile(t, dir, "config.toml", "writable_dirs = []\n")

	gc, err := LoadGlobalConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gc.WritableDirs) != 0 {
		t.Errorf("expected empty writable_dirs, got %v", gc.WritableDirs)
	}
}

func TestLoadGlobalConfigInvalidTOML(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGENT_CALLABLE_CONFIG_DIR", dir)
	writeFile(t, dir, "config.toml", `[[[[invalid`)

	_, err := LoadGlobalConfig()
	if err == nil {
		t.Fatal("expected error for invalid config.toml")
	}
	if !strings.Contains(err.Error(), "parsing") {
		t.Errorf("expected 'parsing' in error, got: %v", err)
	}
}

// --- Helpers ---

// setupE2ETool writes a TOML string to a temp dir, loads it via LoadAll,
// and returns the ConfigToolSpec for the single tool defined in the TOML.
func setupE2ETool(t *testing.T, tomlContent string, writableDirs []string) (string, *spec.ConfigToolSpec) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("AGENT_CALLABLE_CONFIG_DIR", dir)
	toolsDir := filepath.Join(dir, "tools.d")
	if err := os.MkdirAll(toolsDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, toolsDir, "tool.toml", tomlContent)

	tools, _, errs := LoadAll()
	if len(errs) > 0 {
		t.Fatalf("LoadAll: %v", errs)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d: %v", len(tools), toolNames(tools))
	}
	return dir, tools[0].ToToolSpec(writableDirs)
}

func runtimeCtx() spec.RuntimeCtx { return spec.RuntimeCtx{} }

func toolNames(tools []ConfigTool) map[string]bool {
	m := make(map[string]bool, len(tools))
	for _, c := range tools {
		m[c.Name] = true
	}
	return m
}
