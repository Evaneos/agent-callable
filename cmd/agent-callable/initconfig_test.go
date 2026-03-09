package main

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/evaneos/agent-callable/internal/shell"
)

// withStdin replaces os.Stdin with a pipe containing input, runs fn, then restores os.Stdin.
func withStdin(input string, fn func()) {
	r, w, _ := os.Pipe()
	old := os.Stdin
	os.Stdin = r

	_, _ = io.WriteString(w, input)
	w.Close()

	fn()

	os.Stdin = old
	r.Close()
}

// allCategoryFiles returns the set of all .toml file names referenced by shell.Categories.
func allCategoryFiles() map[string]bool {
	files := make(map[string]bool)
	for _, cat := range shell.Categories {
		for _, f := range cat.Files {
			files[f] = true
		}
	}
	return files
}

// allCategoryBuiltins returns the set of all builtin names referenced by shell.Categories.
func allCategoryBuiltins() map[string]bool {
	builtins := make(map[string]bool)
	for _, cat := range shell.Categories {
		for _, b := range cat.Builtins {
			builtins[b] = true
		}
	}
	return builtins
}

func TestInitConfigAllYes(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGENT_CALLABLE_CONFIG_DIR", dir)

	// 2 writable_dirs prompts (/tmp, $HOME) then all categories.
	input := "y\ny\n" + strings.Repeat("y\n", len(shell.Categories))

	var code int
	captureStdout(func() {
		withStdin(input, func() {
			code = initConfig()
		})
	})

	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	if _, err := os.Stat(filepath.Join(dir, "config.toml")); err != nil {
		t.Errorf("config.toml not found: %v", err)
	}

	// All TOML files must exist in tools.d/.
	toolsDir := filepath.Join(dir, "tools.d")
	for name := range allCategoryFiles() {
		if _, err := os.Stat(filepath.Join(toolsDir, name)); err != nil {
			t.Errorf("expected tools.d/%s to exist: %v", name, err)
		}
	}

	// config.toml should have all builtins = true.
	data, _ := os.ReadFile(filepath.Join(dir, "config.toml"))
	content := string(data)
	if !strings.Contains(content, "[builtins]") {
		t.Fatal("expected [builtins] section in config.toml")
	}
	for name := range allCategoryBuiltins() {
		if !strings.Contains(content, name+" = true") {
			t.Errorf("expected %s = true in config.toml", name)
		}
	}
}

func TestInitConfigAllNo(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGENT_CALLABLE_CONFIG_DIR", dir)

	// 2 writable_dirs prompts then all categories.
	input := "y\ny\n" + strings.Repeat("n\n", len(shell.Categories))

	var code int
	captureStdout(func() {
		withStdin(input, func() {
			code = initConfig()
		})
	})

	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	if _, err := os.Stat(filepath.Join(dir, "config.toml")); err != nil {
		t.Errorf("config.toml not found: %v", err)
	}

	// No .toml files in tools.d/.
	toolsDir := filepath.Join(dir, "tools.d")
	entries, err := os.ReadDir(toolsDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("reading tools.d: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".toml") {
			t.Errorf("unexpected file in tools.d: %s", e.Name())
		}
	}

	// All builtins must be disabled in config.toml.
	data, _ := os.ReadFile(filepath.Join(dir, "config.toml"))
	content := string(data)
	if !strings.Contains(content, "[builtins]") {
		t.Fatal("expected [builtins] section in config.toml")
	}
	for name := range allCategoryBuiltins() {
		if !strings.Contains(content, name+" = false") {
			t.Errorf("expected %s = false in config.toml", name)
		}
	}
}

func TestInitConfigQuit(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGENT_CALLABLE_CONFIG_DIR", dir)

	var code int
	out := captureStdout(func() {
		withStdin("q\n", func() {
			code = initConfig()
		})
	})

	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "Aborted") {
		t.Errorf("expected stdout to contain 'Aborted', got: %q", out)
	}
}

func TestInitConfigDefaultIsYes(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGENT_CALLABLE_CONFIG_DIR", dir)

	// 2 writable_dirs prompts (default y) then all categories (default y).
	input := "\n\n" + strings.Repeat("\n", len(shell.Categories))

	var code int
	captureStdout(func() {
		withStdin(input, func() {
			code = initConfig()
		})
	})

	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	if _, err := os.Stat(filepath.Join(dir, "config.toml")); err != nil {
		t.Errorf("config.toml not found: %v", err)
	}

	// All category .toml files must exist.
	toolsDir := filepath.Join(dir, "tools.d")
	for name := range allCategoryFiles() {
		if _, err := os.Stat(filepath.Join(toolsDir, name)); err != nil {
			t.Errorf("expected tools.d/%s to exist: %v", name, err)
		}
	}

	// All builtins enabled.
	data, _ := os.ReadFile(filepath.Join(dir, "config.toml"))
	content := string(data)
	for name := range allCategoryBuiltins() {
		if !strings.Contains(content, name+" = true") {
			t.Errorf("expected %s = true in config.toml", name)
		}
	}
}

