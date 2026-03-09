# agent-callable

A PreToolUse hook automatically validates Bash commands before execution.

Commands recognized as allowed are approved without asking for confirmation.
Unrecognized commands follow normal Claude Code behavior.

IMPORTANT: This plugin works as a transparent hook. Run commands normally (e.g. `gh issue create`, `kubectl get pods`). NEVER wrap or prefix commands with the binary used by this plugin — the validation happens automatically before execution.
