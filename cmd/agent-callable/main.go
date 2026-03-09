package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/evaneos/agent-callable/internal/config"
	"github.com/evaneos/agent-callable/internal/engine"
	"github.com/evaneos/agent-callable/internal/execx"
	"github.com/evaneos/agent-callable/internal/shell"
)

// Set at build time via -ldflags "-X main.version=..."
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		printUsage()
		return 2
	}

	// Meta flags: if first arg starts with --, handle it and return.
	if strings.HasPrefix(args[0], "--") || args[0] == "-v" || args[0] == "-h" {
		return runMeta(args)
	}

	// Normal mode: dispatch to engine.
	e := engine.NewDefault()
	defer e.Close()
	exitCode, err := e.Run(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
	return exitCode
}

func runMeta(args []string) int {
	switch args[0] {
	case "--help", "-h":
		printUsage()
		return 0
	case "--version", "-v":
		fmt.Printf("agent-callable %s\n", version)
		return 0
	case "--list-tools":
		e := engine.NewDefault()
		defer e.Close()
		for _, t := range e.ListTools() {
			fmt.Printf("  %-25s [%s]\n", t.Name, t.Source)
		}
		return 0
	case "--help-config":
		fmt.Print(config.HelpText)
		return 0
	case "--init-config":
		return initConfig()
	case "--claude":
		if len(args) < 2 {
			return 0
		}
		return runClaude(args[1:])
	case "--audit":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "agent-callable: --audit requires <tool> [args...]")
			return 2
		}
		return runAudit(args[1:])
	case "--sh":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "agent-callable: --sh requires a shell expression")
			return 2
		}
		return runShell(args[1])
	default:
		fmt.Fprintf(os.Stderr, "agent-callable: unknown flag %q\n", args[0])
		printUsage()
		return 2
	}
}

const claudeAllow = `{"continue":true,"suppressOutput":false,"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow","permissionDecisionReason":""}}`

func runClaude(args []string) int {
	if len(args) == 1 {
		return runClaudeShell(args[0])
	}
	return runClaudeTool(args)
}

func runClaudeShell(expr string) int {
	e := engine.NewDefault()
	defer e.Close()

	_, err := shell.Validate(expr, e.Registry(), e.ShellValidateOpts())
	if err == nil {
		if e.GlobalConfig().Audit.IncludeAuditChecks {
			e.Audit().Log("ALLOWED", "--claude --sh", []string{expr})
		}
		fmt.Println(claudeAllow)
	} else {
		if e.GlobalConfig().Audit.IncludeAuditChecks {
			e.Audit().Log("BLOCKED", "--claude --sh", []string{expr})
		}
	}
	return 0
}

func runClaudeTool(args []string) int {
	e := engine.NewDefault()
	defer e.Close()
	cr := e.Check(args)
	if cr.Allowed {
		if e.GlobalConfig().Audit.IncludeAuditChecks {
			e.Audit().Log("ALLOWED", "--claude", cr.Args)
		}
		fmt.Println(claudeAllow)
	} else if cr.Tool != "" {
		if e.GlobalConfig().Audit.IncludeAuditChecks {
			e.Audit().Log("BLOCKED", "--claude", cr.Args)
		}
	}
	return 0
}

func runAudit(args []string) int {
	if args[0] == "--sh" {
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "agent-callable: --audit --sh requires a shell expression")
			return 2
		}
		return runAuditShell(args[1])
	}

	e := engine.NewDefault()
	defer e.Close()
	cr := e.Check(args)

	if e.GlobalConfig().Audit.IncludeAuditChecks {
		label := "AUDIT_ALLOWED"
		if !cr.Allowed {
			label = "AUDIT_BLOCKED"
		}
		e.Audit().Log(label, cr.Tool, cr.Args)
	}

	if !cr.Allowed {
		fmt.Fprintln(os.Stderr, cr.Reason)
		return 1
	}
	return 0
}

func runAuditShell(expr string) int {
	e := engine.NewDefault()
	defer e.Close()

	_, err := shell.Validate(expr, e.Registry(), e.ShellValidateOpts())

	if e.GlobalConfig().Audit.IncludeAuditChecks {
		label := "AUDIT_ALLOWED"
		if err != nil {
			label = "AUDIT_BLOCKED"
		}
		e.Audit().Log(label, "--sh", []string{expr})
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "agent-callable --audit --sh: %v\n", err)
		return 1
	}
	return 0
}

func runShell(expr string) int {
	e := engine.NewDefault()
	defer e.Close()

	result, err := shell.Validate(expr, e.Registry(), e.ShellValidateOpts())
	if err != nil {
		e.Audit().Log("BLOCKED", "--sh", []string{expr})
		fmt.Fprintf(os.Stderr, "agent-callable --sh: %v\n", err)
		return 1
	}

	e.Audit().Log("ALLOWED", "--sh", []string{expr})

	env := e.ShellEnv(result.ToolNames)

	exitCode, err := execx.ExecShell(execx.ShellPlan{
		Expr: expr,
		Env:  env,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
	return exitCode
}

func printUsage() {
	fmt.Print(`agent-callable — deny-by-default command filter for LLM agents

Usage:
  agent-callable <command> [args...]              Run a command through the filter
  agent-callable --sh '<expression>'              Run a validated shell expression
  agent-callable --claude '<expression>'          Claude hook: validate shell expression
  agent-callable --claude <tool> [args...]        Claude hook: validate a command
  agent-callable --audit <tool> [args...]         Dry-run: check a simple command
  agent-callable --audit --sh '<expression>'      Dry-run: check a shell expression
  agent-callable --help                    Show this help
  agent-callable --version                 Show version
  agent-callable --list-tools              List all registered tools (built-in + config)
  agent-callable --help-config             Show config file format documentation
  agent-callable --init-config              Generate default configs for common tools
`)
}
