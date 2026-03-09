package config

import "github.com/evaneos/agent-callable/internal/spec"

// ToToolSpec converts a ConfigTool to a spec.ToolSpec.
// writableDirs is passed from GlobalConfig for write_target checking.
func (ct ConfigTool) ToToolSpec(writableDirs []string) *spec.ConfigToolSpec {
	return spec.NewConfigTool(spec.ConfigToolOpts{
		Name:           ct.ToolConfig.Name,
		Allowed:        ct.ToolConfig.Allowed,
		FlagsWithValue: ct.ToolConfig.FlagsWithValue,
		Subcommands:    ct.ToolConfig.Subcommands,
		Env:            ct.ToolConfig.Env,
		AllowAll:       ct.AllowAll,
		WriteTarget:    ct.ToolConfig.WriteTarget,
		WriteFlags:     ct.ToolConfig.WriteFlags,
		WritableDirs:   writableDirs,
	})
}
