package spec

import (
	"testing"
)

func TestAllow(t *testing.T) {
	r := Allow()
	if r.Decision != DecisionAllow {
		t.Error("expected allow")
	}
}

func TestDeny(t *testing.T) {
	r := Deny("reason")
	if r.Decision != DecisionDeny || r.Reason != "reason" {
		t.Errorf("expected deny with reason, got %+v", r)
	}
}

func TestCheckPreamble(t *testing.T) {
	// Empty args -> deny
	res, ok := CheckPreamble("mytool", nil)
	if ok || res.Decision != DecisionDeny {
		t.Errorf("expected deny for nil args, got ok=%v res=%+v", ok, res)
	}
	res, ok = CheckPreamble("mytool", []string{})
	if ok || res.Decision != DecisionDeny {
		t.Errorf("expected deny for empty args, got ok=%v res=%+v", ok, res)
	}
	// Valid args -> ok
	res, ok = CheckPreamble("mytool", []string{"get", "pods"})
	if !ok {
		t.Errorf("expected ok for valid args, got deny: %s", res.Reason)
	}
}

var testFlags = map[string]bool{
	"-n": true, "--namespace": true,
}

func TestFirstNonFlag(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		flags map[string]bool
		want  string
	}{
		{"simple", []string{"list"}, nil, "list"},
		{"skip flags", []string{"--verbose", "list"}, nil, "list"},
		{"skip flag with value", []string{"-n", "ns", "list"}, testFlags, "list"},
		{"double dash stops", []string{"--", "list"}, nil, ""},
		{"empty", []string{}, nil, ""},
		{"only flags", []string{"--verbose", "--all"}, nil, ""},
		{"nil flags map", []string{"-n", "list"}, nil, "list"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FirstNonFlag(tt.args, tt.flags)
			if got != tt.want {
				t.Errorf("FirstNonFlag(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

func TestNthNonFlag(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		n     int
		flags map[string]bool
		want  string
	}{
		{"first", []string{"get", "pods"}, 1, nil, "get"},
		{"second", []string{"get", "pods"}, 2, nil, "pods"},
		{"skip flags", []string{"-n", "ns", "get", "pods"}, 2, testFlags, "pods"},
		{"beyond end", []string{"get"}, 2, nil, ""},
		{"double dash", []string{"get", "--", "pods"}, 2, nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NthNonFlag(tt.args, tt.n, tt.flags)
			if got != tt.want {
				t.Errorf("NthNonFlag(%v, %d) = %q, want %q", tt.args, tt.n, got, tt.want)
			}
		})
	}
}

func TestCountNonFlags(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		flags map[string]bool
		want  int
	}{
		{"simple", []string{"config", "user.name"}, nil, 2},
		{"with flags", []string{"-n", "ns", "get", "pods"}, testFlags, 2},
		{"double dash", []string{"a", "--", "b"}, nil, 1},
		{"empty", []string{}, nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CountNonFlags(tt.args, tt.flags)
			if got != tt.want {
				t.Errorf("CountNonFlags(%v) = %d, want %d", tt.args, got, tt.want)
			}
		})
	}
}

func TestAllNonFlags(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		flags map[string]bool
		want  []string
	}{
		{"simple", []string{"compute", "instances", "list"}, nil, []string{"compute", "instances", "list"}},
		{"with flags", []string{"--project", "p", "compute", "list"}, map[string]bool{"--project": true}, []string{"compute", "list"}},
		{"double dash", []string{"a", "--", "b"}, nil, []string{"a"}},
		{"empty", []string{}, nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AllNonFlags(tt.args, tt.flags)
			if len(got) != len(tt.want) {
				t.Errorf("AllNonFlags(%v) = %v, want %v", tt.args, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("AllNonFlags(%v)[%d] = %q, want %q", tt.args, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestAllPositionalArgs(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		flags map[string]bool
		want  []string
	}{
		{"simple", []string{"/src", "/dst"}, nil, []string{"/src", "/dst"}},
		{"skip short flags", []string{"-r", "/src", "/dst"}, nil, []string{"/src", "/dst"}},
		{"flags with value", []string{"-t", "/dir", "a", "b"}, map[string]bool{"-t": true}, []string{"a", "b"}},
		{"double dash all positional", []string{"--", "-weird", "/dst"}, nil, []string{"-weird", "/dst"}},
		{"mixed before and after dash", []string{"a", "--", "b", "c"}, nil, []string{"a", "b", "c"}},
		{"empty", []string{}, nil, nil},
		{"only flags", []string{"-r", "-v"}, nil, nil},
		{"long flags with value", []string{"--target-directory", "/dir", "a"}, map[string]bool{"--target-directory": true}, []string{"a"}},
		{"double dash with flags before", []string{"-r", "a", "--", "-b", "c"}, nil, []string{"a", "-b", "c"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AllPositionalArgs(tt.args, tt.flags)
			if len(got) != len(tt.want) {
				t.Errorf("AllPositionalArgs(%v) = %v, want %v", tt.args, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("AllPositionalArgs(%v)[%d] = %q, want %q", tt.args, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestContainsFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
		flag string
		want bool
	}{
		{"exact", []string{"--show-secrets"}, "--show-secrets", true},
		{"with value", []string{"--show-secrets=true"}, "--show-secrets", true},
		{"absent", []string{"--verbose"}, "--show-secrets", false},
		{"empty", []string{}, "--show-secrets", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ContainsFlag(tt.args, tt.flag)
			if got != tt.want {
				t.Errorf("ContainsFlag(%v, %q) = %v, want %v", tt.args, tt.flag, got, tt.want)
			}
		})
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		needles []string
		want    bool
	}{
		{"found", []string{"--amend", "file"}, []string{"--amend"}, true},
		{"not found", []string{"--verbose"}, []string{"--amend"}, false},
		{"multiple needles", []string{"-f"}, []string{"--force", "-f"}, true},
		{"empty args", []string{}, []string{"--amend"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ContainsAny(tt.args, tt.needles...)
			if got != tt.want {
				t.Errorf("ContainsAny(%v, %v) = %v, want %v", tt.args, tt.needles, got, tt.want)
			}
		})
	}
}

func TestContainsAnyNonFlag(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		flags   map[string]bool
		needles []string
		want    bool
	}{
		{"found", []string{"compute", "delete"}, nil, []string{"delete", "create"}, true},
		{"not found", []string{"compute", "list"}, nil, []string{"delete"}, false},
		{"skip flags", []string{"--project", "delete", "list"}, map[string]bool{"--project": true}, []string{"delete"}, false},
		{"double dash", []string{"--", "delete"}, nil, []string{"delete"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ContainsAnyNonFlag(tt.args, tt.flags, tt.needles...)
			if got != tt.want {
				t.Errorf("ContainsAnyNonFlag(%v, %v) = %v, want %v", tt.args, tt.needles, got, tt.want)
			}
		})
	}
}

func TestSplitFlag(t *testing.T) {
	tests := []struct {
		arg      string
		wantFlag string
		wantVal  string
	}{
		{"--flag=value", "--flag", "value"},
		{"--flag", "--flag", ""},
		{"-v=1", "-v", "1"},
		{"--network=host", "--network", "host"},
	}
	for _, tt := range tests {
		t.Run(tt.arg, func(t *testing.T) {
			f, v := SplitFlag(tt.arg)
			if f != tt.wantFlag || v != tt.wantVal {
				t.Errorf("SplitFlag(%q) = (%q, %q), want (%q, %q)", tt.arg, f, v, tt.wantFlag, tt.wantVal)
			}
		})
	}
}
