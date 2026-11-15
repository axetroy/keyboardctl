---
"keyboardctl": patch
---

Add `-list` to print every accepted key name, warn when `-text` drops
characters that have no US-layout scan code, and point unknown-key errors at
`-list`.  `uninstall.bat` now also removes the directory from the user PATH
that `install.bat` added.
