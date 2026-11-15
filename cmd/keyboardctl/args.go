package main

import (
	"fmt"
	"strconv"
	"strings"
)

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

// tryParseScanCode parses s as a hex scan code ("0x1E") or a key name
// ("enter", "ctrl").  Unlike mustParseScanCode it reports failure instead of
// exiting, so callers that need to handle the error themselves (and tests on
// any platform) can use it.
func tryParseScanCode(s string) (ScanCode, bool) {
	if strings.HasPrefix(strings.ToLower(s), "0x") {
		v, err := strconv.ParseUint(s[2:], 16, 16)
		if err != nil {
			return 0, false
		}
		return ScanCode(v), true
	}
	return ParseScanCode(s)
}

// scanCodeError describes why s is not a valid key argument, distinguishing a
// malformed hex scan code from an unknown name so the message points at the
// actual mistake.
func scanCodeError(s string) string {
	if strings.HasPrefix(strings.ToLower(s), "0x") {
		return fmt.Sprintf("invalid scan code %q", s)
	}
	return fmt.Sprintf("unknown key name %q — run \"keyboardctl -list\" to see every accepted name", s)
}

// parseCombo splits the arguments of -combo into zero or more modifiers and one
// final key: "ctrl shift a" -> ([ctrl, shift], a).
//
// Every argument before the last must be a modifier.  This is what removes the
// old silent truncation: previously "-combo ctrl shift a" parsed only the first
// two arguments and quietly sent Ctrl+Shift instead of Ctrl+Shift+A, and
// "-combo ctrl alt delete" quietly sent Ctrl+Alt instead of Ctrl+Alt+Del.  A
// wrong key combination is worse than a failed command, so anything that is not
// a modifier in a non-final position is an error.
//
// It returns an error rather than exiting so it can be unit-tested.
func parseCombo(parts []string) (mods []ScanCode, key ScanCode, err error) {
	if len(parts) < 2 {
		return nil, 0, fmt.Errorf("-combo needs a modifier and a key (e.g. -combo ctrl c)")
	}

	for i, p := range parts {
		sc, ok := tryParseScanCode(p)
		if !ok {
			return nil, 0, fmt.Errorf("-combo: %s", scanCodeError(p))
		}

		if i == len(parts)-1 {
			key = sc
			break
		}
		if !isModifier(sc) {
			return nil, 0, fmt.Errorf("-combo: %q is not a modifier, and only the last argument may be the key", p)
		}
		mods = append(mods, sc)
	}
	return mods, key, nil
}

// isModifier reports whether a scan code is a modifier key (Shift, Ctrl, Alt,
// Win).  It reuses the key classification from timing.go.
func isModifier(sc ScanCode) bool {
	return classOf(sc) == classModifier
}
