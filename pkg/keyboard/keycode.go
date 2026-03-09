//go:build linux

package keyboard

// KeyCode represents a Linux input event key code.
// Values match the constants defined in linux/input-event-codes.h and are
// stable across kernel versions as part of the Linux UAPI.
type KeyCode uint16

// Key code constants — standard US keyboard layout.
// Source: linux/input-event-codes.h (UAPI, stable ABI)
const (
	KeyReserved   KeyCode = 0
	KeyEsc        KeyCode = 1
	Key1          KeyCode = 2
	Key2          KeyCode = 3
	Key3          KeyCode = 4
	Key4          KeyCode = 5
	Key5          KeyCode = 6
	Key6          KeyCode = 7
	Key7          KeyCode = 8
	Key8          KeyCode = 9
	Key9          KeyCode = 10
	Key0          KeyCode = 11
	KeyMinus      KeyCode = 12
	KeyEqual      KeyCode = 13
	KeyBackspace  KeyCode = 14
	KeyTab        KeyCode = 15
	KeyQ          KeyCode = 16
	KeyW          KeyCode = 17
	KeyE          KeyCode = 18
	KeyR          KeyCode = 19
	KeyT          KeyCode = 20
	KeyY          KeyCode = 21
	KeyU          KeyCode = 22
	KeyI          KeyCode = 23
	KeyO          KeyCode = 24
	KeyP          KeyCode = 25
	KeyLeftBrace  KeyCode = 26
	KeyRightBrace KeyCode = 27
	KeyEnter      KeyCode = 28
	KeyLeftCtrl   KeyCode = 29
	KeyA          KeyCode = 30
	KeyS          KeyCode = 31
	KeyD          KeyCode = 32
	KeyF          KeyCode = 33
	KeyG          KeyCode = 34
	KeyH          KeyCode = 35
	KeyJ          KeyCode = 36
	KeyK          KeyCode = 37
	KeyL          KeyCode = 38
	KeySemicolon  KeyCode = 39
	KeyApostrophe KeyCode = 40
	KeyGrave      KeyCode = 41
	KeyLeftShift  KeyCode = 42
	KeyBackslash  KeyCode = 43
	KeyZ          KeyCode = 44
	KeyX          KeyCode = 45
	KeyC          KeyCode = 46
	KeyV          KeyCode = 47
	KeyB          KeyCode = 48
	KeyN          KeyCode = 49
	KeyM          KeyCode = 50
	KeyComma      KeyCode = 51
	KeyDot        KeyCode = 52
	KeySlash      KeyCode = 53
	KeyRightShift KeyCode = 54
	KeyKpAsterisk KeyCode = 55
	KeyLeftAlt    KeyCode = 56
	KeySpace      KeyCode = 57
	KeyCapsLock   KeyCode = 58
	KeyF1         KeyCode = 59
	KeyF2         KeyCode = 60
	KeyF3         KeyCode = 61
	KeyF4         KeyCode = 62
	KeyF5         KeyCode = 63
	KeyF6         KeyCode = 64
	KeyF7         KeyCode = 65
	KeyF8         KeyCode = 66
	KeyF9         KeyCode = 67
	KeyF10        KeyCode = 68
	KeyNumLock    KeyCode = 69
	KeyScrollLock KeyCode = 70
	KeyKp7        KeyCode = 71
	KeyKp8        KeyCode = 72
	KeyKp9        KeyCode = 73
	KeyKpMinus    KeyCode = 74
	KeyKp4        KeyCode = 75
	KeyKp5        KeyCode = 76
	KeyKp6        KeyCode = 77
	KeyKpPlus     KeyCode = 78
	KeyKp1        KeyCode = 79
	KeyKp2        KeyCode = 80
	KeyKp3        KeyCode = 81
	KeyKp0        KeyCode = 82
	KeyKpDot      KeyCode = 83
	KeyF11        KeyCode = 87
	KeyF12        KeyCode = 88
	KeyKpEnter    KeyCode = 96
	KeyRightCtrl  KeyCode = 97
	KeyKpSlash    KeyCode = 98
	KeySysRq      KeyCode = 99
	KeyRightAlt   KeyCode = 100
	KeyHome       KeyCode = 102
	KeyUp         KeyCode = 103
	KeyPageUp     KeyCode = 104
	KeyLeft       KeyCode = 105
	KeyRight      KeyCode = 106
	KeyEnd        KeyCode = 107
	KeyDown       KeyCode = 108
	KeyPageDown   KeyCode = 109
	KeyInsert     KeyCode = 110
	KeyDelete     KeyCode = 111
	KeyMute       KeyCode = 113
	KeyVolumeDown KeyCode = 114
	KeyVolumeUp   KeyCode = 115
	KeyPower      KeyCode = 116
	KeyKpEqual    KeyCode = 117
	KeyPause      KeyCode = 119
	KeyLeftMeta   KeyCode = 125
	KeyRightMeta  KeyCode = 126
	KeyCompose    KeyCode = 127
	KeyPrint      KeyCode = 210
)

