package config

import (
	"cmp"
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/evaneos/agent-callable/internal/spec"

	"github.com/BurntSushi/toml"
)

//go:embed help.txt
var HelpText string

// AuditConfig configures the audit trail.
type AuditConfig struct {
	File               string `toml:"file"`                 // path to the audit log file (empty = disabled)
	Mode               string `toml:"mode"`                 // "blocked", "allowed", "all" (default: "all")
	MaxEntries         int    `toml:"max_entries"`          // max log lines kept (0 = unlimited)
	MaskSecrets        bool   `toml:"mask_secrets"`         // mask sensitive values in logged commands
	IncludeAuditChecks bool   `toml:"include_audit_checks"` // log --audit and --claude dry-run checks
}

// GlobalConfig is loaded from config.toml (not tools.d/).
type GlobalConfig struct {
	WritableDirs []string        `toml:"writable_dirs"`
	AllowOnAny   []string        `toml:"allow_on_any"`
	Audit        AuditConfig     `toml:"audit"`
	Builtins     map[string]bool `toml:"builtins"`
}

// ToolConfig represents a tool section in a TOML config file.
// The Name field is set from the TOML section key, not from the file content.
type ToolConfig struct {
	Name           string              `toml:"name"`
	Mode           string              `toml:"mode"`
	Allowed        []string            `toml:"allowed"`
	Subcommands    map[string][]string `toml:"subcommands"`
	Env            map[string]string   `toml:"env"`
	FlagsWithValue []string            `toml:"flags_with_value"`
	WriteTarget    string              `toml:"write_target"`
	WriteFlags     []string            `toml:"write_flags"`
	DeniedFlags    []string            `toml:"denied_flags"`
}

// ConfigTool is the internal representation after loading.
type ConfigTool struct {
	ToolConfig
	AllowAll bool // true when allowed = ["*"]
}

var nameRe = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

func validateFlagPrefix(field string, flags []string) error {
	for _, f := range flags {
		if !strings.HasPrefix(f, "-") {
			return fmt.Errorf("%s entry %q must start with -", field, f)
		}
	}
	return nil
}

var validWriteTargets = []string{spec.WriteTargetLast, spec.WriteTargetAll, spec.WriteTargetAfterFirst}

func validateWriteTarget(name, target string) error {
	if target == "" || slices.Contains(validWriteTargets, target) {
		return nil
	}
	return fmt.Errorf("[%s]: write_target must be one of %q (got %q)", name, validWriteTargets, target)
}

func (c *ToolConfig) validateFlags() error {
	flagFields := []struct {
		name  string
		flags []string
	}{
		{"flags_with_value", c.FlagsWithValue},
		{"write_flags", c.WriteFlags},
		{"denied_flags", c.DeniedFlags},
	}
	for _, f := range flagFields {
		if err := validateFlagPrefix(f.name, f.flags); err != nil {
			return err
		}
	}
	if len(c.WriteFlags) > 0 && c.WriteTarget == "" {
		return fmt.Errorf("[%s]: write_flags requires write_target", c.Name)
	}
	return validateWriteTarget(c.Name, c.WriteTarget)
}

// Validate checks a ToolConfig for structural correctness.
func (c *ToolConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("name is required")
	}
	if !nameRe.MatchString(c.Name) {
		return fmt.Errorf("name %q must match ^[a-zA-Z0-9._-]+$", c.Name)
	}
	if c.Mode != "" && c.Mode != "replace" && c.Mode != "extend" {
		return fmt.Errorf("[%s]: mode must be \"replace\" or \"extend\" (got %q)", c.Name, c.Mode)
	}
	if c.Mode == "extend" {
		// Extend mode: relaxed validation — only check flags format.
		return c.validateFlags()
	}
	if len(c.Allowed) == 0 {
		return fmt.Errorf("[%s]: allowed is required", c.Name)
	}
	if err := c.validateFlags(); err != nil {
		return err
	}
	if slices.Contains(c.Allowed, "*") {
		if len(c.Allowed) > 1 {
			return fmt.Errorf("[%s]: allowed = [\"*\"] cannot be mixed with other values", c.Name)
		}
		return nil
	}
	for k := range c.Subcommands {
		if !slices.Contains(c.Allowed, k) {
			return fmt.Errorf("subcommand key %q is not in allowed list", k)
		}
	}
	return nil
}

