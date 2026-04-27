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

// CheckPreamble validates that args are non-empty and handles universal
// info flags. Returns (Allow, false) for --version/--help, (Deny, false)
// for empty args, or (zero, true) to continue with tool-specific checks.
// Control characters are checked by the engine before calling Check.
func CheckPreamble(toolName string, args []string) (Result, bool) {
	if len(args) == 0 {
		return Deny(fmt.Sprintf("%s requires a subcommand", toolName)), false
	}
	if len(args) == 1 && (args[0] == "--version" || args[0] == "--help") {
		return Allow(), false
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

// MatchFlag returns true if arg matches flag exactly, or — for long flags
// (--foo) — also if arg has the --foo=value form.
func MatchFlag(arg, flag string) bool {
	if arg == flag {
		return true
	}
	if strings.HasPrefix(flag, "--") {
		return strings.HasPrefix(arg, flag+"=")
	}
	return false
}

// MatchFlagOrShortPrefix is MatchFlag plus short-flag prefix matching:
// `-i` also matches `-i.bak`, `-iSUFFIX`, etc.
func MatchFlagOrShortPrefix(arg, flag string) bool {
	if MatchFlag(arg, flag) {
		return true
	}
	return !strings.HasPrefix(flag, "--") && strings.HasPrefix(arg, flag) && len(arg) > len(flag)
}

// FirstMatchingFlag walks args (stopping at "--") and returns the first
// flag from flags whose match against an arg succeeds, or "" if none.
func FirstMatchingFlag(args, flags []string, match func(arg, flag string) bool) string {
	for _, a := range args {
		if a == "--" {
			break
		}
		for _, f := range flags {
			if match(a, f) {
				return f
			}
		}
	}
	return ""
}

// ContainsFlag checks if args contains the given flag (exact match or --flag=... prefix).
func ContainsFlag(args []string, flag string) bool {
	for _, a := range args {
		if MatchFlag(a, flag) {
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
