package spec

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Valid values for ConfigToolOpts.WriteTarget.
const (
	WriteTargetLast       = "last"
	WriteTargetAll        = "all"
	WriteTargetAfterFirst = "after_first"
)

// ConfigToolOpts contains the parameters needed to build a ConfigToolSpec.
// Uses basic Go types so callers don't depend on TOML structs.
type ConfigToolOpts struct {
	Name               string
	Allowed            []string
	FlagsWithValue     []string
	Subcommands        map[string][]string
	Env                map[string]string
	AllowAll           bool
	WriteTarget        string   // "last", "all", "after_first", or "" (no write-target check)
	WriteFlags         []string // flags that trigger write_target checking (e.g. "--fix", "-i")
	DeniedFlags        []string // flags that block execution outright (e.g. "-exec", "-delete", "-O")
	WritableDirs       []string // from GlobalConfig
	StripVersionSuffix bool     // strip a trailing "@version" (npm package-spec syntax, e.g. "prettier@3") before matching the command against Allowed
}

// ConfigToolSpec implements ToolSpec from a config-driven tool definition.
type ConfigToolSpec struct {
	name               string
	env                map[string]string
	allowAll           bool
	allowedSet         map[string]bool
	flagsWithValue     map[string]bool
	subcommands        map[string][]string
	writeTarget        string
	writeFlags         []string // preserved as slice for prefix matching on short flags
	deniedFlags        []string
	writableDirs       []string
	stripVersionSuffix bool
}

// NewConfigTool creates a ToolSpec from basic Go parameters.
func NewConfigTool(opts ConfigToolOpts) *ConfigToolSpec {
	allowed := make(map[string]bool, len(opts.Allowed))
	for _, a := range opts.Allowed {
		allowed[a] = true
	}
	flags := make(map[string]bool, len(opts.FlagsWithValue))
	for _, f := range opts.FlagsWithValue {
		flags[f] = true
	}
	return &ConfigToolSpec{
		name:               opts.Name,
		env:                opts.Env,
		allowAll:           opts.AllowAll,
		allowedSet:         allowed,
		flagsWithValue:     flags,
		subcommands:        opts.Subcommands,
		writeTarget:        opts.WriteTarget,
		writeFlags:         opts.WriteFlags,
		deniedFlags:        opts.DeniedFlags,
		writableDirs:       opts.WritableDirs,
		stripVersionSuffix: opts.StripVersionSuffix,
	}
}

// stripVersionSuffixFrom removes a trailing "@version" from an npm-style
// package spec (e.g. "prettier@3" -> "prettier"). A leading "@" (scoped
// package, e.g. "@angular/cli") is not itself a version separator, and
// neither is a "/" or ":" after the last "@": that's npm's syntax for a
// git/tarball/local-path/registry-alias reference (e.g. "prettier@npm:x",
// "prettier@file:/tmp/x"), left untouched.
func stripVersionSuffixFrom(pkg string) string {
	idx := strings.LastIndex(pkg, "@")
	if idx <= 0 {
		return pkg
	}
	if strings.ContainsAny(pkg[idx+1:], ":/") {
		return pkg
	}
	return pkg[:idx]
}

func (t *ConfigToolSpec) Name() string { return t.name }

// FlagsWithValueMap returns the map of flags that consume the next argument.
func (t *ConfigToolSpec) FlagsWithValueMap() map[string]bool { return t.flagsWithValue }

func (t *ConfigToolSpec) NonInteractiveEnv() map[string]string {
	if len(t.env) == 0 {
		return nil
	}
	env := make(map[string]string, len(t.env))
	maps.Copy(env, t.env)
	return env
}

func (t *ConfigToolSpec) Check(args []string, _ RuntimeCtx) Result {
	// Control characters are checked by the engine before calling Check.

	if denied := t.matchDeniedFlag(args); denied != "" {
		return Deny(fmt.Sprintf("%s: flag %q is denied", t.name, denied))
	}

	if t.allowAll {
		return t.checkWriteTarget(args)
	}

	if len(args) == 0 {
		return Deny(fmt.Sprintf("%s: subcommand required", t.name))
	}

	cmd := NthNonFlag(args, 1, t.flagsWithValue)
	if cmd == "" {
		return Deny(fmt.Sprintf("%s: subcommand not found", t.name))
	}
	if t.stripVersionSuffix {
		cmd = stripVersionSuffixFrom(cmd)
	}

	if !t.allowedSet[cmd] {
		return Deny(fmt.Sprintf("%s: subcommand %q not allowed", t.name, cmd))
	}

	if subs, ok := t.subcommands[cmd]; ok {
		sub := NthNonFlag(args, 2, t.flagsWithValue)
		if sub == "" {
			return Deny(fmt.Sprintf("%s: %s requires a subcommand", t.name, cmd))
		}
		if slices.Contains(subs, sub) {
			return t.checkWriteTarget(args)
		}
		return Deny(fmt.Sprintf("%s: %s subcommand %q not allowed", t.name, cmd, sub))
	}

	return t.checkWriteTarget(args)
}

func (t *ConfigToolSpec) checkWriteTarget(args []string) Result {
	if t.writeTarget == "" {
		return Allow()
	}

	// When write_flags is set, only enforce write_target if a write flag is present.
	if len(t.writeFlags) > 0 && !t.hasWriteFlag(args) {
		return Allow()
	}

	positional := AllPositionalArgs(args, t.flagsWithValue)

	var targets []string
	switch t.writeTarget {
	case WriteTargetLast:
		if len(positional) == 0 {
			return Allow()
		}
		targets = positional[len(positional)-1:]
	case WriteTargetAll:
		targets = positional
	case WriteTargetAfterFirst:
		if len(positional) <= 1 {
			return Allow()
		}
		targets = positional[1:]
	}

	for _, target := range targets {
		if !IsUnderWritableDir(target, t.writableDirs) {
			return Deny(fmt.Sprintf("%s: write target %q outside writable directories", t.name, target))
		}
	}
	return Allow()
}

// matchDeniedFlag returns the first denied flag found in args, or "" if none.
// Matching is exact only — no short-flag prefix matching — to avoid false
// positives like `-d` matching `-delete`. Tokens after `--` are ignored.
func (t *ConfigToolSpec) matchDeniedFlag(args []string) string {
	if len(t.deniedFlags) == 0 {
		return ""
	}
	return FirstMatchingFlag(args, t.deniedFlags, MatchFlag)
}

// hasWriteFlag checks whether any write flag is present in args.
// Short flags (-x) additionally match by prefix (-x.bak, -xSUFFIX).
func (t *ConfigToolSpec) hasWriteFlag(args []string) bool {
	return FirstMatchingFlag(args, t.writeFlags, MatchFlagOrShortPrefix) != ""
}
