package spec

import (
	"strings"
	"testing"
)

func allowAllTool(name string) ConfigToolOpts {
	return ConfigToolOpts{Name: name, AllowAll: true}
}

func toolFromConfig(name string, allowed []string, subcommands map[string][]string, flagsWithValue []string, env map[string]string) ConfigToolOpts {
	return ConfigToolOpts{
		Name:           name,
		Allowed:        allowed,
		Subcommands:    subcommands,
		FlagsWithValue: flagsWithValue,
		Env:            env,
	}
}

// --- Readonly tool ---

func TestConfigToolReadonly(t *testing.T) {
	tool := NewConfigTool(allowAllTool("mycli"))

	allowed := [][]string{
		{"anything"},
		{"--flag", "value"},
		{"sub", "cmd", "--verbose"},
		{},
	}
	for _, a := range allowed {
		res := tool.Check(a, RuntimeCtx{})
		if res.Decision != DecisionAllow {
			t.Errorf("expected allowed for %v, got deny: %s", a, res.Reason)
		}
	}
}

// --- Flat allowlist tool ---

func TestConfigToolFlat(t *testing.T) {
	tool := NewConfigTool(toolFromConfig("mytool", []string{"list", "show", "version"}, nil, nil, nil))

	allowed := [][]string{
		{"list"},
		{"show", "details"},
		{"version"},
		{"--verbose", "list"},
	}
	for _, a := range allowed {
		res := tool.Check(a, RuntimeCtx{})
		if res.Decision != DecisionAllow {
			t.Errorf("expected allowed for %v, got deny: %s", a, res.Reason)
		}
	}

	blocked := [][]string{
		{"delete"},
		{"apply"},
		{"unknown"},
	}
	for _, a := range blocked {
		res := tool.Check(a, RuntimeCtx{})
		if res.Decision == DecisionAllow {
			t.Errorf("expected blocked for %v", a)
		}
	}
}

// --- Nested allowlist tool (nmcli) ---

func newNmcli() *ConfigToolSpec {
	return NewConfigTool(toolFromConfig(
		"nmcli",
		[]string{
			"g", "general",
			"c", "connection",
			"d", "device",
			"r", "radio",
		},
		map[string][]string{
			"g":          {"status", "hostname"},
			"general":    {"status", "hostname"},
			"c":          {"show", "monitor"},
			"connection": {"show", "monitor"},
			"d":          {"status", "show"},
			"device":     {"status", "show"},
			"r":          {"wifi"},
			"radio":      {"wifi"},
		},
		[]string{"-f", "--fields"},
		nil,
	))
}

func TestNmcliAllowed(t *testing.T) {
	tool := newNmcli()

	allowed := [][]string{
		{"g", "status"},
		{"g", "hostname"},
		{"general", "status"},
		{"general", "hostname"},
		{"c", "show"},
		{"c", "monitor"},
		{"connection", "show"},
		{"connection", "monitor"},
		{"d", "status"},
		{"d", "show"},
		{"device", "status"},
		{"device", "show"},
		{"r", "wifi"},
		{"radio", "wifi"},
		// flags_with_value skipping
		{"-f", "NAME,STATE", "g", "status"},
		{"--fields", "NAME", "c", "show"},
	}
	for _, a := range allowed {
		res := tool.Check(a, RuntimeCtx{})
		if res.Decision != DecisionAllow {
			t.Errorf("expected allowed for %v, got deny: %s", a, res.Reason)
		}
	}
}

func TestNmcliBlocked(t *testing.T) {
	tool := newNmcli()

	blocked := [][]string{
		{"c", "add"},
		{"c", "modify"},
		{"connection", "delete"},
		{"connection", "up"},
		{"d", "connect"},
		{"unknown"},
		{"g"},       // nested: subcommand required
		{"c"},       // nested: subcommand required
		{},          // empty
		{"-f", "x"}, // only flags, no positional
	}
	for _, a := range blocked {
		res := tool.Check(a, RuntimeCtx{})
		if res.Decision == DecisionAllow {
			t.Errorf("expected blocked for %v", a)
		}
	}
}

func TestNmcliEnv(t *testing.T) {
	tool := newNmcli()
	env := tool.NonInteractiveEnv()
	if env != nil {
		t.Errorf("expected nil env, got %v", env)
	}
}

// --- Edge cases ---

func TestConfigToolEmptyArgs(t *testing.T) {
	tool := NewConfigTool(toolFromConfig("mytool", []string{"list"}, nil, nil, nil))
	res := tool.Check([]string{}, RuntimeCtx{})
	if res.Decision == DecisionAllow {
		t.Error("expected blocked for empty args")
	}
}

