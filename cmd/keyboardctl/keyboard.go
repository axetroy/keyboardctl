//go:build windows

package main

import (
	"fmt"
	"math/rand"
	"time"
)

// randHoldDuration returns a randomised key-hold duration in the range [40, 120) ms.
// Real physical key-presses last roughly 50–100 ms depending on the typist;
// injecting a press and release with this gap is indistinguishable from hardware
// in terms of timing at the class-driver level.
//
// math/rand is automatically seeded in Go 1.20+ so no explicit seeding is needed.
func randHoldDuration() time.Duration {
	return time.Duration(40+rand.Intn(80)) * time.Millisecond
}

// randTransitionDelay returns a short randomised delay (10–40 ms) used between
// modifier-down and key-down (or key-up and modifier-up) in a combo keystroke.
//
// math/rand is automatically seeded in Go 1.20+ so no explicit seeding is needed.
func randTransitionDelay() time.Duration {
	return time.Duration(10+rand.Intn(31)) * time.Millisecond
}

// PressKey sends a key-down event for the given scan code.
func (d *KeyboardDriver) PressKey(code ScanCode) error {
	mc, flags := scToMakeFlags(code)
	if err := d.SendKey(mc, flags|keyFlagMake); err != nil {
		return fmt.Errorf("PressKey(0x%04X): %w", code, err)
	}
	return nil
}

// ReleaseKey sends a key-up event for the given scan code.
func (d *KeyboardDriver) ReleaseKey(code ScanCode) error {
	mc, flags := scToMakeFlags(code)
	if err := d.SendKey(mc, flags|keyFlagBreak); err != nil {
		return fmt.Errorf("ReleaseKey(0x%04X): %w", code, err)
	}
	return nil
}

// TapKey presses and releases a key with a realistic randomised hold time.
//
// Unlike batching both events in a single IOCTL, press and release are sent
// as two separate kernel calls separated by a random hold duration (40–120 ms).
// This matches how a real keyboard port driver delivers events: the key-down
// interrupt fires first, and the key-up interrupt fires after the physical
// key is held and then released — they are never simultaneous.
func (d *KeyboardDriver) TapKey(code ScanCode) error {
	if err := d.PressKey(code); err != nil {
		return fmt.Errorf("TapKey(0x%04X): %w", code, err)
	}
	time.Sleep(randHoldDuration())
	if err := d.ReleaseKey(code); err != nil {
		return fmt.Errorf("TapKey(0x%04X): %w", code, err)
	}
	return nil
}

// TypeText types the given text string one character at a time, using the
// standard US keyboard layout.  Uppercase letters are typed with Left Shift.
// Unmappable characters are silently skipped.
//
// delay is the nominal inter-keystroke pause (time between key releases and
// the next key press).  When non-zero, a ±30 % random jitter is applied to
// each interval so that consecutive keystrokes do not arrive at perfectly
// uniform intervals, matching natural human typing cadence.
// Use 0 for no inter-keystroke delay (hold time from TapKey still applies).
func (d *KeyboardDriver) TypeText(text string, delay time.Duration) error {
	for _, r := range text {
		sc, needsShift, ok := RuneToScanCode(r)
		if !ok {
			continue
		}

		if needsShift {
			if err := d.PressKey(ScancodeLShift); err != nil {
				return err
			}
		}

		if err := d.TapKey(sc); err != nil {
			return err
		}

		if needsShift {
			if err := d.ReleaseKey(ScancodeLShift); err != nil {
				return err
			}
		}

		if delay > 0 {
			// Apply ±30 % jitter: actual sleep is in [70 %, 130 %) of delay.
			// span = 60 % of delay (the width of the [70 %, 130 %) window).
			// A floor of 1 ensures rand.Int63n never receives 0 for tiny delays.
			lo := int64(float64(delay) * 0.70)
			span := int64(float64(delay) * 0.60)
			if span < 1 {
				span = 1
			}
			time.Sleep(time.Duration(lo + rand.Int63n(span)))
		}
	}
	return nil
}

// PressModifierAndKey presses modifier + key and then releases them in reverse
// order — suitable for combos like Ctrl+C, Alt+F4, etc.
//
// Each event is sent as a separate IOCTL call with small randomised delays
// between them, replicating the natural timing of a human pressing a hotkey:
// the modifier goes down first, then the key is pressed after a short
// transition delay, the key is released after a realistic hold, and finally
// the modifier comes up.
func (d *KeyboardDriver) PressModifierAndKey(modifier, key ScanCode) error {
	// 1. Modifier down
	if err := d.PressKey(modifier); err != nil {
		return fmt.Errorf("PressModifierAndKey(0x%04X, 0x%04X): %w",
			modifier, key, err)
	}
	time.Sleep(randTransitionDelay())

	// 2. Key down
	if err := d.PressKey(key); err != nil {
		return fmt.Errorf("PressModifierAndKey(0x%04X, 0x%04X): %w",
			modifier, key, err)
	}
	time.Sleep(randHoldDuration())

	// 3. Key up
	if err := d.ReleaseKey(key); err != nil {
		return fmt.Errorf("PressModifierAndKey(0x%04X, 0x%04X): %w",
			modifier, key, err)
	}
	time.Sleep(randTransitionDelay())

	// 4. Modifier up
	if err := d.ReleaseKey(modifier); err != nil {
		return fmt.Errorf("PressModifierAndKey(0x%04X, 0x%04X): %w",
			modifier, key, err)
	}
	return nil
}