func TestInitConfigMixed(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGENT_CALLABLE_CONFIG_DIR", dir)

	// 2 writable_dirs prompts then n to first category (Git & GitHub), y to the rest.
	input := "y\ny\n" + "n\n" + strings.Repeat("y\n", len(shell.Categories)-1)

	var code int
	captureStdout(func() {
		withStdin(input, func() {
			code = initConfig()
		})
	})

	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	// First category is "Git & GitHub" with builtins git, gh.
	data, _ := os.ReadFile(filepath.Join(dir, "config.toml"))
	content := string(data)
	if !strings.Contains(content, "[builtins]") {
		t.Fatal("expected [builtins] section in config.toml")
	}
	if !strings.Contains(content, "gh = false") {
		t.Error("expected gh = false in config.toml")
	}
	if !strings.Contains(content, "git = false") {
		t.Error("expected git = false in config.toml")
	}
	// kubectl should NOT be disabled (Kubernetes was "y").
	if strings.Contains(content, "kubectl = false") {
		t.Error("kubectl should not be disabled")
	}
}

func TestInitConfigKeepExisting(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGENT_CALLABLE_CONFIG_DIR", dir)

	// Put a recognizable config.toml.
	original := `writable_dirs = ["/custom-kept"]`
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	// Count how many file categories will need a prompt (none of their files exist).
	toolsDir := filepath.Join(dir, "tools.d")
	nPrompts := 0
	for _, cat := range shell.Categories {
		if len(cat.Files) == 0 || allFilesExist(toolsDir, cat.Files) {
			continue
		}
		nPrompts++
	}

	// keep (default Y) + default Y for each file category prompt.
	input := "\n" + strings.Repeat("\n", nPrompts)

	var code int
	captureStdout(func() {
		withStdin(input, func() {
			code = initConfig()
		})
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "config.toml"))
	if !strings.Contains(string(data), "/custom-kept") {
		t.Errorf("expected original config.toml preserved, got:\n%s", data)
	}
}

func TestInitConfigOverwriteExisting(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGENT_CALLABLE_CONFIG_DIR", dir)

	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(`writable_dirs = ["/old"]`), 0644); err != nil {
		t.Fatal(err)
	}

	// n → sure? y → overwrite; y y for writable_dirs; default y for categories.
	input := "n\ny\n" + "y\ny\n" + strings.Repeat("\n", len(shell.Categories))

	var code int
	captureStdout(func() {
		withStdin(input, func() {
			code = initConfig()
		})
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "config.toml"))
	content := string(data)
	// The old content should be gone; the new one has [builtins].
	if strings.Contains(content, "/old") {
		t.Error("old config should have been overwritten")
	}
	if !strings.Contains(content, "[builtins]") {
		t.Errorf("expected new [builtins] section in overwritten config")
	}
}

// TestInitConfigInstalledFilesImplyEnabledBuiltins verifies that when a
// category's TOML file already exists, the regenerated config.toml still
// enables the associated builtins (they are not disabled by default).
func TestInitConfigInstalledFilesImplyEnabledBuiltins(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGENT_CALLABLE_CONFIG_DIR", dir)

	// Pre-install kubernetes.toml so the Kubernetes category is [already installed].
	toolsDir := filepath.Join(dir, "tools.d")
	if err := os.MkdirAll(toolsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(toolsDir, "kubernetes.toml"), []byte(`[kubectl-readonly]
allowed = ["*"]
`), 0644); err != nil {
		t.Fatal(err)
	}

	// fresh start: y y for writable_dirs, default y for all categories.
	input := "y\ny\n" + strings.Repeat("\n", len(shell.Categories))

	var code int
	captureStdout(func() {
		withStdin(input, func() {
			code = initConfig()
		})
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "config.toml"))
	content := string(data)
	// kubectl and docker must be enabled because kubernetes.toml was already installed.
	if !strings.Contains(content, "kubectl = true") {
		t.Error("expected kubectl = true (kubernetes.toml already installed)")
	}
	if !strings.Contains(content, "docker = true") {
		t.Error("expected docker = true (kubernetes.toml already installed)")
	}
}

// writableDirsInput builds the input for writable_dirs prompts followed by
// default-y for all categories. tmpAnswer and homeAnswer must be "y", "n", or "".
func writableDirsInput(tmpAnswer, homeAnswer string) string {
	return tmpAnswer + "\n" + homeAnswer + "\n" + strings.Repeat("\n", len(shell.Categories))
}

func TestInitConfigWritableDirsYY(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGENT_CALLABLE_CONFIG_DIR", dir)
	home := os.Getenv("HOME")

	var code int
	captureStdout(func() {
		withStdin(writableDirsInput("y", "y"), func() {
			code = initConfig()
		})
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "config.toml"))
	content := string(data)
	if !strings.Contains(content, `"/tmp"`) {
		t.Error("expected /tmp in writable_dirs")
	}
	if home != "" && !strings.Contains(content, `"`+home+`"`) {
		t.Errorf("expected %s in writable_dirs", home)
	}
}

func TestInitConfigWritableDirsYN(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGENT_CALLABLE_CONFIG_DIR", dir)
	home := os.Getenv("HOME")

	var code int
	captureStdout(func() {
		withStdin(writableDirsInput("y", "n"), func() {
			code = initConfig()
		})
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "config.toml"))
	content := string(data)
	if !strings.Contains(content, `"/tmp"`) {
		t.Error("expected /tmp in writable_dirs")
	}
	if home != "" && strings.Contains(content, `"`+home+`"`) {
		t.Errorf("expected %s NOT in writable_dirs", home)
	}
}

func TestInitConfigWritableDirsNN(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGENT_CALLABLE_CONFIG_DIR", dir)

	var code int
	captureStdout(func() {
		withStdin(writableDirsInput("n", "n"), func() {
			code = initConfig()
		})
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "config.toml"))
	content := string(data)
	if !strings.Contains(content, "writable_dirs = []") {
		t.Errorf("expected empty writable_dirs, got:\n%s", content)
	}
}
