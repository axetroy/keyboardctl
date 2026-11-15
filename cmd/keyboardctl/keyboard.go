//go:build windows

package main

import (
	"fmt"
	"math/rand"
	"time"
)

// rng is the process-wide randomness source for event timing.  Go 1.20+ seeds
// the global source randomly, but we keep an explicit source so tests can pass
// a deterministic one to the timing helpers in timing.go.
var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

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

// TapKey presses and releases a key with a realistic hold time.
//
// Press and release are two separate kernel calls separated by a hold duration,
// because that is how a port driver delivers them: the key-down interrupt fires
// first, the key-up interrupt fires when the physical key is released, and they
// are never simultaneous.  The hold time comes from the key's class-specific
// log-normal distribution (timing.go).
func (d *KeyboardDriver) TapKey(code ScanCode) error {
	if err := d.PressKey(code); err != nil {
		return fmt.Errorf("TapKey(0x%04X): %w", code, err)
	}
	time.Sleep(holdDuration(rng, code))
	if err := d.ReleaseKey(code); err != nil {
		return fmt.Errorf("TapKey(0x%04X): %w", code, err)
	}
	return nil
}

// TypeText types the given text one character at a time on the US layout.
// Uppercase letters are typed with Left Shift; unmappable characters are
// skipped (the caller reports them).
//
// delay is the nominal pause between one key release and the next key press.
// It is used as the *median* of a log-normal interval whose spread also
// depends on the key pair, so a "lo" roll is faster than an "az" reach.  With
// delay == 0 the inter-key interval is still non-zero, because real fingers
// take time to travel between keys.
func (d *KeyboardDriver) TypeText(text string, delay time.Duration) error {
	var prev ScanCode
	havePrev := false

	for _, r := range text {
		sc, needsShift, ok := RuneToScanCode(r)
		if !ok {
			continue
		}

		if havePrev {
			time.Sleep(interKeyDelay(rng, prev, sc, delay))
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

		prev, havePrev = sc, true
	}
	return nil
}

// PressModifierAndKey presses modifier + key and releases them in reverse
// order — suitable for combos like Ctrl+C and Alt+F4.
//
// Each event is a separate IOCTL with a small randomised gap, replicating how a
// human presses a hotkey: the modifier settles first, the main key follows
// after a short transition, it is held for a normal press, and the modifier is
// released last.  The gaps are drawn from tight distributions so the sequence
// reads as one chord rather than two separate keystrokes.
func (d *KeyboardDriver) PressModifierAndKey(modifier, key ScanCode) error {
	return d.PressCombo([]ScanCode{modifier}, key)
}

// PressCombo presses zero or more modifiers plus a final key, then releases
// them in reverse order.  Real hotkeys nest this way: the modifiers go down
// (in the order given), the key follows, then the key comes up and the
// modifiers release last, outermost first.  With one modifier it is identical
// to PressModifierAndKey.
func (d *KeyboardDriver) PressCombo(modifiers []ScanCode, key ScanCode) error {
	// 1. Modifiers down, in the order the caller gave them.
	for _, m := range modifiers {
		if err := d.PressKey(m); err != nil {
			return fmt.Errorf("PressCombo: %w", err)
		}
		time.Sleep(comboTransitionDelay(rng))
	}

	// 2. Key down
	if err := d.PressKey(key); err != nil {
		return fmt.Errorf("PressCombo: %w", err)
	}
	time.Sleep(holdDuration(rng, key))

	// 3. Key up
	if err := d.ReleaseKey(key); err != nil {
		return fmt.Errorf("PressCombo: %w", err)
	}
	time.Sleep(comboTransitionDelay(rng))

	// 4. Modifiers up, in reverse order (innermost first).
	for i := len(modifiers) - 1; i >= 0; i-- {
		if err := d.ReleaseKey(modifiers[i]); err != nil {
			return fmt.Errorf("PressCombo: %w", err)
		}
		if i > 0 {
			time.Sleep(comboTransitionDelay(rng))
		}
	}
	return nil
}
