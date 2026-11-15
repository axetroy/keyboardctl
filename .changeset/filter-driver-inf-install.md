---
"keyboardctl": minor
---

Install the driver as a real keyboard class upper filter, and make the install
tell you whether it actually worked:

- The driver is now a KMDF **class upper filter** installed through its INF with
  `pnputil`, registered ahead of `kbdclass` in the Keyboard class `UpperFilters`.
  The previous `sc create` service could never attach to a keyboard stack, so
  injection did not work at all.
- New `IOCTL_KEYBOARD_QUERY_STATE`: `-selftest` now distinguishes "the driver is
  loaded" from "a keyboard is connected and injection will reach the system".
- `install.bat` / `uninstall.bat` rewritten around the INF: the installer refuses
  to clobber a third-party keyboard filter, verifies the registration, offers the
  required reboot, and only self-tests when the filter is actually loaded.
- `keyboardctl <key>` is shorthand for `keyboardctl -key <key>`.
- The release package now ships `keyboardctl.inf` and the signed `keyboardctl.cat`
  the INF requires.
