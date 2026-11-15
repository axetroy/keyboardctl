package main

import (
	"strings"
	"testing"
)

func TestResolveKeyArg(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		args  []string
		other []string
		want  string
	}{
		{"bare argument becomes the key", "", []string{"a"}, nil, "a"},
		{"bare hex argument", "", []string{"0x1E"}, nil, "0x1E"},
		{"explicit -key wins", "b", []string{"a"}, nil, "b"},
		{"no argument leaves the key alone", "", nil, nil, ""},
		{"two arguments are not a key", "", []string{"a", "b"}, nil, ""},
		{"other action flag blocks the shorthand", "", []string{"a"}, []string{"", "", "", "text"}, ""},
		{"empty other flags still allow the shorthand", "", []string{"a"}, []string{"", "", ""}, "a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveKeyArg(tt.key, tt.args, tt.other...); got != tt.want {
				t.Fatalf("resolveKeyArg(%q, %v, %v) = %q, want %q", tt.key, tt.args, tt.other, got, tt.want)
			}
		})
	}
}

func TestCountNonEmpty(t *testing.T) {
	if got := countNonEmpty("", "", ""); got != 0 {
		t.Fatalf("all empty = %d, want 0", got)
	}
	if got := countNonEmpty("a", "", "c"); got != 2 {
		t.Fatalf("mixed = %d, want 2", got)
	}
}

func TestTryParseScanCode(t *testing.T) {
	tests := []struct {
		in    string
		want  ScanCode
		valid bool
	}{
		{"0x1E", ScancodeA, true},
		{"0x1e", ScancodeA, true},
		{"a", ScancodeA, true},
		{"ctrl", ScancodeLCtrl, true},
		{"ENTER", ScancodeEnter, true},
		{"0xZZ", 0, false},
		{"notakey", 0, false},
		{"", 0, false},
	}
	for _, tt := range tests {
		got, ok := tryParseScanCode(tt.in)
		if ok != tt.valid || (ok && got != tt.want) {
			t.Errorf("tryParseScanCode(%q) = (0x%04X, %v), want (0x%04X, %v)",
				tt.in, got, ok, tt.want, tt.valid)
		}
	}
}

func TestScanCodeError(t *testing.T) {
	if got := scanCodeError("0xZZ"); !strings.Contains(got, "invalid scan code") {
		t.Errorf("scanCodeError(0xZZ) = %q, want it to mention invalid scan code", got)
	}
	if got := scanCodeError("notakey"); !strings.Contains(got, "unknown key name") {
		t.Errorf("scanCodeError(notakey) = %q, want it to mention unknown key name", got)
	}
}

func TestIsModifier(t *testing.T) {
	mods := []string{"lctrl", "rctrl", "lshift", "rshift", "lalt", "ralt", "lwin", "rwin"}
	for _, m := range mods {
		sc, ok := tryParseScanCode(m)
		if !ok {
			t.Fatalf("%q did not parse", m)
		}
		if !isModifier(sc) {
			t.Errorf("isModifier(%q) = false, want true", m)
		}
	}
	for _, k := range []string{"a", "enter", "f1", "delete", "capslock"} {
		sc, _ := tryParseScanCode(k)
		if isModifier(sc) {
			t.Errorf("isModifier(%q) = true, want false", k)
		}
	}
}

func TestParseCombo(t *testing.T) {
	tests := []struct {
		name     string
		parts    []string
		wantMods []ScanCode
		wantKey  ScanCode
		wantErr  bool
	}{
		{"single modifier", []string{"ctrl", "c"}, []ScanCode{ScancodeLCtrl}, ScancodeC, false},
		{"alt f4", []string{"lalt", "f4"}, []ScanCode{ScancodeLAlt}, ScancodeF4, false},
		{"two modifiers", []string{"ctrl", "shift", "a"},
			[]ScanCode{ScancodeLCtrl, ScancodeLShift}, ScancodeA, false},
		{"three modifiers", []string{"ctrl", "alt", "delete", "x"},
			nil, 0, true}, // delete is not a modifier, and it is not last
		{"ctrl alt delete", []string{"ctrl", "alt", "delete"},
			[]ScanCode{ScancodeLCtrl, ScancodeLAlt}, ScancodeDelete, false},
		{"ctrl shift esc", []string{"ctrl", "shift", "esc"},
			[]ScanCode{ScancodeLCtrl, ScancodeLShift}, ScancodeEsc, false},
		{"hex key", []string{"ctrl", "0x1E"}, []ScanCode{ScancodeLCtrl}, ScancodeA, false},
		{"hex modifier", []string{"0x1D", "c"}, []ScanCode{ScancodeLCtrl}, ScancodeC, false},
		{"too few", []string{"ctrl"}, nil, 0, true},
		{"empty", nil, nil, 0, true},
		{"unknown key", []string{"ctrl", "notakey"}, nil, 0, true},
		{"unknown modifier", []string{"notakey", "c"}, nil, 0, true},
		// Regression: non-final non-modifier used to be silently dropped, so
		// "ctrl shift a" sent Ctrl+Shift and "ctrl alt delete" sent Ctrl+Alt.
		{"key not last is an error", []string{"ctrl", "c", "shift"}, nil, 0, true},
		{"second key is an error", []string{"ctrl", "a", "b"}, nil, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mods, key, err := parseCombo(tt.parts)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseCombo(%v) = (%v, 0x%04X, nil), want error", tt.parts, mods, key)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseCombo(%v) unexpected error: %v", tt.parts, err)
			}
			if key != tt.wantKey {
				t.Errorf("parseCombo(%v) key = 0x%04X, want 0x%04X", tt.parts, key, tt.wantKey)
			}
			if len(mods) != len(tt.wantMods) {
				t.Fatalf("parseCombo(%v) mods = %v, want %v", tt.parts, mods, tt.wantMods)
			}
			for i := range mods {
				if mods[i] != tt.wantMods[i] {
					t.Errorf("parseCombo(%v) mods[%d] = 0x%04X, want 0x%04X",
						tt.parts, i, mods[i], tt.wantMods[i])
				}
			}
		})
	}
}
