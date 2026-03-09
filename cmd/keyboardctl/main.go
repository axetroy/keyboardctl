//go:build linux

// Command keyboardctl injects keyboard events into the Linux input subsystem
// via the keyboardctl kernel module.
//
// Usage:
//
//	keyboardctl type <text>         — type a string of text
//	keyboardctl press <key>         — press and release a key
//	keyboardctl down  <key>         — send key-down event
//	keyboardctl up    <key>         — send key-up event
//	keyboardctl combo <key> [key…]  — press keys simultaneously (e.g. Ctrl+C)
//
// Prerequisites:
//
//	The keyboardctl kernel module must be loaded:
//	    sudo insmod driver/keyboardctl.ko
//
// The device /dev/keyboardctl must be writable by the current user.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/axetroy/keyboardctl/pkg/keyboard"
)

const usage = `keyboardctl — inject keyboard events via the keyboardctl kernel module

USAGE:
    keyboardctl <command> [arguments]

COMMANDS:
    type  <text>          Type a string of text (US keyboard layout)
    press <key>           Press and release a key
    down  <key>           Send key-down event only
    up    <key>           Send key-up event only
    combo <key> [key…]    Press multiple keys simultaneously (e.g. Ctrl+C)

KEY NAMES (case-insensitive):
    Letters:    a-z
    Numbers:    0-9
    Specials:   enter, space, tab, backspace, esc, delete, insert
    Arrows:     up, down, left, right, home, end, pageup, pagedown
    Modifiers:  ctrl, shift, alt, leftctrl, rightctrl, leftshift,
                rightshift, leftalt, rightalt, leftmeta, rightmeta
    Function:   f1-f12
    Other:      capslock, numlock, scrolllock, pause, print, sysrq,
                minus, equal, comma, dot, slash, backslash, semicolon,
                apostrophe, grave, leftbrace, rightbrace

EXAMPLES:
    keyboardctl type "Hello, World!"
    keyboardctl press enter
    keyboardctl combo ctrl c
    keyboardctl combo leftshift f10
    keyboardctl down leftshift
    keyboardctl up   leftshift

DEVICE:
    Default device: ` + keyboard.DefaultDevice + `
    Override with KEYBOARDCTL_DEVICE environment variable.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	cmd := strings.ToLower(os.Args[1])
	args := os.Args[2:]

	devicePath := os.Getenv("KEYBOARDCTL_DEVICE")
	if devicePath == "" {
		devicePath = keyboard.DefaultDevice
	}

	kb, err := keyboard.OpenDevice(devicePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n\nIs the keyboardctl kernel module loaded?\n  sudo insmod driver/keyboardctl.ko\n", err)
		os.Exit(1)
	}
	defer kb.Close()

	switch cmd {
	case "type":
		if len(args) < 1 {
			fatalf("usage: keyboardctl type <text>\n")
		}
		if err := kb.Type(strings.Join(args, " ")); err != nil {
			fatalf("type: %v\n", err)
		}

	case "press":
		if len(args) < 1 {
			fatalf("usage: keyboardctl press <key>\n")
		}
		kc := mustParseKey(args[0])
		if err := kb.Press(kc); err != nil {
			fatalf("press: %v\n", err)
		}

	case "down":
		if len(args) < 1 {
			fatalf("usage: keyboardctl down <key>\n")
		}
		kc := mustParseKey(args[0])
		if err := kb.KeyDown(kc); err != nil {
			fatalf("down: %v\n", err)
		}

	case "up":
		if len(args) < 1 {
			fatalf("usage: keyboardctl up <key>\n")
		}
		kc := mustParseKey(args[0])
		if err := kb.KeyUp(kc); err != nil {
			fatalf("up: %v\n", err)
		}

	case "combo":
		if len(args) < 1 {
			fatalf("usage: keyboardctl combo <key> [key…]\n")
		}
		keys := make([]keyboard.KeyCode, 0, len(args))
		for _, name := range args {
			keys = append(keys, mustParseKey(name))
		}
		if err := kb.Combo(keys...); err != nil {
			fatalf("combo: %v\n", err)
		}

	case "help", "--help", "-h":
		fmt.Fprint(os.Stdout, usage)

	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", cmd)
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}
}

func mustParseKey(name string) keyboard.KeyCode {
	kc, ok := keyboard.ParseKeyCode(name)
	if !ok {
		fatalf("unknown key name %q — run 'keyboardctl help' for a list of key names\n", name)
	}
	return kc
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format, args...)
	os.Exit(1)
}
