//go:build linux

// Package keyboard provides Go bindings for the keyboardctl kernel module.
//
// Architecture
//
// The package communicates with the keyboardctl character device
// (/dev/keyboardctl) by writing keyboardctl_event structs.  The kernel
// module receives those structs and injects them into the Linux input
// subsystem, which makes the keystrokes visible to every application
// listening on the virtual keyboard — exactly as if a physical keyboard
// had been pressed.
//
// Typical usage:
//
//	kb, err := keyboard.Open()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer kb.Close()
//
//	if err := kb.Type("Hello, World!"); err != nil {
//	    log.Fatal(err)
//	}
package keyboard

import (
	"encoding/binary"
	"fmt"
	"os"
	"unicode"
)

const (
	// DefaultDevice is the path of the character device created by the
	// keyboardctl kernel module.
	DefaultDevice = "/dev/keyboardctl"

	// Event types (mirror linux/input-event-codes.h)
	evSyn uint16 = 0x00
	evKey uint16 = 0x01

	// SYN sub-code
	synReport uint16 = 0

	// Key state values
	keyRelease int32 = 0
	keyPress   int32 = 1
)

// event is the binary payload written to the character device.
// Its layout must stay in sync with struct keyboardctl_event in
// driver/keyboardctl.h.
type event struct {
	Type  uint16
	Code  uint16
	Value int32
}

// Keyboard represents an open connection to the keyboardctl device.
type Keyboard struct {
	f *os.File
}

// Open opens the keyboardctl character device at DefaultDevice.
// The caller must call Close when finished.
func Open() (*Keyboard, error) {
	return OpenDevice(DefaultDevice)
}

// OpenDevice opens the keyboardctl character device at the given path.
// This is useful for testing with a temporary file.
func OpenDevice(path string) (*Keyboard, error) {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("keyboard: open device %q: %w", path, err)
	}
	return &Keyboard{f: f}, nil
}

// Close closes the connection to the keyboardctl device.
func (k *Keyboard) Close() error {
	return k.f.Close()
}

// KeyDown sends a key-press event for the given key code.
// Call sync after sending all desired events to flush them to listeners.
func (k *Keyboard) KeyDown(code KeyCode) error {
	return k.writeEvent(event{Type: evKey, Code: uint16(code), Value: keyPress})
}

// KeyUp sends a key-release event for the given key code.
// Call sync after sending all desired events to flush them to listeners.
func (k *Keyboard) KeyUp(code KeyCode) error {
	return k.writeEvent(event{Type: evKey, Code: uint16(code), Value: keyRelease})
}

// sync sends an EV_SYN/SYN_REPORT event, which causes the input core to
// deliver all pending events to listeners.
func (k *Keyboard) sync() error {
	return k.writeEvent(event{Type: evSyn, Code: synReport, Value: 0})
}

// Press presses and releases a single key.
func (k *Keyboard) Press(code KeyCode) error {
	if err := k.KeyDown(code); err != nil {
		return err
	}
	if err := k.sync(); err != nil {
		return err
	}
	if err := k.KeyUp(code); err != nil {
		return err
	}
	return k.sync()
}

// Combo presses all given keys simultaneously (modifier + key combos such as
// Ctrl+C) and then releases them in reverse order.
//
//	kb.Combo(keyboard.KeyLeftCtrl, keyboard.KeyC)
func (k *Keyboard) Combo(codes ...KeyCode) error {
	for _, c := range codes {
		if err := k.KeyDown(c); err != nil {
			return err
		}
	}
	if err := k.sync(); err != nil {
		return err
	}
	for i := len(codes) - 1; i >= 0; i-- {
		if err := k.KeyUp(codes[i]); err != nil {
			return err
		}
	}
	return k.sync()
}

// Type types the given text string one character at a time using the US-ASCII
// keyboard layout.  Uppercase letters are typed with Left Shift held down.
// Characters that cannot be mapped to a key code are silently skipped.
func (k *Keyboard) Type(text string) error {
	for _, r := range text {
		kc, shift, ok := runeToKey(r)
		if !ok {
			continue
		}
		if shift {
			if err := k.KeyDown(KeyLeftShift); err != nil {
				return err
			}
		}
		if err := k.KeyDown(kc); err != nil {
			return err
		}
		if err := k.sync(); err != nil {
			return err
		}
		if err := k.KeyUp(kc); err != nil {
			return err
		}
		if shift {
			if err := k.KeyUp(KeyLeftShift); err != nil {
				return err
			}
		}
		if err := k.sync(); err != nil {
			return err
		}
	}
	return nil
}

// writeEvent serialises a single event and writes it to the device.
func (k *Keyboard) writeEvent(ev event) error {
	if err := binary.Write(k.f, binary.LittleEndian, ev); err != nil {
		return fmt.Errorf("keyboard: write event: %w", err)
	}
	return nil
}

// runeToKey maps a Unicode rune to a (KeyCode, needsShift, ok) triple using
// the standard US-ASCII keyboard layout.
func runeToKey(r rune) (KeyCode, bool, bool) {
	// Direct lowercase / digit mapping
	if kc, ok := charKeyMap[r]; ok {
		return kc, false, true
	}
	// Uppercase letters → lowercase key + Shift
	if unicode.IsUpper(r) {
		lower := unicode.ToLower(r)
		if kc, ok := charKeyMap[lower]; ok {
			return kc, true, true
		}
	}
	// Shifted symbols
	if kc, ok := shiftedCharKeyMap[r]; ok {
		return kc, true, true
	}
	return 0, false, false
}

// charKeyMap maps unshifted characters to their key codes.
var charKeyMap = map[rune]KeyCode{
	'a': KeyA, 'b': KeyB, 'c': KeyC, 'd': KeyD, 'e': KeyE,
	'f': KeyF, 'g': KeyG, 'h': KeyH, 'i': KeyI, 'j': KeyJ,
	'k': KeyK, 'l': KeyL, 'm': KeyM, 'n': KeyN, 'o': KeyO,
	'p': KeyP, 'q': KeyQ, 'r': KeyR, 's': KeyS, 't': KeyT,
	'u': KeyU, 'v': KeyV, 'w': KeyW, 'x': KeyX, 'y': KeyY,
	'z': KeyZ,
	'1': Key1, '2': Key2, '3': Key3, '4': Key4, '5': Key5,
	'6': Key6, '7': Key7, '8': Key8, '9': Key9, '0': Key0,
	'-':  KeyMinus,
	'=':  KeyEqual,
	'[':  KeyLeftBrace,
	']':  KeyRightBrace,
	'\\': KeyBackslash,
	';':  KeySemicolon,
	'\'': KeyApostrophe,
	'`':  KeyGrave,
	',':  KeyComma,
	'.':  KeyDot,
	'/':  KeySlash,
	' ':  KeySpace,
	'\t': KeyTab,
	'\n': KeyEnter,
}

// shiftedCharKeyMap maps shifted characters to their key codes.
var shiftedCharKeyMap = map[rune]KeyCode{
	'!': Key1, '@': Key2, '#': Key3, '$': Key4, '%': Key5,
	'^': Key6, '&': Key7, '*': Key8, '(': Key9, ')': Key0,
	'_': KeyMinus,
	'+': KeyEqual,
	'{': KeyLeftBrace,
	'}': KeyRightBrace,
	'|': KeyBackslash,
	':': KeySemicolon,
	'"': KeyApostrophe,
	'~': KeyGrave,
	'<': KeyComma,
	'>': KeyDot,
	'?': KeySlash,
}
