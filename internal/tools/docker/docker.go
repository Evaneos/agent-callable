package docker

import (
	"fmt"
	"strings"

	"github.com/evaneos/agent-callable/internal/spec"
)

type Tool struct {
	writableDirs []string
}

func New(writableDirs []string) *Tool { return &Tool{writableDirs: writableDirs} }

func (t *Tool) Name() string { return "docker" }

func (t *Tool) NonInteractiveEnv() map[string]string { return nil }

// docker global flags that consume the next separate argument.
var dockerGlobalFlagsWithValue = map[string]bool{
	"-H": true, "--host": true,
	"-c": true, "--context": true,
	"-l": true, "--log-level": true,
	"--config": true,
}

// dockerRunFlagsWithValue lists common docker run flags that consume the next argument.
var dockerRunFlagsWithValue = map[string]bool{
	"-e": true, "--env": true,
	"-p": true, "--publish": true,
	"-w": true, "--workdir": true,
	"--name":       true,
	"--entrypoint": true,
	"--user":       true, "-u": true,
	"--hostname": true, "-h": true,
	"--label": true, "-l": true,
	"--env-file":   true,
	"--log-driver": true,
	"--log-opt":    true,
	"--memory":     true, "-m": true,
	"--cpus":        true,
	"--restart":     true,
	"--stop-signal": true,
	"--cidfile":     true,
	"--add-host":    true,
	"--dns":         true,
	"--expose":      true,
	"--link":        true,
	"--tmpfs":       true,
	"--shm-size":    true,
	"--platform":    true,
	"--pull":        true,
	"--runtime":     true,
	"--init":        true,
	"--ip":          true,
	"--mac-address": true,
	"--domainname":  true,
	"--annotation":  true,
}

func (t *Tool) Check(args []string, _ spec.RuntimeCtx) spec.Result {
	if res, ok := spec.CheckPreamble("docker", args); !ok {
		return res
	}

	cmd := spec.FirstNonFlag(args, dockerGlobalFlagsWithValue)
	if cmd == "" {
		return spec.Deny("docker subcommand not found")
	}

	// Block commands with significant side effects.
	switch cmd {
	case "exec", "rm", "rmi", "kill", "stop", "start", "restart",
		"build", "push", "login", "logout", "cp", "commit", "system", "volume", "network":
		return spec.Deny(fmt.Sprintf("docker command %q not allowed (side effect)", cmd))
	}

	switch cmd {
	case "version", "info":
		return spec.Allow()
	case "ps", "images", "inspect", "logs", "stats", "pull":
		return spec.Allow()
	case "history", "top", "port", "diff", "events", "search":
		return spec.Allow()
	case "run":
		return t.checkRun(args)
	case "context":
		sub := spec.NthNonFlag(args, 2, dockerGlobalFlagsWithValue)
		if sub == "ls" || sub == "inspect" || sub == "show" {
			return spec.Allow()
		}
		return spec.Deny(fmt.Sprintf("docker context %q not allowed", sub))
	case "container":
		sub := spec.NthNonFlag(args, 2, dockerGlobalFlagsWithValue)
		switch sub {
		case "ls", "list", "inspect", "logs", "stats", "top", "diff", "port":
			return spec.Allow()
		case "run":
			return t.checkRun(args)
		default:
			return spec.Deny(fmt.Sprintf("docker container %q not allowed", sub))
		}
	case "image":
		sub := spec.NthNonFlag(args, 2, dockerGlobalFlagsWithValue)
		switch sub {
		case "ls", "list", "inspect", "history":
			return spec.Allow()
		default:
			return spec.Deny(fmt.Sprintf("docker image %q not allowed", sub))
		}
	case "compose":
		sub := spec.NthNonFlag(args, 2, dockerGlobalFlagsWithValue)
		switch sub {
		case "ps", "ls", "logs", "config", "images", "top", "version":
			return spec.Allow()
		default:
			return spec.Deny(fmt.Sprintf("docker compose %q not allowed", sub))
		}
	case "manifest":
		sub := spec.NthNonFlag(args, 2, dockerGlobalFlagsWithValue)
		if sub == "inspect" {
			return spec.Allow()
		}
		return spec.Deny(fmt.Sprintf("docker manifest %q not allowed", sub))
	default:
		return spec.Deny(fmt.Sprintf("docker command %q not allowed", cmd))
	}
}

