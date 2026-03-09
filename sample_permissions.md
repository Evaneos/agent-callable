# Sample permissions for `.claude/settings.json`

This document shows an example permission configuration for Claude Code, enabling certain tasks to run without manual confirmation.

## Disclaimer

> **Warning**: This list lets Claude run investigation tasks **without watching or being prompted**.
>
> It offers **no security guarantee**. It filters out common destructive commands, but cannot prevent all possible misuse. It assumes the LLM won't actively try to bypass the filter.

---

## Deny-by-default CLI wrappers

These permissions authorize **CLI wrappers** that filter commands before execution. They are not MCPs, but binaries that wrap tools like `kubectl`, `git`, `gh`, etc.

### `agent-callable`

> The Claude Code `agent-callable` plugin makes this allowlist unnecessary: it automatically validates all Bash commands via a PreToolUse hook, without allowlist or CLAUDE.md instructions.

If the plugin is not installed, the manual approach is to add `Bash(agent-callable:*)` to the allowlist and instruct the agent via CLAUDE.md (see `SAMPLE_CLAUDE.md`).

```json
"Bash(agent-callable:*)"
```

See the [README](README.md) for the full list of supported tools.

### Other examples

```json
"Bash(kubectl-readonly:*)"
```

- `kubectl-readonly`: wrapper similar to `agent-callable`, dedicated to read-only kubectl

---

## Shell utility commands

List of shell commands allowed for automation, organized by category.

### Text and data manipulation

```json
"Bash(awk:*)",
"Bash(base64:*)",
"Bash(cut:*)",
"Bash(diff:*)",
"Bash(grep:*)",
"Bash(head:*)",
"Bash(jq:*)",
"Bash(rg:*)",
"Bash(sed:*)",
"Bash(sort:*)",
"Bash(tail:*)",
"Bash(tr:*)",
"Bash(uniq:*)",
"Bash(wc:*)",
"Bash(yq:*)"
```

### Filesystem

```json
"Bash(basename:*)",
"Bash(cp:*)",
"Bash(dirname:*)",
"Bash(file:*)",
"Bash(find:*)",
"Bash(ls:*)",
"Bash(mkdir:*)",
"Bash(pwd:*)",
"Bash(readlink:*)",
"Bash(realpath:*)",
"Bash(stat:*)",
"Bash(touch:*)",
"Bash(tree:*)"
```

### Compression and archives

```json
"Bash(gunzip:*)",
"Bash(gzip:*)",
"Bash(tar:*)",
"Bash(unzip:*)"
```

### Hashing and verification

```json
"Bash(md5sum:*)",
"Bash(sha256sum:*)"
```

### System information

```json
"Bash(date:*)",
"Bash(df:*)",
"Bash(dmesg:*)",
"Bash(du:*)",
"Bash(env:*)",
"Bash(free:*)",
"Bash(getconf:*)",
"Bash(getfacl:*)",
"Bash(groups:*)",
"Bash(hostname:*)",
"Bash(id:*)",
"Bash(locale:*)",
"Bash(lsblk:*)",
"Bash(lscpu:*)",
"Bash(lsmem:*)",
"Bash(lsmod:*)",
"Bash(lspci:*)",
"Bash(lsusb:*)",
"Bash(printenv:*)",
"Bash(timedatectl:*)",
"Bash(udevadm info:*)",
"Bash(uname:*)",
"Bash(uptime:*)",
"Bash(whoami:*)"
```

### Network (read-only)

```json
"Bash(curl:*)",
"Bash(dig:*)",
"Bash(host:*)",
"Bash(ip addr:*)",
"Bash(ip link:*)",
"Bash(ip route:*)",
"Bash(nslookup:*)",
"Bash(ping:*)",
"Bash(ss:*)",
"Bash(traceroute:*)",
"Bash(whois:*)"
```

### Processes and services

```json
"Bash(journalctl:*)",
"Bash(pgrep:*)",
"Bash(ps:*)",
"Bash(systemctl is-active:*)",
"Bash(systemctl is-enabled:*)",
"Bash(systemctl list-units:*)",
"Bash(systemctl status:*)"
```

### Package managers (read-only)

```json
"Bash(apt list:*)",
"Bash(brew list:*)",
"Bash(flatpak list:*)",
"Bash(rpm -ql:*)"
```

### Development tools

```json
"Bash(cargo build:*)",
"Bash(go:*)",
"Bash(make:*)",
"Bash(node:*)",
"Bash(python3:*)",
"Bash(rustc:*)",
"Bash(uv run:*)",
"Bash(uv tool:*)"
```

### Claude CLI

```json
"Bash(claude mcp --help:*)",
"Bash(claude mcp get:*)",
"Bash(claude mcp list:*)",
"Bash(claude plugin --help:*)",
"Bash(claude plugin list:*)",
"Bash(claude plugin marketplace:*)"
```

### Misc utilities

```json
"Bash(bat:*)",
"Bash(convert:*)",
"Bash(echo:*)",
"Bash(eza:*)",
"Bash(ghostty:*)",
"Bash(gnome-extensions list:*)",
"Bash(gnome-extensions show:*)",
"Bash(gpg --show-keys:*)",
"Bash(gsettings get:*)",
"Bash(micro:*)",
"Bash(opendeck --help:*)",
"Bash(opendeck:*)",
"Bash(tldr:*)",
"Bash(type:*)",
"Bash(whereis:*)",
"Bash(which:*)",
"Bash(xdg-mime query:*)",
"Bash(xlsclients:*)"
```

### Shell constructs (loops)

```json
"Bash(do)",
"Bash(done)"
```

---

## Web search and documentation fetch

Authorize web searches and access to certain documentation domains:

