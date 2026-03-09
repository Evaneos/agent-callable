package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/evaneos/agent-callable/internal/shell"
)

// TestDefaultConfigsParseValid ensures every embedded TOML in DefaultConfigs
// is valid and parseable, catching syntax errors at test time.
func TestDefaultConfigsParseValid(t *testing.T) {
	for name, content := range shell.DefaultConfigs {
		configs, err := ParseTOML(content)
		if err != nil {
			t.Errorf("%s: parse error: %v", name, err)
			continue
		}
		if len(configs) == 0 {
			t.Errorf("%s: parsed zero tools", name)
		}
		for _, c := range configs {
			if c.Name == "" {
				t.Errorf("%s: tool with empty name", name)
			}
		}
	}
}

// TestGenerateDefaultsCreatesFiles verifies that GenerateDefaults writes all
// expected files to disk and that each is valid TOML.
func TestGenerateDefaultsCreatesFiles(t *testing.T) {
	dir := t.TempDir()
	created, skipped, err := shell.GenerateDefaults(dir)
	if err != nil {
		t.Fatalf("GenerateDefaults: %v", err)
	}
	if len(skipped) != 0 {
		t.Errorf("unexpected skipped on fresh dir: %v", skipped)
	}

	// config.toml + all tool files
	expectedCount := len(shell.DefaultConfigs) + 1
	if len(created) != expectedCount {
		t.Errorf("created %d files, want %d: %v", len(created), expectedCount, created)
	}

	// Verify each tool TOML on disk is parseable.
	toolsDir := filepath.Join(dir, "tools.d")
	for name := range shell.DefaultConfigs {
		data, err := os.ReadFile(filepath.Join(toolsDir, name))
		if err != nil {
			t.Errorf("%s: not written to disk: %v", name, err)
			continue
		}
		if _, err := ParseTOML(string(data)); err != nil {
			t.Errorf("%s: written file doesn't parse: %v", name, err)
		}
	}

	// Verify config.toml exists.
	if _, err := os.Stat(filepath.Join(dir, "config.toml")); err != nil {
		t.Errorf("config.toml not written: %v", err)
	}
}

// TestGenerateDefaultsIdempotent verifies that existing files are not overwritten.
func TestGenerateDefaultsIdempotent(t *testing.T) {
	dir := t.TempDir()

	// First run.
	_, _, err := shell.GenerateDefaults(dir)
	if err != nil {
		t.Fatalf("first GenerateDefaults: %v", err)
	}

	// Tamper with one file.
	sentinel := filepath.Join(dir, "tools.d", "dev-tools.toml")
	if err := os.WriteFile(sentinel, []byte("# custom"), 0644); err != nil {
		t.Fatal(err)
	}

	// Second run: should skip all.
	created, skipped, err := shell.GenerateDefaults(dir)
	if err != nil {
		t.Fatalf("second GenerateDefaults: %v", err)
	}
	if len(created) != 0 {
		t.Errorf("second run created files: %v", created)
	}
	expectedSkipped := len(shell.DefaultConfigs) + 1
	if len(skipped) != expectedSkipped {
		t.Errorf("skipped %d, want %d: %v", len(skipped), expectedSkipped, skipped)
	}

	// Verify tampered file was NOT overwritten.
	data, _ := os.ReadFile(sentinel)
	if string(data) != "# custom" {
		t.Error("existing file was overwritten")
	}
}
