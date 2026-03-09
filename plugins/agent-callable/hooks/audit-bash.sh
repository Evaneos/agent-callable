#!/bin/bash
# PreToolUse:Bash — delegates to agent-callable --claude.
# Always exit 0. Empty stdout = abstain. JSON stdout = allow.
# Debug: AGENT_CALLABLE_HOOK_DEBUG=1 logs to /tmp/agent-callable-hook.log

exec 2>/dev/null

log() { [[ -n "$AGENT_CALLABLE_HOOK_DEBUG" ]] && printf '%s %s\n' "$(date +%T)" "$*" >> /tmp/agent-callable-hook.log; }

input=$(cat) || true
command=$(printf '%s' "$input" | jq -r '.tool_input.command // empty') || true

if [[ -z "$command" ]]; then
  log "empty command, abstain"
  exit 0
fi

if [[ "$command" == agent-callable\ * ]]; then
  log "agent-callable prefix, allow (will self-check)"
  printf '{"decision":"allow","reason":"agent-callable self-validates"}\n'
  exit 0
fi

result=$(agent-callable --claude "$command") || true

if [[ -n "$result" ]]; then
  log "allow: $command"
  printf '%s\n' "$result"
else
  log "abstain: $command"
fi

exit 0
