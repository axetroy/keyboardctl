package main

// resolveKeyArg implements the bare-argument shorthand: "keyboardctl a" is the
// same as "keyboardctl -key a".  It applies only when -key was not given, there
// is exactly one positional argument, and no other action flag was set, so it
// can never shadow an explicit flag (for example the trailing key of -combo).
func resolveKeyArg(key string, args []string, otherFlags ...string) string {
	if key != "" || len(args) != 1 || countNonEmpty(otherFlags...) != 0 {
		return key
	}
	return args[0]
}

func countNonEmpty(ss ...string) int {
	n := 0
	for _, s := range ss {
		if s != "" {
			n++
		}
	}
	return n
}
