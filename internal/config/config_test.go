package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateFlat(t *testing.T) {
	cfg := ToolConfig{Name: "mytool", Allowed: []string{"list", "show"}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
}

func TestValidateNested(t *testing.T) {
	cfg := ToolConfig{
		Name:    "nmcli",
		Allowed: []string{"g", "general"},
		Subcommands: map[string][]string{
			"g":       {"status", "hostname"},
			"general": {"status", "hostname"},
		},
		FlagsWithValue: []string{"-f", "--fields"},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
}

func TestValidateWildcard(t *testing.T) {
	cfg := ToolConfig{Name: "grep", Allowed: []string{"*"}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
}

func TestValidateWildcardMixed(t *testing.T) {
	cfg := ToolConfig{Name: "grep", Allowed: []string{"*", "list"}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for mixing wildcard with other values")
	}
}

func TestValidateMissingName(t *testing.T) {
	cfg := ToolConfig{Allowed: []string{"list"}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestValidateBadName(t *testing.T) {
	cfg := ToolConfig{Name: "has spaces", Allowed: []string{"list"}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for bad name")
	}
}

func TestValidateSubcmdNotSubset(t *testing.T) {
	cfg := ToolConfig{
		Name:    "bad",
		Allowed: []string{"list"},
		Subcommands: map[string][]string{
			"show": {"all"},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for subcommand key not in allowed")
	}
}

func TestValidateBadFlag(t *testing.T) {
	cfg := ToolConfig{Name: "bad", Allowed: []string{"list"}, FlagsWithValue: []string{"nope"}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for flag not starting with -")
	}
}

func TestValidateEmpty(t *testing.T) {
	cfg := ToolConfig{Name: "empty"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for no allowed")
	}
}

func TestValidateWriteTargetValid(t *testing.T) {
	for _, wt := range []string{"last", "all"} {
		cfg := ToolConfig{Name: "cp", Allowed: []string{"*"}, WriteTarget: wt}
		if err := cfg.Validate(); err != nil {
			t.Errorf("expected valid for write_target=%q, got: %v", wt, err)
		}
	}
}

func TestValidateWriteTargetEmpty(t *testing.T) {
	cfg := ToolConfig{Name: "tar", Allowed: []string{"*"}}
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected valid for empty write_target, got: %v", err)
	}
}

func TestValidateWriteTargetInvalid(t *testing.T) {
	for _, wt := range []string{"first", "none", "any"} {
		cfg := ToolConfig{Name: "cp", Allowed: []string{"*"}, WriteTarget: wt}
		if err := cfg.Validate(); err == nil {
			t.Errorf("expected error for write_target=%q", wt)
		}
	}
}

func TestParseTOMLWithWriteTarget(t *testing.T) {
	content := `[cp]
allowed = ["*"]
write_target = "last"
flags_with_value = ["-t", "--target-directory"]

[mkdir]
allowed = ["*"]
write_target = "all"

[tar]
allowed = ["*"]
`
	configs, err := ParseTOML(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(configs) != 3 {
		t.Fatalf("expected 3 configs, got %d", len(configs))
	}
	byName := make(map[string]ConfigTool)
	for _, c := range configs {
		byName[c.Name] = c
	}
	if cp, ok := byName["cp"]; !ok || cp.WriteTarget != "last" {
		t.Errorf("expected cp write_target=last, got %+v", byName["cp"])
	}
	if mkdir, ok := byName["mkdir"]; !ok || mkdir.WriteTarget != "all" {
		t.Errorf("expected mkdir write_target=all, got %+v", byName["mkdir"])
	}
	if tar, ok := byName["tar"]; !ok || tar.WriteTarget != "" {
		t.Errorf("expected tar write_target empty, got %+v", byName["tar"])
	}
}

func TestLoadDirWildcard(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, tmpDir, "tools.toml", `[grep]
allowed = ["*"]

[sed]
allowed = ["*"]

[curl]
allowed = ["*"]
`)

	configs, errs := LoadDir(tmpDir)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(configs) != 3 {
		t.Fatalf("expected 3 configs, got %d", len(configs))
	}
	for _, c := range configs {
		if !c.AllowAll {
			t.Errorf("expected AllowAll for %s", c.ToolConfig.Name)
		}
	}
}

func TestLoadDirToolEntries(t *testing.T) {
	tmpDir := t.TempDir()
	copyTestFile(t, "testdata/valid_tool.toml", tmpDir)

	configs, errs := LoadDir(tmpDir)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}
	c := configs[0]
	if c.ToolConfig.Name != "systemctl" {
		t.Errorf("expected systemctl, got %s", c.ToolConfig.Name)
	}
	if c.AllowAll {
		t.Error("expected AllowAll=false for restricted tool")
	}
}

func TestLoadDirMixed(t *testing.T) {
	tmpDir := t.TempDir()
	copyTestFile(t, "testdata/valid_mixed.toml", tmpDir)

	configs, errs := LoadDir(tmpDir)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	// 2 wildcards + 1 restricted
	if len(configs) != 3 {
		t.Fatalf("expected 3 configs, got %d", len(configs))
	}

	names := make(map[string]bool)
	for _, c := range configs {
		names[c.ToolConfig.Name] = true
	}
	for _, expected := range []string{"grep", "sed", "systemctl"} {
		if !names[expected] {
			t.Errorf("missing config for %s", expected)
		}
	}
}

func TestLoadDirNested(t *testing.T) {
	tmpDir := t.TempDir()
	copyTestFile(t, "testdata/valid_nested.toml", tmpDir)

	configs, errs := LoadDir(tmpDir)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}
	c := configs[0]
	if c.ToolConfig.Name != "nmcli" {
		t.Errorf("expected nmcli, got %s", c.ToolConfig.Name)
	}
	if len(c.Subcommands) != 8 {
		t.Errorf("expected 8 subcommand keys, got %d", len(c.Subcommands))
	}
	if len(c.FlagsWithValue) != 2 {
		t.Errorf("expected 2 flags_with_value, got %d", len(c.FlagsWithValue))
	}
}

func TestLoadDirInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	copyTestFile(t, "testdata/invalid_no_allowed.toml", tmpDir)
	copyTestFile(t, "testdata/valid_mixed.toml", tmpDir)

	configs, errs := LoadDir(tmpDir)
	// valid_mixed has 3 tools; invalid file produces 1 error
	if len(configs) != 3 {
		t.Fatalf("expected 3 valid configs, got %d", len(configs))
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
}

func TestLoadDirNonExistent(t *testing.T) {
	configs, errs := LoadDir("/nonexistent/path/that/does/not/exist")
	if len(configs) != 0 || len(errs) != 0 {
		t.Fatal("expected empty results for nonexistent dir")
	}
}

func TestSearchDirsWithEnvOverride(t *testing.T) {
	acDir := t.TempDir()
	xdgDir := t.TempDir()
	t.Setenv("AGENT_CALLABLE_CONFIG_DIR", acDir)
	t.Setenv("XDG_CONFIG_HOME", xdgDir)
	dirs := SearchDirs()
	if len(dirs) != 1 {
		t.Fatalf("expected 1 dir (exclusive override), got %d: %v", len(dirs), dirs)
	}
	if dirs[0] != acDir+"/tools.d" {
		t.Errorf("expected %s/tools.d, got %s", acDir, dirs[0])
	}
}

func TestSearchDirsOnlyXDG(t *testing.T) {
	xdgDir := t.TempDir()
	t.Setenv("AGENT_CALLABLE_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", xdgDir)
	dirs := SearchDirs()
	if len(dirs) != 1 {
		t.Fatalf("expected 1 dir, got %d: %v", len(dirs), dirs)
	}
	if dirs[0] != xdgDir+"/agent-callable/tools.d" {
		t.Errorf("expected %s/agent-callable/tools.d, got %s", xdgDir, dirs[0])
	}
}

func TestLoadGlobalConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("AGENT_CALLABLE_CONFIG_DIR", tmpDir)
	writeFile(t, tmpDir, "config.toml", `writable_dirs = ["/tmp", "/var/log"]`)

	gc, err := LoadGlobalConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gc.WritableDirs) != 2 {
		t.Fatalf("expected 2 writable_dirs, got %d", len(gc.WritableDirs))
	}
	if gc.WritableDirs[0] != "/tmp" || gc.WritableDirs[1] != "/var/log" {
		t.Errorf("unexpected writable_dirs: %v", gc.WritableDirs)
	}
}

func TestLoadGlobalConfigMissing(t *testing.T) {
	t.Setenv("AGENT_CALLABLE_CONFIG_DIR", "/nonexistent/path")
	gc, err := LoadGlobalConfig()
	if err != nil {
		t.Fatalf("unexpected error for missing config: %v", err)
	}
	if len(gc.WritableDirs) != 0 {
		t.Errorf("expected empty writable_dirs, got %v", gc.WritableDirs)
	}
}

func TestLoadGlobalConfigBuiltins(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("AGENT_CALLABLE_CONFIG_DIR", tmpDir)
	writeFile(t, tmpDir, "config.toml", `writable_dirs = ["/tmp"]

[builtins]
kubectl = false
git = true
`)

	gc, err := LoadGlobalConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gc.Builtins == nil {
		t.Fatal("expected builtins map, got nil")
	}
	if gc.Builtins["kubectl"] != false {
		t.Error("expected kubectl = false")
	}
	if gc.Builtins["git"] != true {
		t.Error("expected git = true")
	}
}

func TestLoadGlobalConfigNoBuiltins(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("AGENT_CALLABLE_CONFIG_DIR", tmpDir)
	writeFile(t, tmpDir, "config.toml", `writable_dirs = ["/tmp"]`)

	gc, err := LoadGlobalConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gc.Builtins != nil {
		t.Errorf("expected nil builtins, got %v", gc.Builtins)
	}
}

func TestValidateWriteFlagsValid(t *testing.T) {
	cfg := ToolConfig{Name: "eslint", Allowed: []string{"*"}, WriteFlags: []string{"--fix"}, WriteTarget: "all"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
}

func TestValidateWriteFlagsBadEntry(t *testing.T) {
	cfg := ToolConfig{Name: "eslint", Allowed: []string{"*"}, WriteFlags: []string{"fix"}, WriteTarget: "all"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for write_flags entry not starting with -")
	}
}

func TestValidateWriteFlagsWithoutWriteTarget(t *testing.T) {
	cfg := ToolConfig{Name: "eslint", Allowed: []string{"*"}, WriteFlags: []string{"--fix"}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for write_flags without write_target")
	}
}

func TestValidateWriteFlagsWithWriteTargetLast(t *testing.T) {
	cfg := ToolConfig{Name: "mytool", Allowed: []string{"*"}, WriteFlags: []string{"-w"}, WriteTarget: "last"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
}

func TestValidateWriteFlagsMultipleEntries(t *testing.T) {
	cfg := ToolConfig{Name: "sed", Allowed: []string{"*"}, WriteFlags: []string{"-i", "--in-place"}, WriteTarget: "all"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
}

func TestValidateWriteFlagsMixedShortLong(t *testing.T) {
	cfg := ToolConfig{Name: "prettier", Allowed: []string{"*"}, WriteFlags: []string{"--write", "-w"}, WriteTarget: "all"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
}

func TestValidateWriteFlagsEmptySlice(t *testing.T) {
	// Empty slice behaves like absent — no error even without write_target
	cfg := ToolConfig{Name: "tool", Allowed: []string{"*"}, WriteFlags: []string{}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid for empty write_flags, got: %v", err)
	}
}

func TestValidateWriteFlagsWithSubcommandAllowed(t *testing.T) {
	cfg := ToolConfig{Name: "lint", Allowed: []string{"check", "format"}, WriteFlags: []string{"--fix"}, WriteTarget: "all"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
}

func TestParseTOMLWithWriteFlagsMultiple(t *testing.T) {
	content := `[sed]
allowed = ["*"]
write_flags = ["-i", "--in-place"]
write_target = "all"
flags_with_value = ["-e", "-f"]
`
	configs, err := ParseTOML(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}
	c := configs[0]
	if len(c.WriteFlags) != 2 {
		t.Errorf("expected 2 write_flags, got %v", c.WriteFlags)
	}
	if c.WriteTarget != "all" {
		t.Errorf("expected write_target=all, got %q", c.WriteTarget)
	}
}

func TestParseTOMLWithWriteFlagsWriteTargetLast(t *testing.T) {
	content := `[mytool]
allowed = ["*"]
write_flags = ["-w"]
write_target = "last"
`
	configs, err := ParseTOML(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}
	c := configs[0]
	if len(c.WriteFlags) != 1 || c.WriteFlags[0] != "-w" {
		t.Errorf("expected write_flags=[-w], got %v", c.WriteFlags)
	}
	if c.WriteTarget != "last" {
		t.Errorf("expected write_target=last, got %q", c.WriteTarget)
	}
}

func TestParseTOMLWithWriteFlags(t *testing.T) {
	content := `[eslint]
allowed = ["*"]
write_flags = ["--fix"]
write_target = "all"
flags_with_value = ["-c", "--config"]
`
	configs, err := ParseTOML(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}
	c := configs[0]
	if len(c.WriteFlags) != 1 || c.WriteFlags[0] != "--fix" {
		t.Errorf("expected write_flags=[--fix], got %v", c.WriteFlags)
	}
	if c.WriteTarget != "all" {
		t.Errorf("expected write_target=all, got %q", c.WriteTarget)
	}
}

func TestParseTOML(t *testing.T) {
	content := `[jq]
allowed = ["*"]

[yq]
allowed = ["*"]

[helm]
allowed = ["version", "list"]
`
	configs, err := ParseTOML(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(configs) != 3 {
		t.Fatalf("expected 3 configs, got %d", len(configs))
	}
	byName := make(map[string]ConfigTool)
	for _, c := range configs {
		byName[c.Name] = c
	}
	if jq, ok := byName["jq"]; !ok || !jq.AllowAll {
		t.Errorf("expected jq allow_all, got %+v", byName["jq"])
	}
	if helm, ok := byName["helm"]; !ok || helm.AllowAll {
		t.Errorf("expected helm restricted, got %+v", byName["helm"])
	}
}

func TestLoadAllNoConfigNoDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("AGENT_CALLABLE_CONFIG_DIR", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "xdg"))

	configs, _, errs := LoadAll() // no embedded defaults
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(configs) != 0 {
		names := make([]string, len(configs))
		for i, c := range configs {
			names[i] = c.Name
		}
		t.Fatalf("expected 0 config tools (deny-by-default), got %d: %v", len(configs), names)
	}
}

func copyTestFile(t *testing.T, src, dstDir string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	dst := filepath.Join(dstDir, filepath.Base(src))
	if err := os.WriteFile(dst, data, 0644); err != nil {
		t.Fatalf("write %s: %v", dst, err)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
