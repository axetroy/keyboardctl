---
"keyboardctl": patch
---

Fix `-combo` silently dropping arguments, and support multi-modifier combos.

Previously `-combo` read only its first two arguments and ignored the rest, so
`-combo ctrl shift a` sent **Ctrl+Shift** (not Ctrl+Shift+A) and
`-combo ctrl alt delete` sent **Ctrl+Alt** (not Ctrl+Alt+Del) — with no error.
Sending the wrong key combination is worse than failing, so this was a real
defect, not just a missing feature.

- `-combo` now takes **any number of modifiers plus one final key**:
  `-combo ctrl shift a` is Ctrl+Shift+A, `-combo ctrl alt delete` is
  Ctrl+Alt+Del.
- Every argument except the last must be a modifier. A non-modifier anywhere
  else is a hard error, so a mistyped combo fails loudly instead of sending a
  different one.
- New `PressCombo(modifiers, key)` presses the modifiers in order, the key,
  then releases in reverse (key first, outermost modifier last).
  `PressModifierAndKey` is now a one-modifier wrapper over it.
- Argument parsing moved to `parseCombo`/`tryParseScanCode`/`isModifier` in
  `args.go`, which are platform-independent and unit-tested, including
  regression cases for the old silent truncation.
