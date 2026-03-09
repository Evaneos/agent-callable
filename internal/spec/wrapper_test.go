package spec

import (
	"fmt"
	"slices"
	"testing"
)

var timeoutFlags = map[string]bool{
	"-k": true, "--kill-after": true,
	"-s": true, "--signal": true,
}

func TestExtractInner(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		flags    map[string]bool
		skip     int
		wantCmd  string
		wantArgs []string
		wantErr  bool
	}{
		{
			name:     "basic",
			args:     []string{"5", "git", "log"},
			flags:    timeoutFlags,
			skip:     1,
			wantCmd:  "git",
			wantArgs: []string{"log"},
		},
		{
			name:     "wrapper flags before duration",
			args:     []string{"-k", "3", "5", "git", "log"},
			flags:    timeoutFlags,
			skip:     1,
			wantCmd:  "git",
			wantArgs: []string{"log"},
		},
		{
			name:     "inner flags preserved",
			args:     []string{"5", "git", "log", "--oneline"},
			flags:    timeoutFlags,
			skip:     1,
			wantCmd:  "git",
			wantArgs: []string{"log", "--oneline"},
		},
		{
			name:     "both wrapper and inner flags",
			args:     []string{"-k", "3", "5", "git", "log", "--oneline", "-n", "5"},
			flags:    timeoutFlags,
			skip:     1,
			wantCmd:  "git",
			wantArgs: []string{"log", "--oneline", "-n", "5"},
		},
		{
			name:     "flag=value form",
			args:     []string{"--kill-after=3", "5", "git", "log"},
			flags:    timeoutFlags,
			skip:     1,
			wantCmd:  "git",
			wantArgs: []string{"log"},
		},
		{
			name:     "skip 0 (nice style)",
			args:     []string{"git", "log", "--oneline"},
			flags:    nil,
			skip:     0,
			wantCmd:  "git",
			wantArgs: []string{"log", "--oneline"},
		},
		{
			name:     "skip 0 with wrapper flags",
			args:     []string{"-n", "10", "git", "log"},
			flags:    map[string]bool{"-n": true, "--adjustment": true},
			skip:     0,
			wantCmd:  "git",
			wantArgs: []string{"log"},
		},
		{
			name:     "double dash separator",
			args:     []string{"--", "git", "push"},
			flags:    timeoutFlags,
			skip:     0,
			wantCmd:  "git",
			wantArgs: []string{"push"},
		},
		{
			name:     "inner cmd with its own flag",
			args:     []string{"5", "git", "-C", "/path", "log"},
			flags:    timeoutFlags,
			skip:     1,
			wantCmd:  "git",
			wantArgs: []string{"-C", "/path", "log"},
		},
		{
			name:     "inner cmd looks like flag",
			args:     []string{"5", "--version"},
			flags:    timeoutFlags,
			skip:     1,
			wantCmd:  "--version",
			wantArgs: nil,
		},
		{
			name:    "empty args",
			args:    []string{},
			flags:   timeoutFlags,
			skip:    1,
			wantErr: true,
		},
		{
			name:    "duration only no inner cmd",
			args:    []string{"5"},
			flags:   timeoutFlags,
			skip:    1,
			wantErr: true,
		},
		{
			name:    "flags only no inner cmd",
			args:    []string{"-k", "3"},
			flags:   timeoutFlags,
			skip:    1,
			wantErr: true,
		},
		{
			name:     "nested wrapper in args",
			args:     []string{"5", "timeout", "3", "git", "log"},
			flags:    timeoutFlags,
			skip:     1,
			wantCmd:  "timeout",
			wantArgs: []string{"3", "git", "log"},
		},
		{
			name:     "multiple wrapper flags",
			args:     []string{"-k", "3", "-s", "KILL", "5", "git", "status"},
			flags:    timeoutFlags,
			skip:     1,
			wantCmd:  "git",
			wantArgs: []string{"status"},
		},
		{
			name:     "boolean wrapper flags",
			args:     []string{"--foreground", "--verbose", "5", "git", "log"},
			flags:    timeoutFlags,
			skip:     1,
			wantCmd:  "git",
			wantArgs: []string{"log"},
		},
		{
			name:     "double dash before skip",
			args:     []string{"--", "5", "git", "log"},
			flags:    timeoutFlags,
			skip:     1,
			wantCmd:  "git",
			wantArgs: []string{"log"},
		},
		{
			name:    "nil args",
			args:    nil,
			flags:   timeoutFlags,
			skip:    1,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, args, err := extractInner(tt.args, tt.flags, tt.skip)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got cmd=%q args=%v", cmd, args)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cmd != tt.wantCmd {
				t.Errorf("cmd = %q, want %q", cmd, tt.wantCmd)
			}
			if !slices.Equal(args, tt.wantArgs) {
				t.Errorf("args = %v, want %v", args, tt.wantArgs)
			}
		})
	}
}