func TestConfigToolOnlyFlags(t *testing.T) {
	tool := NewConfigTool(toolFromConfig("mytool", []string{"list"}, nil, nil, nil))
	res := tool.Check([]string{"--verbose", "--all"}, RuntimeCtx{})
	if res.Decision == DecisionAllow {
		t.Error("expected blocked for only-flags args")
	}
}

func TestConfigToolDoubleDash(t *testing.T) {
	tool := NewConfigTool(toolFromConfig("mytool", []string{"list"}, nil, nil, nil))
	// -- stops flag parsing; "list" after -- is not positional
	res := tool.Check([]string{"--", "list"}, RuntimeCtx{})
	if res.Decision == DecisionAllow {
		t.Error("expected blocked: -- stops positional parsing")
	}
}

func TestConfigToolNoEnv(t *testing.T) {
	tool := NewConfigTool(allowAllTool("mycli"))
	env := tool.NonInteractiveEnv()
	if env != nil {
		t.Errorf("expected nil env, got %v", env)
	}
}

// --- write_target tests ---

func writeTargetTool(name, writeTarget string, writableDirs []string, flagsWithValue []string) *ConfigToolSpec {
	return NewConfigTool(ConfigToolOpts{
		Name:           name,
		AllowAll:       true,
		WriteTarget:    writeTarget,
		WritableDirs:   writableDirs,
		FlagsWithValue: flagsWithValue,
	})
}

func TestWriteTargetLastAllowed(t *testing.T) {
	tool := writeTargetTool("cp", "last", []string{"/tmp"}, nil)

	allowed := [][]string{
		{"/etc/hosts", "/tmp/hosts"},
		{"-r", "/src", "/tmp/dest"},
		{"-r", "-v", "/src", "/tmp/dest/file"},
	}
	for _, a := range allowed {
		res := tool.Check(a, RuntimeCtx{})
		if res.Decision != DecisionAllow {
			t.Errorf("expected allowed for %v, got deny: %s", a, res.Reason)
		}
	}
}

func TestWriteTargetLastBlocked(t *testing.T) {
	tool := writeTargetTool("cp", "last", []string{"/tmp"}, nil)

	blocked := [][]string{
		{"/etc/hosts", "/usr/local/bin/hosts"},
		{"-r", "/src", "/etc/dest"},
		{"/tmp/src", "/home/user/file"},
	}
	for _, a := range blocked {
		res := tool.Check(a, RuntimeCtx{})
		if res.Decision == DecisionAllow {
			t.Errorf("expected blocked for %v", a)
		}
	}
}

func TestWriteTargetLastWithDoubleDash(t *testing.T) {
	tool := writeTargetTool("cp", "last", []string{"/tmp"}, nil)

	res := tool.Check([]string{"--", "-weird-src", "/tmp/dst"}, RuntimeCtx{})
	if res.Decision != DecisionAllow {
		t.Errorf("expected allowed after --, got deny: %s", res.Reason)
	}

	res = tool.Check([]string{"--", "-src", "/etc/dst"}, RuntimeCtx{})
	if res.Decision == DecisionAllow {
		t.Error("expected blocked for /etc/dst after --")
	}
}

func TestWriteTargetLastNoPositionalArgs(t *testing.T) {
	tool := writeTargetTool("cp", "last", []string{"/tmp"}, nil)
	res := tool.Check([]string{}, RuntimeCtx{})
	if res.Decision != DecisionAllow {
		t.Errorf("expected allowed for no args, got deny: %s", res.Reason)
	}
}

func TestWriteTargetLastSingleArg(t *testing.T) {
	tool := writeTargetTool("cp", "last", []string{"/tmp"}, nil)

	res := tool.Check([]string{"/tmp/file"}, RuntimeCtx{})
	if res.Decision != DecisionAllow {
		t.Errorf("expected allowed for single arg under /tmp, got deny: %s", res.Reason)
	}

	res = tool.Check([]string{"/etc/file"}, RuntimeCtx{})
	if res.Decision == DecisionAllow {
		t.Error("expected blocked for single arg /etc/file")
	}
}

func TestWriteTargetLastWithFlagsWithValue(t *testing.T) {
	tool := writeTargetTool("cp", "last", []string{"/tmp"}, []string{"-t", "--target-directory"})

	// -t consumes next arg, so "a" is the only positional (and last)
	res := tool.Check([]string{"-t", "/dir", "/tmp/file"}, RuntimeCtx{})
	if res.Decision != DecisionAllow {
		t.Errorf("expected allowed, got deny: %s", res.Reason)
	}
}

