package spec

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// ConfigToolOpts contains the parameters needed to build a ConfigToolSpec.
// Uses basic Go types so callers don't depend on TOML structs.
type ConfigToolOpts struct {
	Name           string
	Allowed        []string
	FlagsWithValue []string
	Subcommands    map[string][]string
	Env            map[string]string
	AllowAll       bool
	WriteTarget    string   // "last", "all", or "" (no write-target check)
	WriteFlags     []string // flags that trigger write_target checking (e.g. "--fix", "-i")
	WritableDirs   []string // from GlobalConfig
}

// ConfigToolSpec implements ToolSpec from a config-driven tool definition.
type ConfigToolSpec struct {
	name           string
	env            map[string]string
	allowAll       bool
	allowedSet     map[string]bool
	flagsWithValue map[string]bool
	subcommands    map[string][]string
	writeTarget    string
	writeFlags     []string // preserved as slice for prefix matching on short flags
	writableDirs   []string
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
		name:           opts.Name,
		env:            opts.Env,
		allowAll:       opts.AllowAll,
		allowedSet:     allowed,
		flagsWithValue: flags,
		subcommands:    opts.Subcommands,
		writeTarget:    opts.WriteTarget,
		writeFlags:     opts.WriteFlags,
		writableDirs:   opts.WritableDirs,
	}
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

	if t.allowAll {
		return t.checkWriteTarget(args)
	}

	if len(args) == 0 {
		return Deny(fmt.Sprintf("%s requires a subcommand", t.name))
	}

	cmd := NthNonFlag(args, 1, t.flagsWithValue)
	if cmd == "" {
		return Deny(fmt.Sprintf("%s subcommand not found", t.name))
	}

	if !t.allowedSet[cmd] {
		return Deny(fmt.Sprintf("command %s %q not allowed", t.name, cmd))
	}

	if subs, ok := t.subcommands[cmd]; ok {
		sub := NthNonFlag(args, 2, t.flagsWithValue)
		if sub == "" {
			return Deny(fmt.Sprintf("%s %s requires a subcommand", t.name, cmd))
		}
		if slices.Contains(subs, sub) {
			return t.checkWriteTarget(args)
		}
		return Deny(fmt.Sprintf("command %s %s %q not allowed", t.name, cmd, sub))
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

	switch t.writeTarget {
	case "last":
		if len(positional) == 0 {
			return Allow()
		}
		target := positional[len(positional)-1]
		if !IsUnderWritableDir(target, t.writableDirs) {
			return Deny(fmt.Sprintf("%s: write target %q outside writable directories", t.name, target))
		}
	case "all":
		for _, target := range positional {
			if !IsUnderWritableDir(target, t.writableDirs) {
				return Deny(fmt.Sprintf("%s: write target %q outside writable directories", t.name, target))
			}
		}
	}

	return Allow()
}

// hasWriteFlag checks whether any write flag is present in args.
// Long flags (--foo) match exactly or with = (--foo=bar).
// Short flags (-x) match by prefix (-x, -x.bak, -xSUFFIX).
func (t *ConfigToolSpec) hasWriteFlag(args []string) bool {
	for _, a := range args {
		if a == "--" {
			break
		}
		for _, wf := range t.writeFlags {
			if strings.HasPrefix(wf, "--") {
				// Long flag: exact or --flag=value
				if a == wf || strings.HasPrefix(a, wf+"=") {
					return true
				}
			} else {
				// Short flag: prefix match (-i matches -i, -i.bak, -i'')
				if a == wf || (strings.HasPrefix(a, wf) && len(a) > len(wf)) {
					return true
				}
			}
		}
	}
	return false
}
