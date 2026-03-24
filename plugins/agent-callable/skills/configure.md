---
name: configure
description: "Configure, audit, or troubleshoot agent-callable — the deny-by-default command filter for LLM agents. Use this skill when the user explicitly mentions 'agent-callable' and wants to add a custom tool (TOML), audit the current config, understand why a command is blocked, toggle builtins, or install the binary. Do NOT trigger on general 'command blocked' or 'permission denied' issues unless agent-callable is mentioned by name."
user_invocable: true
---

# agent-callable — configuration helper

## Quick reference

Config dir: `~/.config/agent-callable/`
Global config: `~/.config/agent-callable/config.toml`
Tool configs: `~/.config/agent-callable/tools.d/*.toml`

```bash
agent-callable --list-tools            # registered tools (built-in + config)
agent-callable --help-config           # full TOML format documentation
agent-callable --audit <tool> [args]   # dry-run: would this command be allowed?
agent-callable --audit --sh '<expr>'   # dry-run for shell expressions (pipes, &&)
agent-callable --init-config           # interactive: generate default configs
```

## Built-in tools

docker, gcloud, gh, git, kubectl, npm, pulumi (+ wrappers: nice, timeout, xargs).

Builtins have richer logic (flag inspection, secret filtering, auto-injected env). They are toggled in `config.toml`:
```toml
[builtins]
kubectl = true
git = true
```

## TOML config format (tools.d/)

All args allowed:
```toml
[mytool]
allowed = ["*"]
```

Subcommand allowlist:
```toml
[systemctl]
allowed = ["is-active", "is-enabled", "list-units", "status"]
```

Write-safety (only checks destination when a write flag is present):
```toml
[sed]
allowed = ["*"]
write_flags = ["-i", "--in-place"]
write_target = "last"
flags_with_value = ["-e", "-f", "--expression", "--file"]
```

Nested subcommands:
```toml
[nmcli]
allowed = ["g", "general", "c", "connection"]
flags_with_value = ["-f", "--fields"]
[nmcli.subcommands]
g = ["status", "hostname"]
general = ["status", "hostname"]
```

Environment overrides:
```toml
[git]
allowed = ["status", "log", "diff"]
[git.env]
GIT_TERMINAL_PROMPT = "0"
```

### Fields reference

- `mode` — `"replace"` (default) or `"extend"`. See Extend mode below.
- `allowed` (required for replace) — `["*"]` for unrestricted, or `["sub1", "sub2"]` for allowlist
- `flags_with_value` — flags that consume the next arg (prevents confusing a flag value with a subcommand)
- `write_target` — `"last"` (cp, mv, sed -i) or `"all"` (mkdir, touch, tee): destination args checked against `writable_dirs`
- `write_flags` — only enforce `write_target` when one of these flags is present (e.g. `-i` for sed)
- `[tool.env]` — env vars applied when the command is allowed
- `[tool.subcommands]` — second-level subcommand allowlists (keys must be a subset of `allowed`)

## Extend mode (extending builtins)

Builtins (git, kubectl, gh, docker, npm, pulumi, gcloud) have rich Go logic — flag inspection, secret filtering, actionable deny messages. A plain TOML config for the same tool name would **replace** the builtin entirely, losing all that logic.

`mode = "extend"` solves this: the builtin runs first. If it allows, done. If it denies, the TOML fallback is consulted. This is a "sandwich" — keep the builtin's safety net, add specific exceptions via TOML.

```toml
# Allow git push (builtin blocks it) without losing git's other protections:
[git]
mode = "extend"
allowed = ["push"]

# Allow gh pr create while keeping gh's api write-method detection:
[gh]
mode = "extend"
allowed = ["pr"]
[gh.subcommands]
pr = ["create"]

# Allow kubectl apply for a specific workflow:
[kubectl]
mode = "extend"
allowed = ["apply"]
```

**When to use extend vs replace:**
- The tool is a **builtin** and you want to unlock specific subcommands → `extend`
- The tool is a **builtin** and you want full control over its policy → `replace` (you lose the builtin logic)
- The tool is **not a builtin** (config-only) → `replace` (default, extend would warn)

**When unsure**: ask the user. Typical question: "git push est bloqué par le builtin. Voulez-vous l'autoriser via extend (conserve la logique de sécurité git pour le reste) ou remplacer le builtin entièrement ?"

### Global config (config.toml)

```toml
writable_dirs = ["/tmp"]

[audit]
file = "~/.local/share/agent-callable/audit.log"
mode = "blocked"          # "none" | "blocked" | "allowed" | "all"
max_entries = 10000
mask_secrets = true

[builtins]
# git = true
# kubectl = true
```

## Workflow: add a new tool

1. `agent-callable --list-tools` — check if already registered
   - If it shows `[built-in]` and you need to unlock specific subcommands → use **extend mode**
   - If it shows `[config]` → edit the existing TOML
   - If not listed → create new TOML config
2. **Research the tool's CLI surface** before writing config:
   - Use context7 MCP (`resolve-library-id` then `query-docs`) if available
   - Otherwise `<tool> --help` and web search for the official docs
   - Identify: all subcommands (including aliases), dangerous operations (write/delete/push/apply), flags that take a value argument
   - This step matters — CLIs have flags and subcommands you don't expect, and missing a `flags_with_value` entry causes subtle parsing bugs
3. Create or edit a `.toml` in `~/.config/agent-callable/tools.d/`
4. Verify: `agent-callable --audit <tool> <expected-args>` (should pass)
5. Verify: `agent-callable --audit <tool> <dangerous-args>` (should block)

## Workflow: command unexpectedly blocked

1. `agent-callable --audit <tool> <args>` — shows the deny reason
2. `agent-callable --audit --sh '<full expression>'` — for pipes/shell
3. `agent-callable --list-tools | grep <tool>` — is the tool registered?
4. If missing → research the tool's docs (context7 MCP or web), then add TOML config
5. If `[built-in]` and a specific subcommand is blocked → use `mode = "extend"` to add it as a TOML fallback (keeps the builtin's safety logic for everything else)
6. If `[config]` but wrong → edit the relevant `.toml` in `tools.d/`
7. **Ask the user before extending a builtin** — extending unlocks a command the builtin intentionally blocks (e.g. `git push`, `kubectl apply`). Confirm this is desired.

## Installation (if binary not found)

```bash
which agent-callable || {
  gh release download --repo Evaneos/agent-callable --pattern '*_linux_amd64.tar.gz' -D /tmp
  tar xzf /tmp/agent-callable_*_linux_amd64.tar.gz -C /tmp
  echo "Move /tmp/agent-callable to a directory in your PATH"
}
agent-callable --init-config   # generate default configs
```

## Design principles for writing configs

- **Deny-by-default**: if not explicitly allowed, it's blocked. Prefer blocking too much over too little.
- Read the tool's `--help` before writing a config — CLIs have flags you don't expect.
- Use `write_target`/`write_flags` for any tool that can write files (sed -i, eslint --fix, gofmt -w).
- Declare `flags_with_value` so the parser doesn't confuse flag values with subcommands.
