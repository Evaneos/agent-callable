package audit

import "regexp"

const masked = "****"

// Flags whose next token (separated by space or =) is a secret value.
var sensitiveFlags = regexp.MustCompile(
	`(?i)(--?(?:token|password|passwd|secret|api-key|api_key|auth|credentials|client-secret|client_secret))([ =])(\S+)`)

// Bearer / Basic auth tokens.
var bearerRe = regexp.MustCompile(`(?i)(Bearer|Basic)\s+(\S+)`)

// Environment variable assignments: VAR=value.
// We mask the value by default, unless the variable name is in the allowlist.
var envAssignRe = regexp.MustCompile(`\b([A-Z_][A-Z0-9_]*)=(\S+)`)

// Variables known to be non-sensitive (system + agent-callable internals).
var safeEnvVars = map[string]bool{
	// System / shell
	"TERM": true, "PATH": true, "HOME": true, "SHELL": true,
	"USER": true, "LOGNAME": true, "LANG": true, "LANGUAGE": true,
	"DISPLAY": true, "WAYLAND_DISPLAY": true,
	"EDITOR": true, "VISUAL": true, "PAGER": true,
	"TZ": true, "PWD": true, "OLDPWD": true,
	"SHLVL": true, "COLUMNS": true, "LINES": true,
	"COLORTERM": true, "FORCE_COLOR": true, "NO_COLOR": true, "CLICOLOR": true,
	"TMPDIR": true, "HOSTNAME": true,
	// agent-callable tool env overrides
	"GIT_TERMINAL_PROMPT":      true,
	"GH_PROMPT_DISABLED":       true,
	"HELM_DIFF_OUTPUT":         true,
	"AGENT_CALLABLE_CONFIG_DIR": true,
}

// safeEnvPrefixes are prefixes for variable names that are always safe.
var safeEnvPrefixes = []string{"LC_", "XDG_"}

func isEnvSafe(name string) bool {
	if safeEnvVars[name] {
		return true
	}
	for _, p := range safeEnvPrefixes {
		if len(name) > len(p) && name[:len(p)] == p {
			return true
		}
	}
	return false
}

// maskSecrets replaces sensitive values in a command string with ****.
func maskSecrets(cmd string) string {
	cmd = sensitiveFlags.ReplaceAllString(cmd, "${1}${2}"+masked)
	cmd = bearerRe.ReplaceAllString(cmd, "${1} "+masked)
	cmd = envAssignRe.ReplaceAllStringFunc(cmd, func(match string) string {
		sub := envAssignRe.FindStringSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		if isEnvSafe(sub[1]) {
			return match
		}
		return sub[1] + "=" + masked
	})
	return cmd
}