func TestWriteTargetAllAllowed(t *testing.T) {
	tool := writeTargetTool("mkdir", "all", []string{"/tmp"}, nil)

	allowed := [][]string{
		{"/tmp/a", "/tmp/b"},
		{"-p", "/tmp/a/b/c"},
		{"/tmp/single"},
	}
	for _, a := range allowed {
		res := tool.Check(a, RuntimeCtx{})
		if res.Decision != DecisionAllow {
			t.Errorf("expected allowed for %v, got deny: %s", a, res.Reason)
		}
	}
}

func TestWriteTargetAllBlocked(t *testing.T) {
	tool := writeTargetTool("mkdir", "all", []string{"/tmp"}, nil)

	blocked := [][]string{
		{"/tmp/ok", "/etc/bad"},
		{"/usr/local/bad"},
		{"-p", "/etc/a/b/c"},
	}
	for _, a := range blocked {
		res := tool.Check(a, RuntimeCtx{})
		if res.Decision == DecisionAllow {
			t.Errorf("expected blocked for %v", a)
		}
	}
}

func TestWriteTargetDevNull(t *testing.T) {
	for _, wt := range []string{"last", "all"} {
		tool := writeTargetTool("tee", wt, []string{"/tmp"}, nil)
		res := tool.Check([]string{"/dev/null"}, RuntimeCtx{})
		if res.Decision != DecisionAllow {
			t.Errorf("write_target=%q: expected /dev/null allowed, got deny: %s", wt, res.Reason)
		}
	}
}

func TestWriteTargetEmptyWritableDirs(t *testing.T) {
	tool := writeTargetTool("cp", "last", nil, nil)

	// /dev/null always allowed
	res := tool.Check([]string{"src", "/dev/null"}, RuntimeCtx{})
	if res.Decision != DecisionAllow {
		t.Errorf("expected /dev/null allowed with empty dirs, got deny: %s", res.Reason)
	}

	// everything else blocked
	res = tool.Check([]string{"src", "/tmp/dst"}, RuntimeCtx{})
	if res.Decision == DecisionAllow {
		t.Error("expected blocked with empty writable_dirs")
	}
}

func TestWriteTargetEmptyString(t *testing.T) {
	// No write_target set -> no checking (current behavior)
	tool := writeTargetTool("tar", "", []string{"/tmp"}, nil)
	res := tool.Check([]string{"-xf", "/etc/archive.tar"}, RuntimeCtx{})
	if res.Decision != DecisionAllow {
		t.Errorf("expected allowed without write_target, got deny: %s", res.Reason)
	}
}

func TestWriteTargetDenyMessageContainsToolAndPath(t *testing.T) {
	tool := writeTargetTool("cp", "last", []string{"/tmp"}, nil)
	res := tool.Check([]string{"src", "/etc/passwd"}, RuntimeCtx{})
	if res.Decision == DecisionAllow {
		t.Fatal("expected blocked")
	}
	if res.Reason == "" {
		t.Fatal("expected non-empty reason")
	}
	if !strings.Contains(res.Reason, "cp") || !strings.Contains(res.Reason, "/etc/passwd") {
		t.Errorf("reason should mention tool and path, got: %s", res.Reason)
	}
}

// --- write_flags tests ---

func writeFlagsTool(name string, writeFlags []string, writableDirs []string, flagsWithValue []string) *ConfigToolSpec {
	return NewConfigTool(ConfigToolOpts{
		Name:           name,
		AllowAll:       true,
		WriteTarget:    "all",
		WriteFlags:     writeFlags,
		WritableDirs:   writableDirs,
		FlagsWithValue: flagsWithValue,
	})
}

func TestWriteFlagsTriggersCheck(t *testing.T) {
	tool := writeFlagsTool("eslint", []string{"--fix"}, []string{"/tmp"}, nil)

	// --fix present + target outside writable dirs → blocked
	res := tool.Check([]string{"--fix", "/etc/file.ts"}, RuntimeCtx{})
	if res.Decision == DecisionAllow {
		t.Error("expected blocked for --fix with target outside writable dirs")
	}

	// --fix present + target inside writable dirs → allowed
	res = tool.Check([]string{"--fix", "/tmp/file.ts"}, RuntimeCtx{})
	if res.Decision != DecisionAllow {
		t.Errorf("expected allowed for --fix with writable target, got: %s", res.Reason)
	}
}

