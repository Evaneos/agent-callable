package spec

import (
	"fmt"
	"slices"
	"strings"
)

// Allow returns an allow result.
func Allow() Result { return Result{Decision: DecisionAllow} }

// Deny returns a deny result with the given reason.
func Deny(reason string) Result { return Result{Decision: DecisionDeny, Reason: reason} }

// CheckPreamble validates that args are non-empty. Returns a Deny result
// if validation fails, or a zero Result if OK.
// Control characters are checked by the engine before calling Check.
func CheckPreamble(toolName string, args []string) (Result, bool) {
	if len(args) == 0 {
		return Deny(fmt.Sprintf("%s requires a subcommand", toolName)), false
	}
	return Result{}, true
}

// FirstNonFlag returns the first positional (non-flag) argument.
// flagsWithValue lists flags that consume the next argument (may be nil).
func FirstNonFlag(args []string, flagsWithValue map[string]bool) string {
	if all := AllNonFlags(args, flagsWithValue); len(all) > 0 {
		return all[0]
	}
	return ""
}

// NthNonFlag returns the nth positional (non-flag) argument (1-indexed).
// flagsWithValue lists flags that consume the next argument (may be nil).
func NthNonFlag(args []string, n int, flagsWithValue map[string]bool) string {
	all := AllNonFlags(args, flagsWithValue)
	if n >= 1 && n <= len(all) {
		return all[n-1]
	}
	return ""
}

// CountNonFlags returns the number of positional (non-flag) arguments.
// flagsWithValue lists flags that consume the next argument (may be nil).
func CountNonFlags(args []string, flagsWithValue map[string]bool) int {
	return len(AllNonFlags(args, flagsWithValue))
}

// AllNonFlags returns all positional (non-flag) arguments.
// flagsWithValue lists flags that consume the next argument (may be nil).
func AllNonFlags(args []string, flagsWithValue map[string]bool) []string {
	var tokens []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			break
		}
		if strings.HasPrefix(a, "-") {
			if flagsWithValue[a] {
				i++
			}
			continue
		}
		tokens = append(tokens, a)
	}
	return tokens
}

// AllPositionalArgs returns all positional (non-flag) arguments, including those after --.
// Before --: skips flags and their values per flagsWithValue.
// After --: all remaining arguments are positional.
func AllPositionalArgs(args []string, flagsWithValue map[string]bool) []string {
	var tokens []string
	afterDash := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !afterDash && a == "--" {
			afterDash = true
			continue
		}
		if afterDash {
			tokens = append(tokens, a)
			continue
		}
		if strings.HasPrefix(a, "-") {
			if flagsWithValue[a] {
				i++
			}
			continue
		}
		tokens = append(tokens, a)
	}
	return tokens
}

// ContainsFlag checks if args contains the given flag (exact match or --flag=... prefix).
func ContainsFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag || strings.HasPrefix(a, flag+"=") {
			return true
		}
	}
	return false
}

// ContainsAny checks if any argument exactly matches one of the needles.
func ContainsAny(args []string, needles ...string) bool {
	for _, a := range args {
		if slices.Contains(needles, a) {
			return true
		}
	}
	return false
}

// ContainsAnyNonFlag checks if any positional (non-flag) argument matches one of the needles.
// flagsWithValue lists flags that consume the next argument (may be nil).
func ContainsAnyNonFlag(args []string, flagsWithValue map[string]bool, needles ...string) bool {
	set := make(map[string]struct{}, len(needles))
	for _, n := range needles {
		set[n] = struct{}{}
	}
	for _, a := range AllNonFlags(args, flagsWithValue) {
		if _, ok := set[a]; ok {
			return true
		}
	}
	return false
}

// SplitFlag splits --flag=value into (--flag, value).
// If there is no =, returns (flag, "").
func SplitFlag(arg string) (string, string) {
	if flag, val, ok := strings.Cut(arg, "="); ok {
		return flag, val
	}
	return arg, ""
}
