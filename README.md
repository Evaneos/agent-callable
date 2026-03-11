# agent-callable

[![CI](https://github.com/Evaneos/agent-callable/actions/workflows/ci.yaml/badge.svg)](https://github.com/Evaneos/agent-callable/actions/workflows/ci.yaml)
[![Release](https://img.shields.io/badge/release-v0.13.0-blue)](https://github.com/Evaneos/agent-callable/releases/latest)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Tired of endless prompts from Claude Code?

```
 Do you want to proceed?
❯ 1. Yes
  2. Yes, and don't ask again for this risk-free command
  3. No
```

My job has turned into approving Claude in a loop across a bunch of split terminals. Yes. Tab. Yes. Tab. Yes. Oops — no, that one was actually a question it was asking me. You know the drill.

On the other hand, I don't feel lucky enough to drop all permissions and let any command run. I want Claude to stop asking me to approve harmless commands — or commands with harmless side effects (yes, you can create a file in my workspace, that's kind of the point). But when it's about to do something truly stupid ("oh right, if I had run that command it would have wiped all your prod data"), I want the normal Claude prompt back.

Claude Code has a built-in, well-documented mechanism for this: allowlists in the settings. But the granularity is not great. Allowing `Bash(kubectl get:*)` lacks precision on subcommands and fails to match `kubectl -c foobar get pod`.

**agent-callable** grew out of [kubectl-readonly](https://github.com/Evaneos/kubectl-readonly), a kubectl wrapper that only allows read-only operations. The kubectl policy engine (`kubepolicy`) proved useful enough that I generalized the approach to cover every CLI tool an agent might call. agent-callable imports `kubepolicy` directly for its kubectl filtering — same battle-tested rules, broader scope.

**agent-callable** is two things:

- A binary that filters shell commands based on TOML config files for simple cases, plus edge cases handled in Go.
- A Claude Code plugin that wraps agent-callable in a transparent PreToolUse hook.

The result: with my agent-callable config, Claude Code no longer prompts me for anything I consider risk-free. Everything else gets the normal prompt. It's 100% transparent.

> **No security guarantee.** This tool filters commands to reduce accidental side effects from LLM agents. It is not a sandbox, does not isolate processes, and a determined or creative agent may find ways around it. Use it as a convenience layer, not as a security boundary.

## Quick start

### 1. Install the binary

```bash
go install github.com/evaneos/agent-callable/cmd/agent-callable@latest
```

Then generate the default config:

```bash
agent-callable --init-config    # creates ~/.config/agent-callable/
```

The binary is usable standalone at this point — any LLM agent can call `agent-callable <command>` to run filtered commands.

### 2. Install the Claude Code plugin

Add the marketplace and install the plugin:

```
/plugin marketplace add Evaneos/agent-callable
/plugin install agent-callable@Evaneos/agent-callable
```

That's it — every Bash command now goes through the filter, no allowlist to maintain, no CLAUDE.md to write.

---

## How it works

### As a Claude Code plugin (recommended)

The plugin installs a [PreToolUse hook](https://docs.anthropic.com/en/docs/agents-and-tools/claude-code/hooks). Every time Claude is about to run a Bash command, the hook quietly calls `agent-callable` under the hood:

1. Claude generates a Bash command — it doesn't know the hook exists
2. The hook passes the command to `agent-callable --claude`
3. Allowed → auto-approve, no prompt
4. Not allowed → the hook steps aside, Claude Code shows the normal prompt

The hook never blocks anything itself. It just fast-tracks the boring stuff.

Set `AGENT_CALLABLE_HOOK_DEBUG=1` to log hook decisions to `/tmp/agent-callable-hook.log`.

### As a standalone binary

Outside Claude Code — or in any context where an LLM runs shell commands — the agent prefixes its commands with `agent-callable`:

```bash
agent-callable kubectl get pods -A         # allowed
agent-callable git push                    # blocked
agent-callable --sh 'git log | head -5'    # compound shell expression
```

This requires telling the agent to use the prefix (via CLAUDE.md or equivalent — see `SAMPLE_CLAUDE.md`). The plugin is simpler since it requires no instructions.

---

## What gets filtered

Three categories of side effects:

| Category | Verdict | Examples |
|---|---|---|
| **Remote effect** — modifies an external service | blocked | `git push`, `kubectl apply`, `gh pr create` |
| **Persistent config change** — durably alters a tool's behavior | blocked | `helm repo add`, `gcloud config set` |
| **Local cache/artifact write** — useful for investigation | allowed | `git fetch`, `docker pull`, `gh repo clone` |

The rule is simple: **when in doubt, block.** A false positive (unnecessary prompt) is annoying. A false negative (wiped prod database) is not.

---

## Supported tools

Out of the box, agent-callable ships with built-in filters for 12+ CLI tools. Each one has hand-tuned rules in Go.

<details>
<summary><strong>Built-in tools</strong> (click to expand)</summary>

- **kubectl** — read-only commands, blocks `apply/delete/edit/patch`, filters out secret content
- **git** — investigation + local writes (clone/fetch/checkout/add/commit/mv/rm), blocks remote writes and force flags
- **gh** — read-only + clone/checkout, blocks PR create/merge, issue mutations
- **docker** — inspection + `pull` + `run` with restrictions (no `--privileged`, no host network/pid/ipc, RW mounts only under `writable_dirs`)
- **docker-compose** — inspection only (`ps/logs/config/images`)
- **flux** — `version`, `get ...`, `logs`
- **pulumi** — info + `preview` (auto-injects `--non-interactive`), blocks `--show-secrets`
- **helm** — read-only (`list/status/history/get/show/template/lint/search`)
- **kustomize** — `build` + `cfg` read-only
- **gcloud** — conservative allowlist (list/describe/get/show/read/logs)
- **npm** — read-only + `install/ci` with `--ignore-scripts` + `run` restricted to safe scripts (test, lint, build, etc.)
- **kubectx**, **kubectl-crossplane**, **krew** — read-only
- **xargs**, **timeout**, **nice** — wrapper tools: validate the inner command recursively against the same policy

</details>

Beyond built-ins, default TOML configs add:
- **Text processing** — `sed`, `yq` with conditional write checking (`-i` triggers `writable_dirs`)
- **TypeScript** — `tsc`, `eslint` (`--fix` triggers `writable_dirs`), `prettier` (`--write` triggers `writable_dirs`)
- **Go** — `gofmt` (`-w` triggers `writable_dirs`), `go` (test/build/vet/mod/...)
- **Python** — `ruff` (`--fix` triggers `writable_dirs`), `uv` (`run` restricted to safe commands like pytest/mypy/ruff), `ty`
- And many more (filesystem, network, system info, etc.) — see `agent-callable --list-tools`

---

## Configuration

Everything lives in `~/.config/agent-callable/`. Run `agent-callable --init-config` to generate sensible defaults, or hand-craft your own.

### Adding tools with TOML

Drop files in `~/.config/agent-callable/tools.d/`:

```toml
# Read-only tool: all arguments allowed
[grep]
allowed = ["*"]

# Restricted subcommands
[systemctl]
allowed = ["is-active", "is-enabled", "list-units", "status"]

# Write tool: destination checked against writable_dirs
[cp]
allowed = ["*"]
write_target = "last"
flags_with_value = ["-t", "--target-directory"]

# Conditional write: only check writable_dirs when a write flag is present
[sed]
allowed = ["*"]
write_flags = ["-i", "--in-place"]
write_target = "last"
flags_with_value = ["-e", "-f", "--expression", "--file"]
```

`write_target` controls which arguments are checked against `writable_dirs`:
- `"last"` — last positional arg is the destination (`cp`, `mv`, `ln`, `sed -i`)
- `"all"` — all positional args are destinations (`mkdir`, `touch`, `tee`, `eslint --fix`)

`write_flags` makes `write_target` conditional: the check is only enforced when one of the listed flags is present. Without the flag, the command runs freely (read-only mode). Short flags match by prefix (`-i` matches `-i.bak`), long flags match exactly or with `=` (`--fix` matches `--fix=true`).

Built-in tools always take priority over config files.

### Global settings

`~/.config/agent-callable/config.toml`:

```toml
writable_dirs = ["/tmp"]     # enforced on: redirects, docker volumes, write_target tools

[audit]
file = "~/.local/share/agent-callable/audit.log"  # parent dir auto-created
mode = "none"           # "none", "blocked", "allowed", "all"
max_entries = 10000     # oldest trimmed on open (0 = unlimited)
mask_secrets = true     # mask tokens, passwords, env vars in logged commands
```

---

## Shell mode (`--sh`)

Claude Code rarely runs simple commands. It chains pipes, loops, and conditionals. agent-callable parses the full shell AST — if a compound expression contains only control flow (`for`, `if`, `&&`, `||`, pipes) and allowed commands, the entire expression is auto-approved without prompting.

```bash
agent-callable --sh 'kubectl get pods | grep Running'
agent-callable --sh 'for ns in prod staging; do kubectl get pods -n $ns; done'
agent-callable --sh 'git status && git diff --stat'
```

This mode is deliberately weaker than single-command mode on argument checking: variables like `$ns` can't be resolved statically, so only command names are validated. Dynamic commands (`$CMD args`) and builtins that could bypass validation (`eval`, `exec`, `source`) are blocked. Write redirections are limited to `/dev/null` and `writable_dirs`.

---

## Other flags

```bash
agent-callable --audit <tool> [args...]       # dry-run: check without executing
agent-callable --audit --sh '<expression>'    # dry-run: check a shell expression
agent-callable --claude '<expression>'        # JSON output for the Claude Code hook
agent-callable --list-tools                   # list all registered tools
agent-callable --help-config                  # config format documentation
```

---

## Known limitations

- **Heredocs piped to compound commands** (`cat <<'EOF' | while...done`) are not parsed by the shell validator — you get the normal prompt. This is a limitation of the Go parser `mvdan.cc/sh`.
- **`cp -t DIR src` style** invocations (destination as a flag value) are not fully covered by `write_target = "last"`.
- **No argument validation in `--sh` mode** — variables can't be resolved statically, so only command names are checked.
