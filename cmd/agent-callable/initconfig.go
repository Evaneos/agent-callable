package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/evaneos/agent-callable/internal/config"
	"github.com/evaneos/agent-callable/internal/shell"
)

func initConfig() int {
	dir := config.ConfigBaseDir()
	toolsDir := filepath.Join(dir, "tools.d")
	globalPath := filepath.Join(dir, "config.toml")

	scanner := bufio.NewScanner(os.Stdin)
	selected := make(map[string]bool)
	disabledBuiltins := make(map[string]bool)

	// Handle existing config.toml upfront.
	overwriteGlobal := false
	globalExists := fileExists(globalPath)
	if globalExists {
		fmt.Printf("config.toml already exists at %s\n", globalPath)
		fmt.Print("Keep existing config.toml? [Y]es/[n]o: ")
		if !scanner.Scan() {
			return 0
		}
		if strings.TrimSpace(strings.ToLower(scanner.Text())) == "n" {
			fmt.Print("Are you sure? This will overwrite your config. [y/N]: ")
			if !scanner.Scan() {
				return 0
			}
			if strings.TrimSpace(strings.ToLower(scanner.Text())) == "y" {
				overwriteGlobal = true
			}
		}
		fmt.Println()
	}

	fmt.Println("Select which tool categories to install.")
	fmt.Println()

	// Ask about writable_dirs when generating a fresh config.
	var writableDirs []string
	if !globalExists || overwriteGlobal {
		fmt.Println("Writable directories (write operations are checked against these):")
		candidates := []string{"/tmp"}
		if home := os.Getenv("HOME"); home != "" {
			candidates = append(candidates, home)
		}
		for _, candidate := range candidates {
			fmt.Printf("  %s [Y]es/[n]o/[q]uit: ", candidate)
			if !scanner.Scan() {
				break
			}
			answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
			if answer == "q" {
				fmt.Println("\nAborted.")
				return 0
			}
			if answer != "n" { // default yes
				writableDirs = append(writableDirs, candidate)
			}
		}
		fmt.Println()
	}

	for _, cat := range shell.Categories {
		fmt.Printf("%s\n", cat.Label)
		fmt.Printf("Commands: %s\n", cat.Desc)

		// Categories with only builtins: gated by config.toml.
		if len(cat.Files) == 0 {
			if globalExists && !overwriteGlobal {
				fmt.Println("  [skipped]")
				fmt.Println()
				continue
			}
		} else {
			// File categories: skip if all files already exist.
			if allFilesExist(toolsDir, cat.Files) {
				fmt.Println("  [already installed]")
				fmt.Println()
				continue
			}
		}

		fmt.Print("Install? [Y]es/[n]o/[q]uit ")
		if !scanner.Scan() {
			break
		}
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))

		switch answer {
		case "q":
			fmt.Println("\nAborted.")
			return 0
		case "n":
			for _, b := range cat.Builtins {
				disabledBuiltins[b] = true
			}
			fmt.Println()
		default: // "", "y", "yes"
			for _, f := range cat.Files {
				selected[f] = true
			}
			fmt.Println()
		}
	}

	fmt.Println()

	created, skipped, err := shell.GenerateConfigs(dir, selected, disabledBuiltins, writableDirs, overwriteGlobal)
	if err != nil {
		fmt.Fprintf(os.Stderr, "agent-callable: %v\n", err)
		return 1
	}

	slices.Sort(created)
	slices.Sort(skipped)
	for _, name := range created {
		fmt.Printf("  Created %s\n", name)
	}
	for _, name := range skipped {
		fmt.Printf("  Skipped %s (already exists)\n", name)
	}
	fmt.Printf("\nDefaults generated in %s\n", dir)
	fmt.Println("Run agent-callable --help-config for format documentation.")
	return 0
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func allFilesExist(toolsDir string, files []string) bool {
	if len(files) == 0 {
		return false
	}
	for _, f := range files {
		if !fileExists(filepath.Join(toolsDir, f)) {
			return false
		}
	}
	return true
}
