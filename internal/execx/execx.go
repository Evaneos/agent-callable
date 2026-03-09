package execx

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

type ExecPlan struct {
	Tool string   // e.g. "kubectl"
	Args []string // args passed to the tool
	Env  []string // full environment (key=value)
}

func Exec(plan ExecPlan) (int, error) {
	if plan.Tool == "" {
		return 2, errors.New("agent-callable: missing tool")
	}

	toolPath, err := exec.LookPath(plan.Tool)
	if err != nil {
		return 127, fmt.Errorf("agent-callable: %s not found in PATH", plan.Tool)
	}

	// Basic anti-recursion: refuse to exec if the tool resolves to our own binary.
	self, selfErr := os.Executable()
	if selfErr == nil {
		selfAbs, _ := filepath.EvalSymlinks(self)
		toolAbs, _ := filepath.EvalSymlinks(toolPath)
		if selfAbs != "" && toolAbs != "" && selfAbs == toolAbs {
			return 126, fmt.Errorf("agent-callable: refusing to exec %q because it resolves to agent-callable (recursion risk)", plan.Tool)
		}
	}

	neutralizeStdin()

	execArgs := append([]string{plan.Tool}, plan.Args...)
	if err := syscall.Exec(toolPath, execArgs, plan.Env); err != nil {
		return 1, fmt.Errorf("agent-callable: error executing %s: %w", plan.Tool, err)
	}
	return 0, nil
}
