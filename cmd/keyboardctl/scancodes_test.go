package main

import (
	"testing"
)

// TestScanCodeExtendedBit verifies that the extended-flag sentinel does not
// overlap with any real scan code value.
func TestScanCodeExtendedBit(t *testing.T) {
	if ScanCodeExtended != 0x8000 {
		t.Errorf("ScanCodeExtended = 0x%04X, want 0x8000", ScanCodeExtended)
	}
	// Ensure no plain (non-extended) scan code has the high bit set
	for name, sc := range keyNameMap {
		if sc&ScanCodeExtended == 0 {
			if sc > 0x7F {
				t.Errorf("plain scan code %q = 0x%04X exceeds 0x7F",
					name, sc)
			}
		}
	}
}

// TestParseKeyCode verifies case-insensitive name lookup.
func TestParseKeyCode(t *testing.T) {
	cases := []struct {
		input string
		want  ScanCode
	}{
		{"a", ScancodeA},
		{"A", ScancodeA},
		{"enter", ScancodeEnter},
		{"ENTER", ScancodeEnter},
		{"Enter", ScancodeEnter},
		{"return", ScancodeEnter},
		{"space", ScancodeSpace},
		{"ctrl", ScancodeLCtrl},
		{"lctrl", ScancodeLCtrl},
		{"leftctrl", ScancodeLCtrl},
		{"shift", ScancodeLShift},
		{"lshift", ScancodeLShift},
		{"leftshift", ScancodeLShift},
		{"alt", ScancodeLAlt},
		{"lalt", ScancodeLAlt},
		{"up", ScancodeUp},
		{"down", ScancodeDown},
		{"left", ScancodeLeft},
		{"right", ScancodeRight},
		{"f1", ScancodeF1},
		{"F1", ScancodeF1},
		{"f12", ScancodeF12},
		{"delete", ScancodeDelete},
		{"del", ScancodeDelete},
		{"win", ScancodeLWin},
		{"super", ScancodeLWin},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, ok := ParseScanCode(tc.input)
			if !ok {
				t.Fatalf("ParseScanCode(%q) = _, false", tc.input)
			}
			if got != tc.want {
				t.Errorf("ParseScanCode(%q) = 0x%04X, want 0x%04X",
					tc.input, got, tc.want)
			}
		})
	}
}

// TestParseKeyCode_unknown verifies that unknown names return (0, false).
func TestParseKeyCode_unknown(t *testing.T) {
	if _, ok := ParseScanCode("notakey"); ok {
		t.Error("ParseScanCode(\"notakey\") returned ok=true")
	}
	if _, ok := ParseScanCode(""); ok {
		t.Error("ParseScanCode(\"\") returned ok=true")
	}
}

// TestExtendedKeys verifies that arrow keys and navigation keys are marked
// as extended (require KEY_E0).
func TestExtendedKeys(t *testing.T) {
	extended := []struct {
		name string
		sc   ScanCode
	}{
		{"up", ScancodeUp},
		{"down", ScancodeDown},
		{"left", ScancodeLeft},
		{"right", ScancodeRight},
		{"home", ScancodeHome},
		{"end", ScancodeEnd},
		{"insert", ScancodeInsert},
		{"delete", ScancodeDelete},
		{"pageup", ScancodePageUp},
		{"pagedown", ScancodePageDown},
		{"rctrl", ScancodeRCtrl},
		{"ralt", ScancodeRAlt},
		{"lwin", ScancodeLWin},
	}
	for _, tc := range extended {
		t.Run(tc.name, func(t *testing.T) {
			if tc.sc&ScanCodeExtended == 0 {
				t.Errorf("%s (0x%04X) should have ScanCodeExtended bit set",
					tc.name, tc.sc)
			}
		})
	}
}

// TestNonExtendedKeys verifies that standard typing keys are NOT extended.
func TestNonExtendedKeys(t *testing.T) {
	plain := []struct {
		name string
		sc   ScanCode
	}{
		{"a", ScancodeA},
		{"enter", ScancodeEnter},
		{"space", ScancodeSpace},
		{"lctrl", ScancodeLCtrl},
		{"lshift", ScancodeLShift},
		{"lalt", ScancodeLAlt},
		{"f1", ScancodeF1},
		{"f12", ScancodeF12},
	}
	for _, tc := range plain {
		t.Run(tc.name, func(t *testing.T) {
			if tc.sc&ScanCodeExtended != 0 {
				t.Errorf("%s (0x%04X) should NOT have ScanCodeExtended bit set",
					tc.name, tc.sc)
			}
		})
	}
}