// keyNames maps human-readable names (used by the CLI) to key codes.
// Names are case-insensitive after lowercasing in the CLI.
var keyNames = map[string]KeyCode{
	"esc":        KeyEsc,
	"escape":     KeyEsc,
	"f1":         KeyF1,
	"f2":         KeyF2,
	"f3":         KeyF3,
	"f4":         KeyF4,
	"f5":         KeyF5,
	"f6":         KeyF6,
	"f7":         KeyF7,
	"f8":         KeyF8,
	"f9":         KeyF9,
	"f10":        KeyF10,
	"f11":        KeyF11,
	"f12":        KeyF12,
	"1":          Key1,
	"2":          Key2,
	"3":          Key3,
	"4":          Key4,
	"5":          Key5,
	"6":          Key6,
	"7":          Key7,
	"8":          Key8,
	"9":          Key9,
	"0":          Key0,
	"minus":      KeyMinus,
	"equal":      KeyEqual,
	"backspace":  KeyBackspace,
	"tab":        KeyTab,
	"q":          KeyQ,
	"w":          KeyW,
	"e":          KeyE,
	"r":          KeyR,
	"t":          KeyT,
	"y":          KeyY,
	"u":          KeyU,
	"i":          KeyI,
	"o":          KeyO,
	"p":          KeyP,
	"leftbrace":  KeyLeftBrace,
	"rightbrace": KeyRightBrace,
	"enter":      KeyEnter,
	"return":     KeyEnter,
	"leftctrl":   KeyLeftCtrl,
	"ctrl":       KeyLeftCtrl,
	"a":          KeyA,
	"s":          KeyS,
	"d":          KeyD,
	"f":          KeyF,
	"g":          KeyG,
	"h":          KeyH,
	"j":          KeyJ,
	"k":          KeyK,
	"l":          KeyL,
	"semicolon":  KeySemicolon,
	"apostrophe": KeyApostrophe,
	"grave":      KeyGrave,
	"leftshift":  KeyLeftShift,
	"shift":      KeyLeftShift,
	"backslash":  KeyBackslash,
	"z":          KeyZ,
	"x":          KeyX,
	"c":          KeyC,
	"v":          KeyV,
	"b":          KeyB,
	"n":          KeyN,
	"m":          KeyM,
	"comma":      KeyComma,
	"dot":        KeyDot,
	"period":     KeyDot,
	"slash":      KeySlash,
	"rightshift": KeyRightShift,
	"leftalt":    KeyLeftAlt,
	"alt":        KeyLeftAlt,
	"space":      KeySpace,
	"capslock":   KeyCapsLock,
	"numlock":    KeyNumLock,
	"scrolllock": KeyScrollLock,
	"kpenter":    KeyKpEnter,
	"rightctrl":  KeyRightCtrl,
	"rightalt":   KeyRightAlt,
	"altgr":      KeyRightAlt,
	"home":       KeyHome,
	"up":         KeyUp,
	"pageup":     KeyPageUp,
	"left":       KeyLeft,
	"right":      KeyRight,
	"end":        KeyEnd,
	"down":       KeyDown,
	"pagedown":   KeyPageDown,
	"insert":     KeyInsert,
	"delete":     KeyDelete,
	"del":        KeyDelete,
	"mute":       KeyMute,
	"volumedown": KeyVolumeDown,
	"volumeup":   KeyVolumeUp,
	"power":      KeyPower,
	"pause":      KeyPause,
	"leftmeta":   KeyLeftMeta,
	"rightmeta":  KeyRightMeta,
	"super":      KeyLeftMeta,
	"compose":    KeyCompose,
	"print":      KeyPrint,
	"sysrq":      KeySysRq,
}

// ParseKeyCode parses a human-readable key name into a KeyCode.
// Names are case-insensitive (e.g. "Enter", "ENTER", and "enter" all work).
// Returns 0 and false if the name is not recognised.
func ParseKeyCode(name string) (KeyCode, bool) {
	lower := toLower(name)
	kc, ok := keyNames[lower]
	return kc, ok
}

// toLower is a pure-ASCII lowercase helper that avoids importing strings.
func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}
