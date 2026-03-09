package execx

import (
	"os"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// neutralizeStdin replaces stdin with /dev/null if it's a TTY,
// preventing interactive prompts from child processes.
func neutralizeStdin() {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		f, err := os.Open("/dev/null")
		if err == nil {
			_ = unix.Dup2(int(f.Fd()), int(os.Stdin.Fd()))
			_ = f.Close()
		}
	}
}
