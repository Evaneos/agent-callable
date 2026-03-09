package spec

// ContainsControlCharacters blocks dangerous bytes (null byte, DEL, etc.)
// to prevent parsing surprises or confusion attacks.
func ContainsControlCharacters(args []string) bool {
	for _, arg := range args {
		for _, r := range arg {
			// Allow \t \n \r (safely handled by the OS) but block the rest.
			if r == 0x00 ||
				(r >= 0x01 && r <= 0x08) ||
				(r >= 0x0E && r <= 0x1F) ||
				r == 0x0B ||
				r == 0x0C ||
				r == 0x7F {
				return true
			}
		}
	}
	return false
}
