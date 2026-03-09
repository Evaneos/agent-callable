package docker

import (
	"strings"
	"testing"

	"github.com/evaneos/agent-callable/internal/spec"
	"github.com/evaneos/agent-callable/internal/spectest"
)

var testWritableDirs = []string{"/tmp"}

func TestDockerAllowlist(t *testing.T) {
	tool := New(testWritableDirs)
	allowed := [][]string{
		{"version"},
		{"ps"},
		{"images"},
		{"inspect", "container_id"},
		{"logs", "container_id"},
		{"pull", "alpine:latest"},
		{"history", "alpine:latest"},
		{"top", "container_id"},
		{"port", "container_id"},
		{"diff", "container_id"},
		{"events", "--since", "1h"},
		{"search", "nginx"},
		// Management commands
		{"container", "ls"},
		{"container", "inspect", "x"},
		{"container", "logs", "x"},
		{"container", "stats", "x"},
		{"container", "top", "x"},
		{"container", "diff", "x"},
		{"container", "port", "x"},
		{"image", "ls"},
		{"image", "inspect", "alpine"},
		{"image", "history", "alpine"},
		// Context
		{"context", "ls"},
		{"context", "inspect", "default"},
		// Manifest read-only
		{"manifest", "inspect", "alpine:latest"},
		// Global flags with value argument
		{"-c", "mycontext", "ps"},
		{"--context", "mycontext", "images"},
		{"-H", "unix:///var/run/docker.sock", "ps"},
		{"--config", "/custom/path", "version"},
	}
	spectest.AssertAllowedBatch(t, tool, allowed)
}

func TestDockerBlocksWrites(t *testing.T) {
	tool := New(testWritableDirs)
	blocked := [][]string{
		{"exec", "-it", "x", "sh"},
		{"rm", "-f", "x"},
		{"build", "."},
		{"push", "x"},
		// Management commands write
		{"container", "rm", "x"},
		{"container", "start", "x"},
		{"image", "rm", "alpine"},
		{"image", "push", "x"},
		{"context", "create", "x"},
		// Global flags don't bypass sub-command checks
		{"-c", "mycontext", "rm", "-f", "x"},
	}
	spectest.AssertBlockedBatch(t, tool, blocked)
}

func TestDockerComposeV2(t *testing.T) {
	tool := New(testWritableDirs)

	// === ALLOWED: docker compose read-only ===
	allowed := [][]string{
		{"compose", "ps"},
		{"compose", "logs"},
		{"compose", "logs", "-f", "web"},
		{"compose", "config"},
		{"compose", "images"},
		{"compose", "top", "web"},
		{"compose", "version"},
		{"compose", "ls"},
		// Global flags before compose
		{"-c", "mycontext", "compose", "ps"},
		{"--context", "prod", "compose", "logs"},
	}
	spectest.AssertAllowedBatch(t, tool, allowed)

	// === BLOCKED: docker compose write ===
	blocked := [][]string{
		{"compose", "up"},
		{"compose", "up", "-d"},
		{"compose", "down"},
		{"compose", "down", "--volumes"},
		{"compose", "exec", "web", "sh"},
		{"compose", "build"},
		{"compose", "run", "web", "npm", "test"},
		{"compose", "rm"},
		{"compose", "start"},
		{"compose", "stop"},
		{"compose", "restart"},
		{"compose", "pull"},
		{"compose", "push"},
		{"compose", "create"},
		{"compose", "kill"},
		{"compose", "pause"},
		{"compose", "unpause"},
		// Global flags don't bypass
		{"-c", "mycontext", "compose", "up", "-d"},
		{"--host", "ssh://remote", "compose", "down"},
	}
	spectest.AssertBlockedBatch(t, tool, blocked)
}

