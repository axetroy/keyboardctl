//go:build windows

// Command keyboardctl injects keyboard events into Windows via the
// KeyboardSimulator kernel driver.
//
// Prerequisites:
//
//	1. Build the driver:   cd driver && build -cZ
//	2. Install the driver: scripts\install.bat
//	3. Run the CLI (Administrator): keyboardctl.exe -key 0x1E
//
// Usage:
//
//	keyboardctl -key <scancode>              press a single key by hex scan code
//	keyboardctl -text <string> [-delay <ms>] type a string of text
//	keyboardctl -combo <mod> <key>          press modifier+key combo
//	keyboardctl -hold <scancode>            send key-down only
//	keyboardctl -release <scancode>         send key-up only
//
// Named key aliases (case-insensitive) can be used wherever a scan code is
// expected (e.g. -key enter, -combo ctrl c, -key f12).
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const usageText = `keyboardctl — inject keyboard events via the KeyboardSimulator driver

USAGE:
    keyboardctl -key    <key>                   press and release a key
    keyboardctl -hold   <key>                   send key-down event only
    keyboardctl -release <key>                  send key-up event only
    keyboardctl -combo  <modifier> <key>        press modifier+key simultaneously
    keyboardctl -text   <string> [-delay <ms>]  type a string character by character

KEY ARGUMENT:
    A key is either a hex scan code (e.g. 0x1E) or a name (case-insensitive):

    Letters:   a-z                   Numbers:  0-9
    Specials:  enter, space, tab, backspace, esc, delete, insert
    Arrows:    up, down, left, right, home, end, pageup, pagedown
    Modifiers: ctrl, shift, alt, lctrl, rctrl, lshift, rshift, lalt, ralt,
               win, lwin, rwin, apps
    Function:  f1-f12
    Other:     capslock, numlock, scrolllock, print, minus, equal, comma, dot,
               slash, backslash, semicolon, apostrophe, grave, leftbrace, rightbrace

EXAMPLES:
    keyboardctl.exe -key 0x1E           # press A
    keyboardctl.exe -key a              # press A (by name)
    keyboardctl.exe -key enter
    keyboardctl.exe -combo ctrl c       # Ctrl+C
    keyboardctl.exe -combo lalt f4      # Alt+F4
    keyboardctl.exe -text "Hello World" -delay 100
    keyboardctl.exe -hold lshift        # hold Shift
    keyboardctl.exe -release lshift     # release Shift

REQUIREMENTS:
    - KeyboardSimulator driver must be installed and running
    - Must be run as Administrator

DRIVER SETUP:
    cd driver && build -cZ
    scripts\install.bat
`

func main() {
	fs := flag.NewFlagSet("keyboardctl", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usageText) }

	keyFlag     := fs.String("key", "", "press and release a key (scan code or name)")
	holdFlag    := fs.String("hold", "", "send key-down only")
	releaseFlag := fs.String("release", "", "send key-up only")
	comboFlag   := fs.String("combo", "", "modifier+key combo, e.g. -combo ctrl c")
	textFlag    := fs.String("text", "", "type a string of text")
	delayFlag   := fs.Int("delay", 0, "inter-keystroke delay in milliseconds (used with -text)")

	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}

	// Require at least one action flag
	nActions := countNonEmpty(*keyFlag, *holdFlag, *releaseFlag, *comboFlag, *textFlag)
	if nActions == 0 {
		fmt.Fprint(os.Stderr, usageText)
		os.Exit(1)
	}
	if nActions > 1 {
		fatalf("specify only one action flag at a time\n")
	}

	drv, err := Open()
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"error: %v\n\nIs the KeyboardSimulator driver installed and running?\n"+
				"  scripts\\install.bat\n", err)
		os.Exit(1)
	}
	defer drv.Close()

	switch {
	case *keyFlag != "":
		sc := mustParseScanCode(*keyFlag)
		if err := drv.TapKey(sc); err != nil {
			fatalf("%v\n", err)
		}

	case *holdFlag != "":
		sc := mustParseScanCode(*holdFlag)
		if err := drv.PressKey(sc); err != nil {
			fatalf("%v\n", err)
		}

	case *releaseFlag != "":
		sc := mustParseScanCode(*releaseFlag)
		if err := drv.ReleaseKey(sc); err != nil {
			fatalf("%v\n", err)
		}

	case *comboFlag != "":
		// Accept either "-combo ctrl c" or remaining args after the flag
		parts := strings.Fields(*comboFlag)
		remaining := fs.Args()
		parts = append(parts, remaining...)
		if len(parts) < 2 {
			fatalf("-combo requires two arguments: <modifier> <key>\n")
		}
		mod := mustParseScanCode(parts[0])
		key := mustParseScanCode(parts[1])
		if err := drv.PressModifierAndKey(mod, key); err != nil {
			fatalf("%v\n", err)
		}

	case *textFlag != "":
		delay := time.Duration(*delayFlag) * time.Millisecond
		if err := drv.TypeText(*textFlag, delay); err != nil {
			fatalf("%v\n", err)
		}
	}
}

// mustParseScanCode parses a key argument as either a hex scan code (0x1E) or
// a human-readable name (enter, ctrl, f1, …).  Exits on failure.
func mustParseScanCode(s string) ScanCode {
	// Try hex scan code first (0x...)
	if strings.HasPrefix(strings.ToLower(s), "0x") {
		v, err := strconv.ParseUint(s[2:], 16, 16)
		if err != nil {
			fatalf("invalid scan code %q: %v\n", s, err)
		}
		return ScanCode(v)
	}
	// Try name lookup
	sc, ok := ParseScanCode(s)
	if !ok {
		fatalf("unknown key name %q — run keyboardctl with no flags for help\n", s)
	}
	return sc
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

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format, args...)
	os.Exit(1)
}
