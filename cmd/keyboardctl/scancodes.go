// Package main provides the keyboardctl CLI for injecting keyboard events
// into Windows via the KeyboardSimulator kernel driver.
package main

// ScanCode is a PC/AT keyboard scan code (Set 1 / XT format).
//
// Most keys carry a plain 8-bit make code.  Extended keys (arrows, Insert,
// Delete, Home, End, etc.) additionally require the KEY_E0 flag in the
// KEYBOARD_INPUT_DATA.Flags field; those keys are encoded here with the
// high bit set (ScanCodeExtended) so the driver layer can detect them.
//
//	MakeCode = uint16(sc &^ ScanCodeExtended)
//	Flags    |= KEY_E0  if sc & ScanCodeExtended != 0
type ScanCode uint16

// ScanCodeExtended marks a ScanCode that needs the KEY_E0 flag when sent to
// the driver.  Strip this bit before using the value as a MakeCode.
const ScanCodeExtended ScanCode = 0x8000

// Standard US-keyboard scan codes (Set 1 / XT make codes).
const (
	ScancodeEsc        ScanCode = 0x01
	Scancode1          ScanCode = 0x02
	Scancode2          ScanCode = 0x03
	Scancode3          ScanCode = 0x04
	Scancode4          ScanCode = 0x05
	Scancode5          ScanCode = 0x06
	Scancode6          ScanCode = 0x07
	Scancode7          ScanCode = 0x08
	Scancode8          ScanCode = 0x09
	Scancode9          ScanCode = 0x0A
	Scancode0          ScanCode = 0x0B
	ScancodeMinus      ScanCode = 0x0C
	ScancodeEqual      ScanCode = 0x0D
	ScancodeBackspace  ScanCode = 0x0E
	ScancodeTab        ScanCode = 0x0F
	ScancodeQ          ScanCode = 0x10
	ScancodeW          ScanCode = 0x11
	ScancodeE          ScanCode = 0x12
	ScancodeR          ScanCode = 0x13
	ScancodeT          ScanCode = 0x14
	ScancodeY          ScanCode = 0x15
	ScancodeU          ScanCode = 0x16
	ScancodeI          ScanCode = 0x17
	ScancodeO          ScanCode = 0x18
	ScancodeP          ScanCode = 0x19
	ScancodeLeftBrace  ScanCode = 0x1A
	ScancodeRightBrace ScanCode = 0x1B
	ScancodeEnter      ScanCode = 0x1C
	ScancodeLCtrl      ScanCode = 0x1D
	ScancodeA          ScanCode = 0x1E
	ScancodeS          ScanCode = 0x1F
	ScancodeD          ScanCode = 0x20
	ScancodeF          ScanCode = 0x21
	ScancodeG          ScanCode = 0x22
	ScancodeH          ScanCode = 0x23
	ScancodeJ          ScanCode = 0x24
	ScancodeK          ScanCode = 0x25
	ScancodeL          ScanCode = 0x26
	ScancodeSemicolon  ScanCode = 0x27
	ScancodeApostrophe ScanCode = 0x28
	ScancodeGrave      ScanCode = 0x29
	ScancodeLShift     ScanCode = 0x2A
	ScancodeBackslash  ScanCode = 0x2B
	ScancodeZ          ScanCode = 0x2C
	ScancodeX          ScanCode = 0x2D
	ScancodeC          ScanCode = 0x2E
	ScancodeV          ScanCode = 0x2F
	ScancodeB          ScanCode = 0x30
	ScancodeN          ScanCode = 0x31
	ScancodeM          ScanCode = 0x32
	ScancodeComma      ScanCode = 0x33
	ScancodeDot        ScanCode = 0x34
	ScancodeSlash      ScanCode = 0x35
	ScancodeRShift     ScanCode = 0x36
	ScancodeKpStar     ScanCode = 0x37
	ScancodeLAlt       ScanCode = 0x38
	ScancodeSpace      ScanCode = 0x39
	ScancodeCapsLock   ScanCode = 0x3A
	ScancodeF1         ScanCode = 0x3B
	ScancodeF2         ScanCode = 0x3C
	ScancodeF3         ScanCode = 0x3D
	ScancodeF4         ScanCode = 0x3E
	ScancodeF5         ScanCode = 0x3F
	ScancodeF6         ScanCode = 0x40
	ScancodeF7         ScanCode = 0x41
	ScancodeF8         ScanCode = 0x42
	ScancodeF9         ScanCode = 0x43
	ScancodeF10        ScanCode = 0x44
	ScancodeNumLock    ScanCode = 0x45
	ScancodeScrollLock ScanCode = 0x46
	ScancodeKp7        ScanCode = 0x47
	ScancodeKp8        ScanCode = 0x48
	ScancodeKp9        ScanCode = 0x49
	ScancodeKpMinus    ScanCode = 0x4A
	ScancodeKp4        ScanCode = 0x4B
	ScancodeKp5        ScanCode = 0x4C
	ScancodeKp6        ScanCode = 0x4D
	ScancodeKpPlus     ScanCode = 0x4E
	ScancodeKp1        ScanCode = 0x4F
	ScancodeKp2        ScanCode = 0x50
	ScancodeKp3        ScanCode = 0x51
	ScancodeKp0        ScanCode = 0x52
	ScancodeKpDot      ScanCode = 0x53
	ScancodeF11        ScanCode = 0x57
	ScancodeF12        ScanCode = 0x58

	/* Extended keys — require KEY_E0 flag (ScanCodeExtended bit set) */
	ScancodeRCtrl      ScanCode = ScanCodeExtended | 0x1D
	ScancodeKpEnter    ScanCode = ScanCodeExtended | 0x1C
	ScancodeKpSlash    ScanCode = ScanCodeExtended | 0x35
	ScancodeRAlt       ScanCode = ScanCodeExtended | 0x38
	ScancodeHome       ScanCode = ScanCodeExtended | 0x47
	ScancodeUp         ScanCode = ScanCodeExtended | 0x48
	ScancodePageUp     ScanCode = ScanCodeExtended | 0x49
	ScancodeLeft       ScanCode = ScanCodeExtended | 0x4B
	ScancodeRight      ScanCode = ScanCodeExtended | 0x4D
	ScancodeEnd        ScanCode = ScanCodeExtended | 0x4F
	ScancodeDown       ScanCode = ScanCodeExtended | 0x50
	ScancodePageDown   ScanCode = ScanCodeExtended | 0x51
	ScancodeInsert     ScanCode = ScanCodeExtended | 0x52
	ScancodeDelete     ScanCode = ScanCodeExtended | 0x53
	ScancodeLWin       ScanCode = ScanCodeExtended | 0x5B
	ScancodeRWin       ScanCode = ScanCodeExtended | 0x5C
	ScancodeApps       ScanCode = ScanCodeExtended | 0x5D
	ScancodePrintScr   ScanCode = ScanCodeExtended | 0x37
)

