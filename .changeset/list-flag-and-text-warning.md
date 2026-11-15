---
"keyboardctl": patch
---

Make the CLI easier to use and installs fail loudly:

- `-list` prints every accepted key name (sorted, including aliases).
- `-selftest` checks the driver is reachable without sending a keystroke;
  `install.bat` runs it so a broken install is reported immediately.
- `-text` warns up front about characters that have no US-layout scan code
  instead of dropping them silently; unknown-key errors now point at `-list`.
- `uninstall.bat` removes the directory from the user PATH that
  `install.bat` added.
