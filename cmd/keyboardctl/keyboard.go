//go:build windows

package main

import (
	"fmt"
	"time"
)

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

// TapKey presses and releases a key in a single batched IOCTL call.
func (d *KeyboardDriver) TapKey(code ScanCode) error {
	mc, baseFlags := scToMakeFlags(code)
	events := []KeyboardInputData{
		{MakeCode: mc, Flags: baseFlags | keyFlagMake},
		{MakeCode: mc, Flags: baseFlags | keyFlagBreak},
	}
	if err := d.sendBatch(events); err != nil {
		return fmt.Errorf("TapKey(0x%04X): %w", code, err)
	}
	return nil
}

// TypeText types the given text string one character at a time, using the
// standard US keyboard layout.  Uppercase letters are typed with Left Shift.
// Unmappable characters are silently skipped.
//
// delay is the inter-keystroke pause; use 0 for no delay.
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
			time.Sleep(delay)
		}
	}
	return nil
}

// PressModifierAndKey presses modifier + key simultaneously, then releases
// them in reverse order — suitable for combos like Ctrl+C, Alt+F4, etc.
func (d *KeyboardDriver) PressModifierAndKey(modifier, key ScanCode) error {
	modMc, modFlags := scToMakeFlags(modifier)
	keyMc, keyFlags := scToMakeFlags(key)

	// Down: modifier first, then key
	events := []KeyboardInputData{
		{MakeCode: modMc, Flags: modFlags | keyFlagMake},
		{MakeCode: keyMc, Flags: keyFlags | keyFlagMake},
		// Up: key first, then modifier
		{MakeCode: keyMc, Flags: keyFlags | keyFlagBreak},
		{MakeCode: modMc, Flags: modFlags | keyFlagBreak},
	}

	if err := d.sendBatch(events); err != nil {
		return fmt.Errorf("PressModifierAndKey(0x%04X, 0x%04X): %w",
			modifier, key, err)
	}
	return nil
}