// TestRuneToScanCode_lowercase verifies that lower-case letters map to their
// scan code without Shift.
func TestRuneToScanCode_lowercase(t *testing.T) {
	cases := []struct {
		r  rune
		sc ScanCode
	}{
		{'a', ScancodeA}, {'s', ScancodeS}, {'z', ScancodeZ},
		{'1', Scancode1}, {'0', Scancode0},
		{' ', ScancodeSpace}, {'\n', ScancodeEnter},
	}
	for _, tc := range cases {
		sc, shift, ok := RuneToScanCode(tc.r)
		if !ok {
			t.Errorf("RuneToScanCode(%q) = _, _, false", tc.r)
			continue
		}
		if shift {
			t.Errorf("RuneToScanCode(%q) shift=true, want false", tc.r)
		}
		if sc != tc.sc {
			t.Errorf("RuneToScanCode(%q) = 0x%04X, want 0x%04X", tc.r, sc, tc.sc)
		}
	}
}

// TestRuneToScanCode_uppercase verifies that upper-case letters return the
// lower-case scan code with needsShift=true.
func TestRuneToScanCode_uppercase(t *testing.T) {
	cases := []struct {
		r  rune
		sc ScanCode
	}{
		{'A', ScancodeA}, {'Z', ScancodeZ}, {'M', ScancodeM},
	}
	for _, tc := range cases {
		sc, shift, ok := RuneToScanCode(tc.r)
		if !ok {
			t.Errorf("RuneToScanCode(%q) = _, _, false", tc.r)
			continue
		}
		if !shift {
			t.Errorf("RuneToScanCode(%q) shift=false, want true", tc.r)
		}
		if sc != tc.sc {
			t.Errorf("RuneToScanCode(%q) = 0x%04X, want 0x%04X", tc.r, sc, tc.sc)
		}
	}
}

// TestRuneToScanCode_shiftedSymbols verifies that symbols like '!' require Shift.
func TestRuneToScanCode_shiftedSymbols(t *testing.T) {
	cases := []struct {
		r  rune
		sc ScanCode
	}{
		{'!', Scancode1},  // Shift+1
		{'@', Scancode2},  // Shift+2
		{'_', ScancodeMinus},  // Shift+-
		{':', ScancodeSemicolon}, // Shift+;
	}
	for _, tc := range cases {
		sc, shift, ok := RuneToScanCode(tc.r)
		if !ok {
			t.Errorf("RuneToScanCode(%q) = _, _, false", tc.r)
			continue
		}
		if !shift {
			t.Errorf("RuneToScanCode(%q) shift=false, want true", tc.r)
		}
		if sc != tc.sc {
			t.Errorf("RuneToScanCode(%q) = 0x%04X, want 0x%04X", tc.r, sc, tc.sc)
		}
	}
}

// TestRuneToScanCode_unknown verifies that unmappable runes return ok=false.
func TestRuneToScanCode_unknown(t *testing.T) {
	// U+00E9 (é) has no mapping in the US layout scan code table
	_, _, ok := RuneToScanCode('\u00e9')
	if ok {
		t.Error("RuneToScanCode('é') returned ok=true, want false")
	}
}

// TestKnownScancodeValues spot-checks a selection of scan code values against
// the well-known PC/AT Set 1 scan code table.
func TestKnownScancodeValues(t *testing.T) {
	cases := []struct {
		name string
		sc   ScanCode
		want uint16 // plain make code (without extended bit)
	}{
		{"Esc",        ScancodeEsc,        0x01},
		{"1",          Scancode1,          0x02},
		{"0",          Scancode0,          0x0B},
		{"Backspace",  ScancodeBackspace,  0x0E},
		{"Tab",        ScancodeTab,        0x0F},
		{"Q",          ScancodeQ,          0x10},
		{"Enter",      ScancodeEnter,      0x1C},
		{"LCtrl",      ScancodeLCtrl,      0x1D},
		{"A",          ScancodeA,          0x1E},
		{"LShift",     ScancodeLShift,     0x2A},
		{"Z",          ScancodeZ,          0x2C},
		{"Space",      ScancodeSpace,      0x39},
		{"CapsLock",   ScancodeCapsLock,   0x3A},
		{"F1",         ScancodeF1,         0x3B},
		{"F10",        ScancodeF10,        0x44},
		{"F11",        ScancodeF11,        0x57},
		{"F12",        ScancodeF12,        0x58},
		{"Up (ext)",   ScancodeUp,         0x48},
		{"Down (ext)", ScancodeDown,       0x50},
		{"Left (ext)", ScancodeLeft,       0x4B},
		{"Right (ext)", ScancodeRight,      0x4D},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := uint16(tc.sc &^ ScanCodeExtended)
			if got != tc.want {
				t.Errorf("scancode = 0x%02X, want 0x%02X", got, tc.want)
			}
		})
	}
}