// keyNameMap maps case-folded human-readable key names to their scan codes.
// Used by ParseScanCode and the CLI argument parser.
var keyNameMap = map[string]ScanCode{
	"esc":        ScancodeEsc,
	"escape":     ScancodeEsc,
	"1":          Scancode1,
	"2":          Scancode2,
	"3":          Scancode3,
	"4":          Scancode4,
	"5":          Scancode5,
	"6":          Scancode6,
	"7":          Scancode7,
	"8":          Scancode8,
	"9":          Scancode9,
	"0":          Scancode0,
	"minus":      ScancodeMinus,
	"equal":      ScancodeEqual,
	"backspace":  ScancodeBackspace,
	"tab":        ScancodeTab,
	"q":          ScancodeQ,
	"w":          ScancodeW,
	"e":          ScancodeE,
	"r":          ScancodeR,
	"t":          ScancodeT,
	"y":          ScancodeY,
	"u":          ScancodeU,
	"i":          ScancodeI,
	"o":          ScancodeO,
	"p":          ScancodeP,
	"leftbrace":  ScancodeLeftBrace,
	"rightbrace": ScancodeRightBrace,
	"enter":      ScancodeEnter,
	"return":     ScancodeEnter,
	"lctrl":      ScancodeLCtrl,
	"ctrl":       ScancodeLCtrl,
	"leftctrl":   ScancodeLCtrl,
	"a":          ScancodeA,
	"s":          ScancodeS,
	"d":          ScancodeD,
	"f":          ScancodeF,
	"g":          ScancodeG,
	"h":          ScancodeH,
	"j":          ScancodeJ,
	"k":          ScancodeK,
	"l":          ScancodeL,
	"semicolon":  ScancodeSemicolon,
	"apostrophe": ScancodeApostrophe,
	"grave":      ScancodeGrave,
	"lshift":     ScancodeLShift,
	"shift":      ScancodeLShift,
	"leftshift":  ScancodeLShift,
	"backslash":  ScancodeBackslash,
	"z":          ScancodeZ,
	"x":          ScancodeX,
	"c":          ScancodeC,
	"v":          ScancodeV,
	"b":          ScancodeB,
	"n":          ScancodeN,
	"m":          ScancodeM,
	"comma":      ScancodeComma,
	"dot":        ScancodeDot,
	"period":     ScancodeDot,
	"slash":      ScancodeSlash,
	"rshift":     ScancodeRShift,
	"rightshift": ScancodeRShift,
	"lalt":       ScancodeLAlt,
	"alt":        ScancodeLAlt,
	"leftalt":    ScancodeLAlt,
	"space":      ScancodeSpace,
	"capslock":   ScancodeCapsLock,
	"f1":         ScancodeF1,
	"f2":         ScancodeF2,
	"f3":         ScancodeF3,
	"f4":         ScancodeF4,
	"f5":         ScancodeF5,
	"f6":         ScancodeF6,
	"f7":         ScancodeF7,
	"f8":         ScancodeF8,
	"f9":         ScancodeF9,
	"f10":        ScancodeF10,
	"f11":        ScancodeF11,
	"f12":        ScancodeF12,
	"numlock":    ScancodeNumLock,
	"scrolllock": ScancodeScrollLock,
	"kpenter":    ScancodeKpEnter,
	"rctrl":      ScancodeRCtrl,
	"rightctrl":  ScancodeRCtrl,
	"ralt":       ScancodeRAlt,
	"rightalt":   ScancodeRAlt,
	"altgr":      ScancodeRAlt,
	"home":       ScancodeHome,
	"up":         ScancodeUp,
	"pageup":     ScancodePageUp,
	"left":       ScancodeLeft,
	"right":      ScancodeRight,
	"end":        ScancodeEnd,
	"down":       ScancodeDown,
	"pagedown":   ScancodePageDown,
	"insert":     ScancodeInsert,
	"delete":     ScancodeDelete,
	"del":        ScancodeDelete,
	"lwin":       ScancodeLWin,
	"rwin":       ScancodeRWin,
	"win":        ScancodeLWin,
	"super":      ScancodeLWin,
	"apps":       ScancodeApps,
	"menu":       ScancodeApps,
	"print":      ScancodePrintScr,
	"printscr":   ScancodePrintScr,
}

