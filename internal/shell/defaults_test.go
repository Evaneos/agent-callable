package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateConfigsSelectSubset(t *testing.T) {
	dir := t.TempDir()

	selected := map[string]bool{
		"text-processing.toml": true,
		"network.toml":         true,
	}
	created, skipped, err := GenerateConfigs(dir, selected, nil, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skipped) != 0 {
		t.Errorf("expected no skipped files, got %v", skipped)
	}

	// Expect exactly the 2 selected tools.d files + config.toml.
	wantCreated := map[string]bool{
		"text-processing.toml": true,
		"network.toml":         true,
		"config.toml":          true,
	}
	if len(created) != len(wantCreated) {
		t.Errorf("expected %d created files, got %d: %v", len(wantCreated), len(created), created)
	}
	for _, name := range created {
		if !wantCreated[name] {
			t.Errorf("unexpected created file: %s", name)
		}
	}

	// Verify files exist on disk.
	toolsDir := filepath.Join(dir, "tools.d")
	for name := range selected {
		path := filepath.Join(toolsDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file %s to exist: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "config.toml")); err != nil {
		t.Errorf("expected config.toml to exist: %v", err)
	}

	// Verify unselected files were NOT created.
	entries, err := os.ReadDir(toolsDir)
	if err != nil {
		t.Fatalf("cannot read tools.d: %v", err)
	}
	if len(entries) != len(selected) {
		t.Errorf("expected %d files in tools.d, got %d", len(selected), len(entries))
	}
}

func TestGenerateConfigsEmptySelection(t *testing.T) {
	dir := t.TempDir()

	created, skipped, err := GenerateConfigs(dir, map[string]bool{}, nil, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skipped) != 0 {
		t.Errorf("expected no skipped files, got %v", skipped)
	}

	// Only config.toml should be created; no tools.d/*.toml files.
	if len(created) != 1 || created[0] != "config.toml" {
		t.Errorf("expected only config.toml to be created, got %v", created)
	}
	if _, err := os.Stat(filepath.Join(dir, "config.toml")); err != nil {
		t.Errorf("expected config.toml to exist: %v", err)
	}

	toolsDir := filepath.Join(dir, "tools.d")
	entries, err := os.ReadDir(toolsDir)
	if err != nil {
		t.Fatalf("cannot read tools.d: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected no files in tools.d, got %v", entries)
	}
}

func TestGenerateConfigsNilSelection(t *testing.T) {
	dir := t.TempDir()

	created, skipped, err := GenerateConfigs(dir, nil, nil, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skipped) != 0 {
		t.Errorf("expected no skipped files, got %v", skipped)
	}

	// All DefaultConfigs files + config.toml should be created.
	wantCount := len(DefaultConfigs) + 1 // +1 for config.toml
	if len(created) != wantCount {
		t.Errorf("expected %d created files, got %d: %v", wantCount, len(created), created)
	}

	// Verify every DefaultConfigs file exists on disk.
	toolsDir := filepath.Join(dir, "tools.d")
	for name := range DefaultConfigs {
		path := filepath.Join(toolsDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file %s to exist: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "config.toml")); err != nil {
		t.Errorf("expected config.toml to exist: %v", err)
	}
}

func TestCategoriesCoverAllDefaults(t *testing.T) {
	covered := make(map[string]bool)
	for _, cat := range Categories {
		for _, f := range cat.Files {
			covered[f] = true
		}
	}

	for name := range DefaultConfigs {
		if !covered[name] {
			t.Errorf("DefaultConfigs key %q does not appear in any Category.Files", name)
		}
	}
}

func TestCategoriesFilesExist(t *testing.T) {
	for _, cat := range Categories {
		for _, f := range cat.Files {
			if _, ok := DefaultConfigs[f]; !ok {
				t.Errorf("Categories[%q].Files contains %q which is not a key in DefaultConfigs", cat.Label, f)
			}
		}
	}
}

func TestGenerateConfigsDisabledBuiltins(t *testing.T) {
	dir := t.TempDir()

	disabled := map[string]bool{
		"kubectl": true,
		"docker":  true,
	}
	_, _, err := GenerateConfigs(dir, map[string]bool{}, disabled, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatalf("reading config.toml: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "[builtins]") {
		t.Error("expected [builtins] section in config.toml")
	}
	if !strings.Contains(content, "kubectl = false") {
		t.Error("expected kubectl = false in config.toml")
	}
	if !strings.Contains(content, "docker = false") {
		t.Error("expected docker = false in config.toml")
	}
}

func TestGenerateConfigsNoDisabledBuiltins(t *testing.T) {
	dir := t.TempDir()

	_, _, err := GenerateConfigs(dir, map[string]bool{}, nil, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatalf("reading config.toml: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "[builtins]") {
		t.Error("expected [builtins] section in config.toml")
	}
	for _, name := range AllBuiltins() {
		if !strings.Contains(content, name+" = true") {
			t.Errorf("expected %s = true in config.toml", name)
		}
	}
}