```json
"WebSearch",
"WebFetch(domain:docs.cloudamqp.com)",
"WebFetch(domain:docs.github.com)",
"WebFetch(domain:github.com)",
"WebFetch(domain:keys.openpgp.org)",
"WebFetch(domain:timothycrosley.github.io)",
"WebFetch(domain:www.cloudamqp.com)"
```

---

## MCP tools (Model Context Protocol)

Authorize MCP tools once and for all, whether active or not:

```json
"mcp__context7__get-library-docs",
"mcp__context7__resolve-library-id",
"mcp__notion__notion-create-pages",
"mcp__notion__notion-fetch",
"mcp__notion__notion-search",
"mcp__notion__notion-update-database",
"mcp__notion__notion-update-page"
```

---

## Full configuration

For a complete configuration, assemble all entries in a single `"allow"` array:

```json
{
  "permissions": {
    "allow": [
      "Bash(agent-callable:*)",
      "Bash(kubectl-readonly:*)",
      "Bash(apt list:*)",
      "Bash(awk:*)",
      "Bash(base64:*)",
      "Bash(basename:*)",
      "Bash(bat:*)",
      "Bash(brew list:*)",
      "Bash(cargo build:*)",
      "Bash(claude mcp --help:*)",
      "Bash(claude mcp get:*)",
      "Bash(claude mcp list:*)",
      "Bash(claude plugin --help:*)",
      "Bash(claude plugin list:*)",
      "Bash(claude plugin marketplace:*)",
      "Bash(convert:*)",
      "Bash(cp:*)",
      "Bash(curl:*)",
      "Bash(cut:*)",
      "Bash(date:*)",
      "Bash(df:*)",
      "Bash(diff:*)",
      "Bash(dig:*)",
      "Bash(dirname:*)",
      "Bash(dmesg:*)",
      "Bash(do)",
      "Bash(done)",
      "Bash(du:*)",
      "Bash(echo:*)",
      "Bash(env:*)",
      "Bash(eza:*)",
      "Bash(file:*)",
      "Bash(find:*)",
      "Bash(flatpak list:*)",
      "Bash(free:*)",
      "Bash(getconf:*)",
      "Bash(getfacl:*)",
      "Bash(ghostty:*)",
      "Bash(gnome-extensions list:*)",
      "Bash(gnome-extensions show:*)",
      "Bash(go:*)",
      "Bash(gpg --show-keys:*)",
      "Bash(grep:*)",
      "Bash(groups:*)",
      "Bash(gunzip:*)",
      "Bash(gzip:*)",
      "Bash(gsettings get:*)",
      "Bash(head:*)",
      "Bash(host:*)",
      "Bash(hostname:*)",
      "Bash(id:*)",
      "Bash(ip addr:*)",
      "Bash(ip link:*)",
      "Bash(ip route:*)",
      "Bash(journalctl:*)",
      "Bash(jq:*)",
      "Bash(locale:*)",
      "Bash(ls:*)",
      "Bash(lsblk:*)",
      "Bash(lscpu:*)",
      "Bash(lsmem:*)",
      "Bash(lsmod:*)",
      "Bash(lspci:*)",
      "Bash(lsusb:*)",
      "Bash(make:*)",
      "Bash(md5sum:*)",
      "Bash(micro:*)",
      "Bash(mkdir:*)",
      "Bash(node:*)",
      "Bash(nslookup:*)",
      "Bash(opendeck --help:*)",
      "Bash(opendeck:*)",
      "Bash(pgrep:*)",
      "Bash(ping:*)",
      "Bash(printenv:*)",
      "Bash(ps:*)",
      "Bash(pwd:*)",
      "Bash(python3:*)",
      "Bash(readlink:*)",
      "Bash(realpath:*)",
      "Bash(rg:*)",
      "Bash(rpm -ql:*)",
      "Bash(rustc:*)",
      "Bash(sed:*)",
      "Bash(sha256sum:*)",
      "Bash(sort:*)",
      "Bash(ss:*)",
      "Bash(stat:*)",
      "Bash(systemctl is-active:*)",
      "Bash(systemctl is-enabled:*)",
      "Bash(systemctl list-units:*)",
      "Bash(systemctl status:*)",
      "Bash(tail:*)",
      "Bash(tar:*)",
      "Bash(timedatectl:*)",
      "Bash(tldr:*)",
      "Bash(touch:*)",
      "Bash(tr:*)",
      "Bash(traceroute:*)",
      "Bash(tree:*)",
      "Bash(type:*)",
      "Bash(udevadm info:*)",
      "Bash(uname:*)",
      "Bash(uniq:*)",
      "Bash(unzip:*)",
      "Bash(uptime:*)",
      "Bash(uv run:*)",
      "Bash(uv tool:*)",
      "Bash(wc:*)",
      "Bash(whereis:*)",
      "Bash(which:*)",
      "Bash(whoami:*)",
      "Bash(whois:*)",
      "Bash(xdg-mime query:*)",
      "Bash(xlsclients:*)",
      "Bash(yq:*)",
      "WebFetch(domain:docs.cloudamqp.com)",
      "WebFetch(domain:docs.github.com)",
      "WebFetch(domain:github.com)",
      "WebFetch(domain:keys.openpgp.org)",
      "WebFetch(domain:timothycrosley.github.io)",
      "WebFetch(domain:www.cloudamqp.com)",
      "WebSearch",
      "mcp__context7__get-library-docs",
      "mcp__context7__resolve-library-id",
      "mcp__notion__notion-create-pages",
      "mcp__notion__notion-fetch",
      "mcp__notion__notion-search",
      "mcp__notion__notion-update-database",
      "mcp__notion__notion-update-page"
    ],
    "deny": []
  }
}
```