func TestDockerEdgeCases(t *testing.T) {
	tool := New(testWritableDirs)

	// === ALLOWED edge cases ===
	allowed := [][]string{
		{"info"},
		{"stats", "--no-stream"},
		{"stats", "--format", "table {{.Name}}\t{{.CPUPerc}}"},
		{"container", "list"},
		{"ps", "-a", "--format", "table {{.Names}}\t{{.Status}}"},
		{"images", "--format", "{{.Repository}}:{{.Tag}}"},
		{"inspect", "--format", "{{.State.Status}}", "container_id"},
		{"logs", "--tail", "100", "-f", "container_id"},
		{"pull", "nginx:1.25"},
		{"context", "show"},
		{"image", "list"},
	}
	spectest.AssertAllowedBatch(t, tool, allowed)

	// === BLOCKED edge cases ===
	blocked := [][]string{
		// Explicitly blocked commands
		{"login", "--username", "user"},
		{"logout"},
		{"cp", "container:/path", "/local"},
		{"commit", "container", "image"},
		{"kill", "container"},
		{"stop", "container"},
		{"start", "container"},
		{"restart", "container"},
		{"system", "prune"},
		{"volume", "create", "myvol"},
		{"network", "create", "mynet"},
		{"rmi", "alpine"},
		// Unknown management commands
		{"builder", "prune"},
		{"manifest", "create", "x"},
		{"manifest", "push", "x"},
		{"manifest", "annotate", "x", "y"},
		{"trust", "sign", "x"},
		{"plugin", "install", "x"},
		{"swarm", "init"},
		{"service", "create", "x"},
		{"node", "update", "x"},
		{"stack", "deploy", "x"},
		{"secret", "create", "x"},
		{"config", "create", "x"},
	}
	spectest.AssertBlockedBatch(t, tool, blocked)
}

func TestDockerEmptyAndBareArgs(t *testing.T) {
	tool := New(testWritableDirs)
	spectest.AssertBlocked(t, tool, []string{})
	spectest.AssertBlocked(t, tool, []string{"-H", "unix:///var/run/docker.sock"})
}

func TestDockerRunAllowed(t *testing.T) {
	tool := New(testWritableDirs)

	tests := []struct {
		name string
		args []string
	}{
		{"simple run", split("run --rm alpine echo hello")},
		{"bind mount in writable_dirs", split("run --rm -v /tmp/data:/data alpine cat /data/file")},
		{"bind mount subfolder writable_dirs", split("run --rm -v /tmp/sub/deep:/x alpine ls")},
		{"bind mount :ro outside writable", split("run --rm -v /home/user/proj:/app:ro alpine ls /app")},
		{"sensitive file :ro", split("run --rm -v /etc/hosts:/etc/hosts:ro alpine cat /etc/hosts")},
		{"named volume via -v", split("run --rm -v myvolume:/data alpine ls")},
		{"mount type=bind in writable", split("run --rm --mount type=bind,source=/tmp/x,target=/y alpine ls")},
		{"mount type=bind readonly", split("run --rm --mount type=bind,source=/anywhere,target=/y,readonly alpine ls")},
		{"mount type=volume named", split("run --rm --mount type=volume,source=mydata,target=/data alpine ls")},
		{"mount type=tmpfs", split("run --rm --mount type=tmpfs,target=/tmp alpine ls")},
		{"network bridge", split("run --rm --network bridge alpine ping 1.1.1.1")},
		{"pid container", split("run --rm --pid container:xyz alpine ps")},
		{"env vars", split("run --rm -e FOO=bar alpine env")},
		{"port mapping", split("run --rm -p 8080:80 alpine sh")},
		{"workdir", split("run --rm -w /app alpine pwd")},
		{"name", split("run --rm --name test alpine echo")},
		{"entrypoint", split("run --rm --entrypoint /bin/echo alpine hello")},
		{"container run", split("container run --rm alpine echo hello")},
		{"global flags before run", split("-c mycontext run --rm alpine echo")},
		{"volume long form", split("run --rm --volume=/tmp/x:/y alpine sh")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spectest.AssertAllowed(t, tool, tt.args)
		})
	}
}

func TestDockerRunBlocked(t *testing.T) {
	tool := New(testWritableDirs)

	tests := []struct {
		name string
		args []string
	}{
		{"privileged", split("run --privileged alpine sh")},
		{"cap-add space", split("run --cap-add SYS_ADMIN alpine sh")},
		{"cap-add equals", split("run --cap-add=NET_ADMIN alpine sh")},
		{"device space", split("run --device /dev/sda alpine sh")},
		{"device equals", split("run --device=/dev/kvm alpine sh")},
		{"security-opt space", split("run --security-opt seccomp=unconfined alpine sh")},
		{"security-opt equals", split("run --security-opt=apparmor=unconfined alpine sh")},
		{"volumes-from", split("run --volumes-from other alpine sh")},
		{"pid host equals", split("run --pid=host alpine ps")},
		{"pid host space", split("run --pid host alpine ps")},
		{"network host equals", split("run --network=host alpine curl")},
		{"network host space", split("run --network host alpine curl")},
		{"net host equals", split("run --net=host alpine curl")},
		{"net host space", split("run --net host alpine curl")},
		{"ipc host", split("run --ipc=host alpine sh")},
		{"uts host", split("run --uts=host alpine sh")},
		{"userns host", split("run --userns=host alpine sh")},
		{"cgroupns host", split("run --cgroupns=host alpine sh")},
		{"bind mount rw outside writable", split("run -v /home/user/secrets:/secrets alpine sh")},
		{"bind mount rw sensitive file", split("run -v /etc/passwd:/etc/passwd alpine cat")},
		{"bind mount rw explicit outside", split("run -v /etc:/mnt:rw alpine sh")},
		{"mount bind rw outside writable", split("run --mount type=bind,source=/etc,target=/etc alpine sh")},
		{"container run privileged", split("container run --privileged alpine sh")},
		{"global flags no bypass", split("-c ctx run --privileged alpine sh")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spectest.AssertBlocked(t, tool, tt.args)
		})
	}
}