// charScanMap maps ASCII characters to their (ScanCode, needsShift) pairs for
// the US keyboard layout.  Upper-case letters are handled separately in
// TypeText: they use the lower-case scan code with Shift held.
var charScanMap = map[rune]ScanCode{
	'1': Scancode1, '2': Scancode2, '3': Scancode3, '4': Scancode4, '5': Scancode5,
	'6': Scancode6, '7': Scancode7, '8': Scancode8, '9': Scancode9, '0': Scancode0,
	'-': ScancodeMinus, '=': ScancodeEqual,
	'\t': ScancodeTab, '\n': ScancodeEnter, ' ': ScancodeSpace,
	'q': ScancodeQ, 'w': ScancodeW, 'e': ScancodeE, 'r': ScancodeR, 't': ScancodeT,
	'y': ScancodeY, 'u': ScancodeU, 'i': ScancodeI, 'o': ScancodeO, 'p': ScancodeP,
	'[': ScancodeLeftBrace, ']': ScancodeRightBrace,
	'a': ScancodeA, 's': ScancodeS, 'd': ScancodeD, 'f': ScancodeF, 'g': ScancodeG,
	'h': ScancodeH, 'j': ScancodeJ, 'k': ScancodeK, 'l': ScancodeL,
	';': ScancodeSemicolon, '\'': ScancodeApostrophe, '`': ScancodeGrave,
	'\\': ScancodeBackslash,
	'z': ScancodeZ, 'x': ScancodeX, 'c': ScancodeC, 'v': ScancodeV, 'b': ScancodeB,
	'n': ScancodeN, 'm': ScancodeM,
	',': ScancodeComma, '.': ScancodeDot, '/': ScancodeSlash,
}

// shiftedCharScanMap maps characters that require Shift to their base scan codes.
var shiftedCharScanMap = map[rune]ScanCode{
	'!': Scancode1, '@': Scancode2, '#': Scancode3, '$': Scancode4, '%': Scancode5,
	'^': Scancode6, '&': Scancode7, '*': Scancode8, '(': Scancode9, ')': Scancode0,
	'_': ScancodeMinus, '+': ScancodeEqual,
	'{': ScancodeLeftBrace, '}': ScancodeRightBrace,
	'|': ScancodeBackslash,
	':': ScancodeSemicolon, '"': ScancodeApostrophe, '~': ScancodeGrave,
	'<': ScancodeComma, '>': ScancodeDot, '?': ScancodeSlash,
}

// ParseScanCode returns the ScanCode for the given case-insensitive key name.
// Returns (0, false) if the name is not recognised.
func ParseScanCode(name string) (ScanCode, bool) {
	sc, ok := keyNameMap[asciiToLower(name)]
	return sc, ok
}

// RuneToScanCode maps a Unicode rune to its (ScanCode, needsShift) pair using
// the standard US keyboard layout.  Returns (0, false, false) if unmappable.
func RuneToScanCode(r rune) (sc ScanCode, shift bool, ok bool) {
	if s, found := charScanMap[r]; found {
		return s, false, true
	}
	// Upper-case letters: use lower-case scan code + Shift
	if r >= 'A' && r <= 'Z' {
		lower := r + ('a' - 'A')
		if s, found := charScanMap[lower]; found {
			return s, true, true
		}
	}
	if s, found := shiftedCharScanMap[r]; found {
		return s, true, true
	}
	return 0, false, false
}

// asciiToLower folds ASCII uppercase letters to lowercase without importing
// the strings package.
func asciiToLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}
