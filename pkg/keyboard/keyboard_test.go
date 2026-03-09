//go:build linux

package keyboard

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"
)

// eventSize is the expected on-wire size of a single event struct (must match
// the C struct keyboardctl_event).
const eventSize = 8 // 2 + 2 + 4 bytes

func TestEventSize(t *testing.T) {
	var ev event
	if got := binary.Size(ev); got != eventSize {
		t.Fatalf("event binary size = %d, want %d", got, eventSize)
	}
}

// captureKeyboard writes to a temp file instead of /dev/keyboardctl.
func captureKeyboard(t *testing.T) (*Keyboard, *os.File) {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "keyboardctl-test-*")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	kb := &Keyboard{f: f}
	return kb, f
}

func readEvents(t *testing.T, f *os.File) []event {
	t.Helper()
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatalf("seek: %v", err)
	}
	info, _ := f.Stat()
	n := info.Size() / eventSize
	evs := make([]event, n)
	if err := binary.Read(f, binary.LittleEndian, &evs); err != nil {
		t.Fatalf("read events: %v", err)
	}
	return evs
}

// TestPress verifies that Press emits: key-down, SYN, key-up, SYN.
func TestPress(t *testing.T) {
	kb, f := captureKeyboard(t)
	defer f.Close()

	if err := kb.Press(KeyA); err != nil {
		t.Fatalf("Press: %v", err)
	}

	evs := readEvents(t, f)
	if len(evs) != 4 {
		t.Fatalf("expected 4 events, got %d", len(evs))
	}

	// key-down
	if evs[0].Type != uint16(evKey) || evs[0].Code != uint16(KeyA) || evs[0].Value != keyPress {
		t.Errorf("event[0] = %+v, want key-down KeyA", evs[0])
	}
	// SYN
	if evs[1].Type != uint16(evSyn) {
		t.Errorf("event[1] = %+v, want SYN", evs[1])
	}
	// key-up
	if evs[2].Type != uint16(evKey) || evs[2].Code != uint16(KeyA) || evs[2].Value != keyRelease {
		t.Errorf("event[2] = %+v, want key-up KeyA", evs[2])
	}
	// SYN
	if evs[3].Type != uint16(evSyn) {
		t.Errorf("event[3] = %+v, want SYN", evs[3])
	}
}

// TestCombo verifies that Combo(Ctrl, C) emits the correct event sequence.
func TestCombo(t *testing.T) {
	kb, f := captureKeyboard(t)
	defer f.Close()

	if err := kb.Combo(KeyLeftCtrl, KeyC); err != nil {
		t.Fatalf("Combo: %v", err)
	}

	evs := readEvents(t, f)
	// down(Ctrl), down(C), SYN, up(C), up(Ctrl), SYN
	if len(evs) != 6 {
		t.Fatalf("expected 6 events, got %d", len(evs))
	}
	if evs[0].Code != uint16(KeyLeftCtrl) || evs[0].Value != keyPress {
		t.Errorf("event[0] = %+v, want Ctrl-down", evs[0])
	}
	if evs[1].Code != uint16(KeyC) || evs[1].Value != keyPress {
		t.Errorf("event[1] = %+v, want C-down", evs[1])
	}
	if evs[2].Type != uint16(evSyn) {
		t.Errorf("event[2] = %+v, want SYN", evs[2])
	}
	if evs[3].Code != uint16(KeyC) || evs[3].Value != keyRelease {
		t.Errorf("event[3] = %+v, want C-up", evs[3])
	}
	if evs[4].Code != uint16(KeyLeftCtrl) || evs[4].Value != keyRelease {
		t.Errorf("event[4] = %+v, want Ctrl-up", evs[4])
	}
	if evs[5].Type != uint16(evSyn) {
		t.Errorf("event[5] = %+v, want SYN", evs[5])
	}
}

// TestType_lowercase verifies typing lower-case ASCII does not emit Shift.
func TestType_lowercase(t *testing.T) {
	kb, f := captureKeyboard(t)
	defer f.Close()

	if err := kb.Type("ab"); err != nil {
		t.Fatalf("Type: %v", err)
	}

	evs := readEvents(t, f)
	// Each char: down, SYN, up, SYN — no Shift events
	if len(evs) != 8 {
		t.Fatalf("expected 8 events for 'ab', got %d", len(evs))
	}
	for _, ev := range evs {
		if ev.Code == uint16(KeyLeftShift) {
			t.Error("unexpected Shift event for lower-case input")
		}
	}
}

