package execx

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"
)

// ShellPlan describes a shell expression to execute.
type ShellPlan struct {
	Expr string   // shell expression to pass to sh -c
	Env  []string // full environment (key=value)
}

// ExecShell executes a shell expression via sh -c, replacing the current process.
func ExecShell(plan ShellPlan) (int, error) {
	if plan.Expr == "" {
		return 2, errors.New("agent-callable: empty shell expression")
	}

	shPath, err := exec.LookPath("sh")
	if err != nil {
		return 127, fmt.Errorf("agent-callable: sh not found in PATH")
	}

	neutralizeStdin()

	execArgs := []string{"sh", "-c", plan.Expr}
	if err := syscall.Exec(shPath, execArgs, plan.Env); err != nil {
		return 1, fmt.Errorf("agent-callable: shell execution error: %w", err)
	}
	return 0, nil
}
