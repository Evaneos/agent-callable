package spec

import "testing"

func TestIsUnderWritableDir(t *testing.T) {
	dirs := []string{"/tmp", "/var/log"}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{"under tmp", "/tmp/foo", true},
		{"exact tmp", "/tmp", true},
		{"under var/log", "/var/log/app.log", true},
		{"dev null", "/dev/null", true},
		{"outside", "/etc/passwd", false},
		{"prefix trick", "/tmpfoo", false},
		{"empty path", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsUnderWritableDir(tt.path, dirs)
			if got != tt.want {
				t.Errorf("IsUnderWritableDir(%q, %v) = %v, want %v", tt.path, dirs, got, tt.want)
			}
		})
	}
}

func TestIsUnderWritableDirEmptyDirs(t *testing.T) {
	if IsUnderWritableDir("/tmp/foo", nil) {
		t.Error("expected false with nil dirs")
	}
	if !IsUnderWritableDir("/dev/null", nil) {
		t.Error("/dev/null should always be writable")
	}
}