func TestWriteFlagsAbsentSkipsCheck(t *testing.T) {
	tool := writeFlagsTool("eslint", []string{"--fix"}, []string{"/tmp"}, nil)

	// No --fix → read-only mode, target anywhere is fine
	res := tool.Check([]string{"/etc/file.ts"}, RuntimeCtx{})
	if res.Decision != DecisionAllow {
		t.Errorf("expected allowed without write flag, got: %s", res.Reason)
	}
}

func TestWriteFlagsEmptyBehavesAsDefault(t *testing.T) {
	// No write_flags → write_target always enforced (backwards compat)
	tool := NewConfigTool(ConfigToolOpts{
		Name:         "cp",
		AllowAll:     true,
		WriteTarget:  "last",
		WritableDirs: []string{"/tmp"},
	})

	res := tool.Check([]string{"src", "/etc/dst"}, RuntimeCtx{})
	if res.Decision == DecisionAllow {
		t.Error("expected blocked: no write_flags means write_target always checked")
	}
}

func TestWriteFlagsWithFlagsWithValue(t *testing.T) {
	tool := writeFlagsTool("eslint", []string{"--fix"}, []string{"/tmp"}, []string{"-c", "--config"})

	// -c consumes next arg, so /etc/file.ts is the only positional
	res := tool.Check([]string{"--fix", "-c", ".eslintrc", "/tmp/file.ts"}, RuntimeCtx{})
	if res.Decision != DecisionAllow {
		t.Errorf("expected allowed, got: %s", res.Reason)
	}

	// Without --fix, even with -c, everything is allowed (read-only)
	res = tool.Check([]string{"-c", ".eslintrc", "/etc/file.ts"}, RuntimeCtx{})
	if res.Decision != DecisionAllow {
		t.Errorf("expected allowed without write flag, got: %s", res.Reason)
	}
}

func TestWriteFlagsSedInPlace(t *testing.T) {
	tool := writeFlagsTool("sed", []string{"-i", "--in-place"}, []string{"/tmp"}, []string{"-e", "-f"})

	// sed -i 's/foo/bar/' /tmp/file → allowed
	res := tool.Check([]string{"-i", "-e", "s/foo/bar/", "/tmp/file"}, RuntimeCtx{})
	if res.Decision != DecisionAllow {
		t.Errorf("expected allowed for sed -i in writable dir, got: %s", res.Reason)
	}

	// sed -i 's/foo/bar/' /etc/file → blocked
	res = tool.Check([]string{"-i", "-e", "s/foo/bar/", "/etc/file"}, RuntimeCtx{})
	if res.Decision == DecisionAllow {
		t.Error("expected blocked for sed -i outside writable dirs")
	}

	// sed -i.bak (prefix match) → also triggers write check
	res = tool.Check([]string{"-i.bak", "-e", "s/foo/bar/", "/etc/file"}, RuntimeCtx{})
	if res.Decision == DecisionAllow {
		t.Error("expected blocked for sed -i.bak outside writable dirs")
	}
}

func TestWriteFlagsSedReadOnly(t *testing.T) {
	tool := writeFlagsTool("sed", []string{"-i", "--in-place"}, []string{"/tmp"}, []string{"-e", "-f"})

	// sed without -i → read-only, any target OK
	res := tool.Check([]string{"-e", "s/foo/bar/", "/etc/file"}, RuntimeCtx{})
	if res.Decision != DecisionAllow {
		t.Errorf("expected allowed for sed read-only, got: %s", res.Reason)
	}
}

func TestWriteFlagsPrettierShortFlag(t *testing.T) {
	tool := writeFlagsTool("prettier", []string{"--write", "-w"}, []string{"/tmp"}, nil)

	// -w triggers write check
	res := tool.Check([]string{"-w", "/etc/file.ts"}, RuntimeCtx{})
	if res.Decision == DecisionAllow {
		t.Error("expected blocked for prettier -w outside writable dirs")
	}

	// --write triggers write check
	res = tool.Check([]string{"--write", "/tmp/file.ts"}, RuntimeCtx{})
	if res.Decision != DecisionAllow {
		t.Errorf("expected allowed for prettier --write in writable dir, got: %s", res.Reason)
	}

	// --check → no write, anywhere OK
	res = tool.Check([]string{"--check", "/etc/file.ts"}, RuntimeCtx{})
	if res.Decision != DecisionAllow {
		t.Errorf("expected allowed for prettier --check, got: %s", res.Reason)
	}
}