// xdgBaseDir returns the XDG-based config directory for agent-callable.
// Uses XDG_CONFIG_HOME if set, otherwise ~/.config.
func xdgBaseDir() string {
	home, _ := os.UserHomeDir()
	xdg := cmp.Or(os.Getenv("XDG_CONFIG_HOME"), filepath.Join(home, ".config"))
	return filepath.Join(xdg, "agent-callable")
}

// SearchDirs returns the directories to search for tool config files.
// When AGENT_CALLABLE_CONFIG_DIR is set, only that directory is used (exclusive override).
func SearchDirs() []string {
	if d := os.Getenv("AGENT_CALLABLE_CONFIG_DIR"); d != "" {
		return []string{filepath.Join(d, "tools.d")}
	}
	return []string{filepath.Join(xdgBaseDir(), "tools.d")}
}

// ConfigBaseDir returns the base config directory (parent of tools.d/).
func ConfigBaseDir() string {
	if d := os.Getenv("AGENT_CALLABLE_CONFIG_DIR"); d != "" {
		return d
	}
	return xdgBaseDir()
}

// parseSections decodes a TOML string where each top-level section is a tool.
func parseSections(content string) ([]ConfigTool, error) {
	var raw map[string]ToolConfig
	if err := toml.Unmarshal([]byte(content), &raw); err != nil {
		return nil, err
	}
	var configs []ConfigTool
	for name, tc := range raw {
		tc.Name = name
		isWild := len(tc.Allowed) == 1 && tc.Allowed[0] == "*"
		if err := tc.Validate(); err != nil {
			return nil, err
		}
		configs = append(configs, ConfigTool{ToolConfig: tc, AllowAll: isWild})
	}
	return configs, nil
}

// LoadDir loads all .toml files from a directory.
// Each file defines tools as [toolname] sections.
func LoadDir(dir string) ([]ConfigTool, []error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, []error{fmt.Errorf("reading %s: %w", dir, err)}
	}

	var configs []ConfigTool
	var errs []error

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".toml") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			errs = append(errs, fmt.Errorf("reading %s: %w", path, err))
			continue
		}
		parsed, err := parseSections(string(data))
		if err != nil {
			errs = append(errs, fmt.Errorf("parsing %s: %w", path, err))
			continue
		}
		configs = append(configs, parsed...)
	}
	return configs, errs
}

// LoadGlobalConfig loads config.toml from the base config directory.
func LoadGlobalConfig() (*GlobalConfig, error) {
	path := filepath.Join(ConfigBaseDir(), "config.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &GlobalConfig{}, nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var gc GlobalConfig
	if err := toml.Unmarshal(data, &gc); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &gc, nil
}

// ParseTOML parses a TOML string into ConfigTools.
func ParseTOML(content string) ([]ConfigTool, error) {
	return parseSections(content)
}

// LoadAll loads configs from all search directories (first match wins per name),
// then merges embedded defaults (lowest priority). Also loads the global config.
func LoadAll(embeddedDefaults ...map[string]string) ([]ConfigTool, *GlobalConfig, []error) {
	seen := make(map[string]bool)
	var all []ConfigTool
	var allErrs []error

	for _, dir := range SearchDirs() {
		configs, errs := LoadDir(dir)
		allErrs = append(allErrs, errs...)
		for _, c := range configs {
			if !seen[c.Name] {
				seen[c.Name] = true
				all = append(all, c)
			}
		}
	}

	// Merge embedded defaults (lowest priority: disk configs win).
	for _, defaults := range embeddedDefaults {
		for filename, content := range defaults {
			configs, err := ParseTOML(content)
			if err != nil {
				allErrs = append(allErrs, fmt.Errorf("parsing embedded %s: %w", filename, err))
				continue
			}
			for _, c := range configs {
				if !seen[c.Name] {
					seen[c.Name] = true
					all = append(all, c)
				}
			}
		}
	}

	gc, err := LoadGlobalConfig()
	if err != nil {
		allErrs = append(allErrs, err)
		gc = &GlobalConfig{}
	}

	return all, gc, allErrs
}
