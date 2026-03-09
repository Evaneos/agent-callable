package shell

// safeBuiltins is the set of shell builtins allowed in --sh mode.
// Builtins that can bypass command validation (eval, exec, source, ., command, builtin, trap) are excluded.
var safeBuiltins = map[string]bool{
	"echo": true, "printf": true,
	"true": true, "false": true, ":": true,
	"test": true, "[": true,
	"break": true, "continue": true, "return": true, "exit": true,
	"export": true, "local": true, "declare": true, "readonly": true,
	"typeset": true, "unset": true, "set": true, "shift": true, "let": true,
	"pwd": true, "type": true, "hash": true, "read": true,
	"wait": true, "getopts": true,
	"cd": true, "pushd": true, "popd": true, "dirs": true,
}

// dangerousBuiltins are builtins that must be blocked because they can
// bypass the command validation or execute arbitrary code.
var dangerousBuiltins = map[string]bool{
	"eval":    true,
	"exec":    true,
	"source":  true,
	".":       true,
	"command": true,
	"builtin": true,
	"trap":    true,
}