func TestWriteFlagsLongFlagWithEquals(t *testing.T) {
	tool := writeFlagsTool("ruff", []string{"--fix"}, []string{"/tmp"}, nil)

	// --fix=true should also trigger
	res := tool.Check([]string{"--fix=true", "/etc/file.py"}, RuntimeCtx{})
	if res.Decision == DecisionAllow {
		t.Error("expected blocked for --fix=true outside writable dirs")
	}
}

func TestWriteFlagsAfterDoubleDashIgnored(t *testing.T) {
	tool := writeFlagsTool("eslint", []string{"--fix"}, []string{"/tmp"}, nil)

	// --fix after -- is not a flag, so write check is NOT triggered
	res := tool.Check([]string{"--", "--fix", "/etc/file.ts"}, RuntimeCtx{})
	if res.Decision != DecisionAllow {
		t.Errorf("expected allowed: --fix after -- should not trigger write check, got: %s", res.Reason)
	}
}

func TestWriteFlagsBeforeDoubleDashTriggersCheck(t *testing.T) {
	tool := writeFlagsTool("eslint", []string{"--fix"}, []string{"/tmp"}, nil)

	// --fix before -- triggers check; /etc/file after -- is a positional arg
	res := tool.Check([]string{"--fix", "--", "/etc/file.ts"}, RuntimeCtx{})
	if res.Decision == DecisionAllow {
		t.Error("expected blocked: --fix before -- triggers check, /etc/file.ts after -- is positional")
	}
}

func TestWriteFlagsWithWriteTargetLast(t *testing.T) {
	tool := NewConfigTool(ConfigToolOpts{
		Name:         "mytool",
		AllowAll:     true,
		WriteTarget:  "last",
		WriteFlags:   []string{"--fix"},
		WritableDirs: []string{"/tmp"},
	})

	// --fix present, last arg in writable dir → allowed
	res := tool.Check([]string{"--fix", "/etc/src", "/tmp/dst"}, RuntimeCtx{})
	if res.Decision != DecisionAllow {
		t.Errorf("expected allowed (last arg in /tmp), got: %s", res.Reason)
	}

	// --fix present, last arg outside writable dir → blocked
	res = tool.Check([]string{"--fix", "/tmp/src", "/etc/dst"}, RuntimeCtx{})
	if res.Decision == DecisionAllow {
		t.Error("expected blocked (last arg outside writable dirs)")
	}

	// No --fix → allowed anywhere (read-only mode)
	res = tool.Check([]string{"/etc/src", "/etc/dst"}, RuntimeCtx{})
	if res.Decision != DecisionAllow {
		t.Errorf("expected allowed without write flag, got: %s", res.Reason)
	}
}

func TestWriteFlagsWithSubcommandRestricted(t *testing.T) {
	tool := NewConfigTool(ConfigToolOpts{
		Name:         "lint",
		Allowed:      []string{"check", "format"},
		WriteTarget:  "last",
		WriteFlags:   []string{"--fix"},
		WritableDirs: []string{"/tmp"},
	})

	// check without --fix → allowed anywhere
	res := tool.Check([]string{"check", "/etc/file"}, RuntimeCtx{})
	if res.Decision != DecisionAllow {
		t.Errorf("expected allowed for check read-only, got: %s", res.Reason)
	}

	// check --fix in writable dir → allowed (last positional is /tmp/file)
	res = tool.Check([]string{"check", "--fix", "/tmp/file"}, RuntimeCtx{})
	if res.Decision != DecisionAllow {
		t.Errorf("expected allowed for check --fix in writable dir, got: %s", res.Reason)
	}

	// check --fix outside writable dir → blocked (last positional is /etc/file)
	res = tool.Check([]string{"check", "--fix", "/etc/file"}, RuntimeCtx{})
	if res.Decision == DecisionAllow {
		t.Error("expected blocked for check --fix outside writable dirs")
	}

	// unknown subcommand → blocked regardless of write flags
	res = tool.Check([]string{"delete", "/tmp/file"}, RuntimeCtx{})
	if res.Decision == DecisionAllow {
		t.Error("expected blocked for unknown subcommand")
	}
}

func TestWriteFlagsPresentButNoPositionalArgs(t *testing.T) {
	tool := writeFlagsTool("eslint", []string{"--fix"}, []string{"/tmp"}, nil)

	// --fix present but no file targets → allowed (nothing to write)
	res := tool.Check([]string{"--fix"}, RuntimeCtx{})
	if res.Decision != DecisionAllow {
		t.Errorf("expected allowed for --fix with no positional args, got: %s", res.Reason)
	}
}

