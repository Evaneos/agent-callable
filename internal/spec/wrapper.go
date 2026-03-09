package spec

import (
	"fmt"
	"strings"
)

// extractInner finds the inner command in wrapper args.
// It walks the args, skipping the wrapper's own flags (per flagsWithValue)
// and `skip` positional args, then returns everything from the inner
// command position onwards as-is (preserving the inner command's flags).
func extractInner(args []string, flagsWithValue map[string]bool, skip int) (cmd string, cmdArgs []string, err error) {
	positionalsSeen := 0
	i := 0
	for i < len(args) {
		a := args[i]
		if a == "--" {
			i++
			// After --, consume remaining skips as plain positionals.
			for i < len(args) && positionalsSeen < skip {
				positionalsSeen++
				i++
			}
			break
		}
		// Once all wrapper positionals are consumed (and skip > 0),
		// the next arg is the inner command regardless of form.
		if skip > 0 && positionalsSeen >= skip {
			break
		}
		if strings.HasPrefix(a, "-") {
			if strings.ContainsRune(a, '=') {
				i++ // --flag=value: single token
			} else if flagsWithValue[a] {
				i += 2 // flag + separate value
			} else {
				i++ // boolean flag
			}
			continue
		}
		if positionalsSeen < skip {
			positionalsSeen++
			i++
			continue
		}
		break // skip == 0: first non-flag is the inner command
	}
	if i >= len(args) {
		return "", nil, fmt.Errorf("no inner command found")
	}
	if i+1 < len(args) {
		return args[i], args[i+1:], nil
	}
	return args[i], nil, nil
}

// InnerExtractor extracts the inner command and its args from wrapper args.
type InnerExtractor func(args []string) (cmd string, cmdArgs []string, err error)

// ExtractAfterFlags returns an extractor that treats all args after the
// wrapper's own flags as the inner command. Use for wrappers with no
// positional args of their own (nice, time).
func ExtractAfterFlags(flagsWithValue map[string]bool) InnerExtractor {
	return func(args []string) (string, []string, error) {
		return extractInner(args, flagsWithValue, 0)
	}
}

// ExtractAfterFlagsAndN returns an extractor that skips the wrapper's flags
// and N positional args before the inner command. Use for wrappers like
// timeout (N=1 for DURATION).
func ExtractAfterFlagsAndN(n int, flagsWithValue map[string]bool) InnerExtractor {
	return func(args []string) (string, []string, error) {
		return extractInner(args, flagsWithValue, n)
	}
}

// WrapperToolSpec implements ToolSpec for commands that wrap another command
// (timeout, nice, etc.). It extracts the inner command and delegates
// validation to a shared CheckFunc.
type WrapperToolSpec struct {
	name      string
	extract   InnerExtractor
	checkFunc func(name string, args []string) (extraArgs []string, err error)
}

// NewWrapper creates a WrapperToolSpec. Call SetCheckFunc before use.
func NewWrapper(name string, extract InnerExtractor) *WrapperToolSpec {
	return &WrapperToolSpec{name: name, extract: extract}
}

func (t *WrapperToolSpec) Name() string { return t.name }

func (t *WrapperToolSpec) NonInteractiveEnv() map[string]string { return nil }

// SetCheckFunc injects the function used to validate the inner command.
// The function returns extra args to inject (e.g. --non-interactive) and an error if blocked.
func (t *WrapperToolSpec) SetCheckFunc(fn func(string, []string) ([]string, error)) {
	t.checkFunc = fn
}

func (t *WrapperToolSpec) Check(args []string, _ RuntimeCtx) Result {
	// Control characters are checked by the engine before calling Check.
	if t.checkFunc == nil {
		return Deny(fmt.Sprintf("%s: wrapper not initialized (no check function)", t.name))
	}
	cmd, cmdArgs, err := t.extract(args)
	if err != nil {
		return Deny(fmt.Sprintf("%s: %s", t.name, err))
	}
	extraArgs, err := t.checkFunc(cmd, cmdArgs)
	if err != nil {
		return Deny(err.Error())
	}
	return Result{Decision: DecisionAllow, NonInteractiveArgs: extraArgs}
}
