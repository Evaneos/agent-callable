# Dev directives for `agent-callable`

`CLAUDE.md` is a symlink to `AGENTS.md`.

This file contains directives for **developing and maintaining** `agent-callable`.
For a runtime usage example in another project, see `SAMPLE_CLAUDE.md`.

## Tool purpose (context for design decisions)

- `agent-callable` is a **deny-by-default** command filter designed to be **pre-approvable** in an LLM agent's allowlist.
- Goal: reduce the surface of **accidental side effects** from an agent (push/PR/apply/destroy/etc.).
- This is a best-effort filter, not a security boundary. It offers no guarantee against intentional or creative bypass.

## Policy principles (to follow in code)

- **Deny-by-default**: any new subcommand/flag starts **blocked** until explicitly classified as "OK".
- **Prefer false positives** (blocking too much) over false negatives (letting dangerous things through).
- **No shell**: never evaluate shell; execute the binary directly (reduces injection risk).
- **Non-interactive**:
  - neutralize `stdin` if TTY when relevant,
  - disable prompts (e.g. `GIT_TERMINAL_PROMPT=0`, `GH_PROMPT_DISABLED=1`),
  - avoid/neutralize pagers (e.g. `PAGER=cat` when useful).
- **LLM-friendly messages**: on block, return a **short** and **actionable** error (without requesting input).
- **Secrets**: block modes/flags that exfiltrate secrets (e.g. `kubectl get secrets -o yaml/json`, `pulumi --show-secrets`).
- **Persistent changes**: block commands that durably modify config (e.g. `helm repo add`, `gcloud config set`) unless explicitly decided.
- **Remote effects**: block remote/destructive operations (push, PR create/merge, apply/destroy, etc.).
- **Local "cache/artifact" writes**: usually OK if useful for investigation (e.g. `git fetch`, `docker pull`, `gh repo clone`), as long as it doesn't durably reconfigure the tool.

## Code organization

- Each tool is a `ToolSpec` in `internal/tools/<tool>/`.
- TOML-configured tools that write files must declare `write_target` (`"last"` or `"all"`) so destinations are checked against `writable_dirs`. See `internal/spec/configtool.go:checkWriteTarget`.
- Any allowlist change must be:
  - **tested** (table-driven unit tests),
  - **documented** in `README.md` (supported commands + philosophy/heuristics if needed).

## Git & PR conventions

- Rebase merge (fast-forward), no merge commits, no squash. Individual commits land on main as-is.
- Clean history: no fixup/revert commits in branches — amend or rebase before PR.
- [Conventional Commits](https://www.conventionalcommits.org/) with scope = tool or module (`defaults`, `engine`, `gcloud`, `spec`, …).
- Commit messages must be changelog-ready (*what* and *why*).
- Rebase on `origin/main` before submitting the PR.

## Versioning & release

Single semver `v<major>.<minor>.<patch>` for both binary and plugin.
Source of truth: `plugins/agent-callable/.claude-plugin/plugin.json` → `version` field.
To release: edit `version` in plugin.json, then `make tag` (commits plugin.json + README badge + marketplace.json, creates annotated tag).
GoReleaser runs in CI (GitHub Actions) on tag push — do not run locally.
`make build-all` and `make package` are for local dev only.

## Dev workflow

- **Go**: Go 1.25+ (see `go.mod`).
- **Local build**: `make build` or `make install`
- **Tests**: `make test`
- **Format / modules**: `make fmt`, `make tidy`
- **Distribution**: `make build-all`, `make package`, `make clean-all`
- **Claude Code plugin**: `make plugin-sync` (sync `plugins/agent-callable/` to `~/.claude/plugins/cache/`), also run by `make install`
- **Info**: `make info` (version, plugin install status)
- **After any code change**: run `make install` to rebuild the binary AND refresh the plugin cache, so the user can test immediately.