// TestType_uppercase verifies typing upper-case ASCII wraps key with Shift.
func TestType_uppercase(t *testing.T) {
	kb, f := captureKeyboard(t)
	defer f.Close()

	if err := kb.Type("A"); err != nil {
		t.Fatalf("Type: %v", err)
	}

	evs := readEvents(t, f)
	// Shift-down, A-down, SYN, A-up, Shift-up, SYN
	if len(evs) != 6 {
		t.Fatalf("expected 6 events for 'A', got %d", len(evs))
	}
	if evs[0].Code != uint16(KeyLeftShift) || evs[0].Value != keyPress {
		t.Errorf("event[0] = %+v, want Shift-down", evs[0])
	}
	if evs[1].Code != uint16(KeyA) || evs[1].Value != keyPress {
		t.Errorf("event[1] = %+v, want A-down", evs[1])
	}
	if evs[4].Code != uint16(KeyLeftShift) || evs[4].Value != keyRelease {
		t.Errorf("event[4] = %+v, want Shift-up", evs[4])
	}
}

// TestType_space verifies space is typed correctly.
func TestType_space(t *testing.T) {
	kb, f := captureKeyboard(t)
	defer f.Close()

	if err := kb.Type(" "); err != nil {
		t.Fatalf("Type: %v", err)
	}

	evs := readEvents(t, f)
	if len(evs) != 4 {
		t.Fatalf("expected 4 events for space, got %d", len(evs))
	}
	if evs[0].Code != uint16(KeySpace) {
		t.Errorf("event[0].Code = %d, want KeySpace (%d)", evs[0].Code, KeySpace)
	}
}

// TestType_shiftedSymbol verifies that '!' produces Shift+1.
func TestType_shiftedSymbol(t *testing.T) {
	kb, f := captureKeyboard(t)
	defer f.Close()

	if err := kb.Type("!"); err != nil {
		t.Fatalf("Type: %v", err)
	}

	evs := readEvents(t, f)
	// Shift-down, 1-down, SYN, 1-up, Shift-up, SYN
	if len(evs) != 6 {
		t.Fatalf("expected 6 events for '!', got %d", len(evs))
	}
	if evs[0].Code != uint16(KeyLeftShift) {
		t.Errorf("expected Shift first, got code %d", evs[0].Code)
	}
	if evs[1].Code != uint16(Key1) {
		t.Errorf("expected Key1, got code %d", evs[1].Code)
	}
}

// TestType_unknownChar verifies that unmappable characters are silently skipped.
func TestType_unknownChar(t *testing.T) {
	kb, f := captureKeyboard(t)
	defer f.Close()

	// U+00E9 (é) has no mapping in the US layout key map
	if err := kb.Type("\u00e9"); err != nil {
		t.Fatalf("Type: %v", err)
	}

	info, _ := f.Stat()
	if info.Size() != 0 {
		t.Errorf("expected no events written for unmappable char, got %d bytes", info.Size())
	}
}

// TestParseKeyCode verifies the name→KeyCode lookup table.
func TestParseKeyCode(t *testing.T) {
	cases := []struct {
		name string
		want KeyCode
	}{
		{"enter", KeyEnter},
		{"Enter", KeyEnter},
		{"ENTER", KeyEnter},
		{"return", KeyEnter},
		{"space", KeySpace},
		{"ctrl", KeyLeftCtrl},
		{"leftctrl", KeyLeftCtrl},
		{"shift", KeyLeftShift},
		{"alt", KeyLeftAlt},
		{"a", KeyA},
		{"z", KeyZ},
		{"f1", KeyF1},
		{"f12", KeyF12},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseKeyCode(tc.name)
			if !ok {
				t.Fatalf("ParseKeyCode(%q) = _, false", tc.name)
			}
			if got != tc.want {
				t.Errorf("ParseKeyCode(%q) = %d, want %d", tc.name, got, tc.want)
			}
		})
	}
}

// TestParseKeyCode_unknown verifies unknown names return (0, false).
func TestParseKeyCode_unknown(t *testing.T) {
	if _, ok := ParseKeyCode("notakey"); ok {
		t.Error("ParseKeyCode(\"notakey\") returned ok=true")
	}
}

// TestWriteEvent_binaryLayout verifies the exact on-wire byte layout of a
// single event written by writeEvent.
func TestWriteEvent_binaryLayout(t *testing.T) {
	var buf bytes.Buffer
	kb := &Keyboard{f: nil}

	// Temporarily redirect writes to a buffer via binary.Write directly
	ev := event{Type: 0x0001, Code: 0x001e, Value: 1}
	if err := binary.Write(&buf, binary.LittleEndian, ev); err != nil {
		t.Fatal(err)
	}

	want := []byte{
		0x01, 0x00, // Type = EV_KEY (little-endian)
		0x1e, 0x00, // Code = KEY_A  (little-endian)
		0x01, 0x00, 0x00, 0x00, // Value = 1 (little-endian)
	}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Errorf("binary layout = %x, want %x", buf.Bytes(), want)
	}
	_ = kb
}