func TestDockerRunVolumeEdgeCases(t *testing.T) {
	tool := New(testWritableDirs)

	tests := []struct {
		name    string
		args    []string
		allowed bool
	}{
		{"two volumes both ok", split("run -v /tmp/a:/a -v /home:/b:ro alpine sh"), true},
		{"two volumes second rw outside", split("run -v /tmp/a:/a -v /home:/b alpine sh"), false},
		{"empty volume spec", split("run -v :/bad alpine sh"), false},
		{"volume long equals form", split("run --volume=/tmp/x:/y alpine sh"), true},
		{"relative path dot", split("run -v .:/app alpine sh"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := tool.Check(tt.args, spec.RuntimeCtx{})
			if tt.allowed && res.Decision != spec.DecisionAllow {
				t.Errorf("expected allowed, got deny: %s", res.Reason)
			}
			if !tt.allowed && res.Decision == spec.DecisionAllow {
				t.Errorf("expected blocked for %v", tt.args)
			}
		})
	}
}

func TestDockerRunVolumeMoreEdgeCases(t *testing.T) {
	tool := New(testWritableDirs)

	tests := []struct {
		name    string
		args    []string
		allowed bool
	}{
		// Single bind path without destination -> blocked
		{"single bind path no dest", split("run -v /host/path alpine sh"), false},
		// Single named volume (no colon) -> allowed
		{"single named volume no colon", split("run -v myvolume alpine sh"), true},
		// -v as last arg (empty volume spec) -> blocked
		{"empty volume spec flag at end", []string{"run", "-v"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := tool.Check(tt.args, spec.RuntimeCtx{})
			if tt.allowed && res.Decision != spec.DecisionAllow {
				t.Errorf("expected allowed, got deny: %s", res.Reason)
			}
			if !tt.allowed && res.Decision == spec.DecisionAllow {
				t.Errorf("expected blocked for %v", tt.args)
			}
		})
	}
}

func TestDockerRunMountEdgeCases(t *testing.T) {
	tool := New(testWritableDirs)

	tests := []struct {
		name    string
		args    []string
		allowed bool
	}{
		// src field instead of source
		{"mount bind src field rw outside", split("run --rm --mount type=bind,src=/etc,target=/y alpine sh"), false},
		{"mount bind src field rw writable", split("run --rm --mount type=bind,src=/tmp/x,target=/y alpine sh"), true},
		// ro=1 makes it readonly -> allowed
		{"mount bind ro=1", split("run --rm --mount type=bind,source=/etc,target=/y,ro=1 alpine sh"), true},
		// ro=true -> allowed
		{"mount bind ro=true", split("run --rm --mount type=bind,source=/etc,target=/y,ro=true alpine sh"), true},
		// unrecognized mount type -> blocked
		{"mount unrecognized type", split("run --rm --mount type=nfs,source=x,target=/y alpine sh"), false},
		// empty mount type (no type field) -> allowed (defaults to nil behavior)
		{"mount no type field", split("run --rm --mount target=/y alpine sh"), true},
		// --mount as last arg (empty mount spec) -> blocked
		{"empty mount spec flag at end", []string{"run", "--mount"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := tool.Check(tt.args, spec.RuntimeCtx{})
			if tt.allowed && res.Decision != spec.DecisionAllow {
				t.Errorf("expected allowed, got deny: %s", res.Reason)
			}
			if !tt.allowed && res.Decision == spec.DecisionAllow {
				t.Errorf("expected blocked for %v", tt.args)
			}
		})
	}
}

// split is a test helper that splits a string into args.
func split(s string) []string {
	return strings.Split(s, " ")
}