func TestWriteFlagsEmptyArgs(t *testing.T) {
	tool := writeFlagsTool("eslint", []string{"--fix"}, []string{"/tmp"}, nil)

	// Empty args with write_flags configured → allowed (allowAll + no write flag)
	res := tool.Check([]string{}, RuntimeCtx{})
	if res.Decision != DecisionAllow {
		t.Errorf("expected allowed for empty args, got: %s", res.Reason)
	}
}

func TestWriteFlagsDevNull(t *testing.T) {
	tool := writeFlagsTool("eslint", []string{"--fix"}, []string{"/tmp"}, nil)

	// --fix with /dev/null → always allowed
	res := tool.Check([]string{"--fix", "/dev/null"}, RuntimeCtx{})
	if res.Decision != DecisionAllow {
		t.Errorf("expected /dev/null always allowed, got: %s", res.Reason)
	}
}

func TestWriteFlagsSecondFlagTriggers(t *testing.T) {
	tool := writeFlagsTool("ruff", []string{"--fix", "--fix-only"}, []string{"/tmp"}, nil)

	// Only --fix-only present (second write flag) → triggers check
	res := tool.Check([]string{"--fix-only", "/etc/file.py"}, RuntimeCtx{})
	if res.Decision == DecisionAllow {
		t.Error("expected blocked for --fix-only outside writable dirs")
	}

	// --fix-only with writable target → allowed
	res = tool.Check([]string{"--fix-only", "/tmp/file.py"}, RuntimeCtx{})
	if res.Decision != DecisionAllow {
		t.Errorf("expected allowed for --fix-only in writable dir, got: %s", res.Reason)
	}
}

func TestWriteFlagsShortPrefixDoesNotMatchUnrelated(t *testing.T) {
	// This documents that short flag prefix matching is broad:
	// -i matches -input, -if, etc. This is by design for sed -iSUFFIX.
	tool := writeFlagsTool("sed", []string{"-i"}, []string{"/tmp"}, []string{"-e"})

	// -if is matched by -i prefix → triggers write check
	res := tool.Check([]string{"-if", "-e", "s/x/y/", "/etc/file"}, RuntimeCtx{})
	if res.Decision == DecisionAllow {
		t.Error("expected blocked: -if matched by -i prefix")
	}
}

func TestWriteFlagsSedLongForm(t *testing.T) {
	tool := writeFlagsTool("sed", []string{"-i", "--in-place"}, []string{"/tmp"}, []string{"-e", "-f"})

	// --in-place triggers write check
	res := tool.Check([]string{"--in-place", "-e", "s/x/y/", "/etc/file"}, RuntimeCtx{})
	if res.Decision == DecisionAllow {
		t.Error("expected blocked for sed --in-place outside writable dirs")
	}

	// --in-place with writable target → allowed
	res = tool.Check([]string{"--in-place", "-e", "s/x/y/", "/tmp/file"}, RuntimeCtx{})
	if res.Decision != DecisionAllow {
		t.Errorf("expected allowed for sed --in-place in writable dir, got: %s", res.Reason)
	}
}

func TestWriteFlagsDenyMessageContainsInfo(t *testing.T) {
	tool := writeFlagsTool("eslint", []string{"--fix"}, []string{"/tmp"}, nil)

	res := tool.Check([]string{"--fix", "/etc/src/file.ts"}, RuntimeCtx{})
	if res.Decision == DecisionAllow {
		t.Fatal("expected blocked")
	}
	if !strings.Contains(res.Reason, "eslint") || !strings.Contains(res.Reason, "/etc/src/file.ts") {
		t.Errorf("reason should mention tool and path, got: %s", res.Reason)
	}
}

func TestConfigToolNameAndFlagsMap(t *testing.T) {
	tool := NewConfigTool(ConfigToolOpts{
		Name:           "mytool",
		AllowAll:       true,
		FlagsWithValue: []string{"-f", "--file"},
	})
	if tool.Name() != "mytool" {
		t.Errorf("expected Name()=%q, got %q", "mytool", tool.Name())
	}
	m := tool.FlagsWithValueMap()
	if !m["-f"] || !m["--file"] {
		t.Errorf("expected FlagsWithValueMap to contain -f and --file, got %v", m)
	}
	if m["--other"] {
		t.Error("expected FlagsWithValueMap not to contain --other")
	}
}

