## agent-callable

> The Claude Code `agent-callable` plugin makes this section unnecessary: it automatically validates all Bash commands via a PreToolUse hook. The following only applies if the plugin is not installed.

Prefix with `agent-callable` calls to supported commands with no side effects (pre-approved via `Bash(agent-callable:*)`).

Supported commands: apt, awk, base64, basename, bat, cat, convert, cp, curl, cut, date, df, diff, dig, dirname, dmesg, docker, docker-compose, du, echo, env, envsubst, eslint, eza, file, find, flatpak, flux, free, gcloud, getconf, gh, git, gnome-extensions, gofmt, grep, groups, gsettings, gunzip, gzip, head, helm, host, hostname, id, ip, journalctl, jq, krew, kubectl, kubectl-crossplane, kubectl-readonly, kubectx, kustomize, locale, ls, lsblk, lscpu, lsmem, lsmod, lspci, lsusb, md5sum, mkdir, npm, nslookup, paste, pgrep, ping, prettier, printenv, ps, pulumi, readlink, realpath, rg, ruff, sed, seq, sha256sum, sleep, sort, ss, stat, systemctl, tail, tar, tee, timedatectl, tldr, touch, tr, traceroute, tree, tsc, ty, type, udevadm, uname, uniq, unzip, uptime, uv, wc, whereis, which, whoami, whois, xargs, xdg-mime, yamllint, yq.

Additional tools can be added via TOML config. `agent-callable --list-tools` to list all available tools.

### Rules

- If `agent-callable` blocks a command, retry without `agent-callable` (normal user prompt).
- `docker run` via `agent-callable`: no `--privileged`/`--cap-add`/`--network=host`. RW bind mounts only under `/tmp`, `:ro` OK anywhere.
- `npm install`/`npm ci`: add `--ignore-scripts` to avoid prompts.
- `npm run`: only safe scripts allowed (test, lint, build, format, typecheck, check, dev, start).
- `eslint --fix`, `prettier --write`, `sed -i`, `gofmt -w`, `ruff --fix`: write targets must be in writable dirs.
- Secrets: do not display/expose (e.g. `kubectl get secrets -o yaml/json`).
- Non-interactive: no prompts/pagers.

### Shell mode (`--sh`)

For compound expressions (loops, pipes, `&&`):

```bash
agent-callable --sh 'for ns in prod staging; do kubectl get pods -n $ns; echo "---"; done'
agent-callable --sh 'helm list -A | awk "{print \$1}" | sort'
```

Dangerous builtins (`eval`, `exec`, `source`) and dynamic commands (`$CMD args`) are blocked. Write redirections only to `/dev/null` and `writable_dirs` (default: `/tmp`).