// checkRun validates a `docker run` command against the allowlist.
func (t *Tool) checkRun(args []string) spec.Result {
	blockedBool := map[string]bool{
		"--privileged": true,
	}

	blockedWithValue := map[string]bool{
		"--cap-add":      true,
		"--device":       true,
		"--security-opt": true,
		"--volumes-from": true,
	}

	namespaceFlagsWithValue := map[string]bool{
		"--pid":      true,
		"--network":  true,
		"--net":      true,
		"--ipc":      true,
		"--uts":      true,
		"--userns":   true,
		"--cgroupns": true,
	}

	for i := 0; i < len(args); i++ {
		a := args[i]

		if !strings.HasPrefix(a, "-") {
			continue
		}

		flag, val := spec.SplitFlag(a)

		if blockedBool[flag] {
			return spec.Deny(fmt.Sprintf("docker run: flag %q not allowed (privilege)", flag))
		}

		if blockedWithValue[flag] {
			return spec.Deny(fmt.Sprintf("docker run: flag %q not allowed (privilege)", flag))
		}

		if namespaceFlagsWithValue[flag] {
			if val == "" && i+1 < len(args) {
				val = args[i+1]
				i++
			}
			if val == "host" {
				return spec.Deny(fmt.Sprintf("docker run: %s=host not allowed (host namespace)", flag))
			}
			continue
		}

		if flag == "-v" || flag == "--volume" {
			if val == "" && i+1 < len(args) {
				val = args[i+1]
				i++
			}
			if err := t.checkVolume(val); err != nil {
				return spec.Deny(fmt.Sprintf("docker run: volume %q blocked: %v", val, err))
			}
			continue
		}

		if flag == "--mount" {
			if val == "" && i+1 < len(args) {
				val = args[i+1]
				i++
			}
			if err := t.checkMount(val); err != nil {
				return spec.Deny(fmt.Sprintf("docker run: mount %q blocked: %v", val, err))
			}
			continue
		}

		if dockerGlobalFlagsWithValue[flag] && val == "" && i+1 < len(args) {
			i++
			continue
		}

		if dockerRunFlagsWithValue[flag] && val == "" && i+1 < len(args) {
			i++
		}
	}

	return spec.Allow()
}

func (t *Tool) checkVolume(vol string) error {
	if vol == "" {
		return fmt.Errorf("empty volume specification")
	}

	parts := strings.SplitN(vol, ":", 3)

	if len(parts) == 1 {
		src := parts[0]
		if isBindPath(src) {
			return fmt.Errorf("host path %q without destination", src)
		}
		return nil
	}

	src := parts[0]

	if src == "" {
		return fmt.Errorf("empty host path in volume specification")
	}

	if !isBindPath(src) {
		return nil
	}

	if len(parts) == 3 {
		opts := parts[2]
		if isReadOnlyOpt(opts) {
			return nil
		}
	}

	if !spec.IsUnderWritableDir(src, t.writableDirs) {
		return fmt.Errorf("rw bind mount %q outside of writable_dirs", src)
	}
	return nil
}

func (t *Tool) checkMount(mount string) error {
	if mount == "" {
		return fmt.Errorf("empty mount specification")
	}

	fields := parseMountFields(mount)
	mountType := fields["type"]

	if mountType == "tmpfs" || mountType == "volume" {
		return nil
	}

	if mountType == "" {
		return nil
	}

	if mountType == "bind" {
		src := fields["source"]
		if src == "" {
			src = fields["src"]
		}

		if _, ok := fields["readonly"]; ok {
			return nil
		}
		if fields["ro"] == "true" || fields["ro"] == "1" {
			return nil
		}

		if !spec.IsUnderWritableDir(src, t.writableDirs) {
			return fmt.Errorf("rw bind mount %q outside of writable_dirs", src)
		}
		return nil
	}

	return fmt.Errorf("unrecognized mount type %q", mountType)
}

func isBindPath(s string) bool {
	return strings.HasPrefix(s, "/") || strings.HasPrefix(s, ".") || strings.HasPrefix(s, "~")
}

func isReadOnlyOpt(opts string) bool {
	for _, o := range strings.Split(opts, ",") {
		if o == "ro" || o == "readonly" {
			return true
		}
	}
	return false
}

func parseMountFields(mount string) map[string]string {
	fields := make(map[string]string)
	for _, part := range strings.Split(mount, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			fields[kv[0]] = kv[1]
		} else {
			fields[kv[0]] = ""
		}
	}
	return fields
}