func TestWrapperToolSpec_Name(t *testing.T) {
	w := NewWrapper("timeout", ExtractAfterFlagsAndN(1, timeoutFlags))
	if w.Name() != "timeout" {
		t.Errorf("Name() = %q, want %q", w.Name(), "timeout")
	}
}

func TestWrapperToolSpec_NonInteractiveEnv(t *testing.T) {
	w := NewWrapper("timeout", ExtractAfterFlagsAndN(1, timeoutFlags))
	if env := w.NonInteractiveEnv(); env != nil {
		t.Errorf("expected nil env, got %v", env)
	}
}

func TestWrapperToolSpec_CheckNoCheckFunc(t *testing.T) {
	w := NewWrapper("timeout", ExtractAfterFlagsAndN(1, timeoutFlags))
	res := w.Check([]string{"5", "git", "log"}, RuntimeCtx{})
	if res.Decision == DecisionAllow {
		t.Error("expected deny when checkFunc not set")
	}
}

func TestWrapperToolSpec_Check(t *testing.T) {
	allowGit := func(name string, args []string) ([]string, error) {
		if name == "git" {
			return nil, nil
		}
		return nil, fmt.Errorf("command %q not allowed", name)
	}

	tests := []struct {
		name    string
		wrapper *WrapperToolSpec
		args    []string
		want    Decision
	}{
		{
			name:    "timeout inner allowed",
			wrapper: newTestWrapper("timeout", ExtractAfterFlagsAndN(1, timeoutFlags), allowGit),
			args:    []string{"5", "git", "log"},
			want:    DecisionAllow,
		},
		{
			name:    "timeout inner denied",
			wrapper: newTestWrapper("timeout", ExtractAfterFlagsAndN(1, timeoutFlags), allowGit),
			args:    []string{"5", "terraform", "apply"},
			want:    DecisionDeny,
		},
		{
			name:    "timeout no inner cmd",
			wrapper: newTestWrapper("timeout", ExtractAfterFlagsAndN(1, timeoutFlags), allowGit),
			args:    []string{"5"},
			want:    DecisionDeny,
		},
		{
			name:    "timeout empty args",
			wrapper: newTestWrapper("timeout", ExtractAfterFlagsAndN(1, timeoutFlags), allowGit),
			args:    []string{},
			want:    DecisionDeny,
		},
		{
			name:    "timeout with flags inner allowed",
			wrapper: newTestWrapper("timeout", ExtractAfterFlagsAndN(1, timeoutFlags), allowGit),
			args:    []string{"-k", "3", "5", "git", "log", "--oneline"},
			want:    DecisionAllow,
		},
		{
			name: "nice inner allowed",
			wrapper: newTestWrapper("nice", ExtractAfterFlags(map[string]bool{
				"-n": true, "--adjustment": true,
			}), allowGit),
			args: []string{"-n", "10", "git", "status"},
			want: DecisionAllow,
		},
		{
			name: "nice inner denied",
			wrapper: newTestWrapper("nice", ExtractAfterFlags(map[string]bool{
				"-n": true, "--adjustment": true,
			}), allowGit),
			args: []string{"terraform", "apply"},
			want: DecisionDeny,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := tt.wrapper.Check(tt.args, RuntimeCtx{})
			if res.Decision != tt.want {
				t.Errorf("Check(%v) = %v (reason: %s), want %v", tt.args, res.Decision, res.Reason, tt.want)
			}
		})
	}
}

func newTestWrapper(name string, extractor InnerExtractor, checkFn func(string, []string) ([]string, error)) *WrapperToolSpec {
	w := NewWrapper(name, extractor)
	w.SetCheckFunc(checkFn)
	return w
}
