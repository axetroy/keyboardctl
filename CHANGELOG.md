# keyboardctl

## 1.2.1

### Patch Changes

- 5c6c52a: Fix the installer never enabling test-signing, which left the driver unloaded.

  `install.bat` decided whether to run `bcdedit /set testsigning on` by checking
  `Get-AuthenticodeSignature` on the driver **after** importing the bundled test
  certificate. The chain therefore validated and the status read `Valid`, so the
  check always passed and the command was always skipped — contradicting the
  script's own comment that the driver "loads only if the certificate is trusted
  AND test-signing is on".

  A test-signed kernel driver does not load while test-signing is off, so the
  service was registered but `DriverEntry` never ran and `\\.\KbdFilter` never
  appeared. Every command, including `-selftest`, then failed with
  `driver: open \\.\KbdFilter: The system cannot find the file specified`. Rebooting
  did not help, and re-running the installer did not either: it saw the certificate
  already trusted and skipped the setting again.

  - `install.bat` now reads the real BCD state (`bcdedit /enum {current}`) to decide
    whether test-signing is on, and **fails loudly** if it cannot enable it, instead
    of reporting a successful install that cannot work. This usually means Secure
    Boot is blocking it, which the message now says.
  - `install.bat` now detects **Memory integrity (HVCI)** and stops with an
    explanation. HVCI refuses to load a test-signed driver even with test-signing
    on, so a machine with it enabled can never load this driver; the installer used
    to report success there too.
  - `install.bat` verifies the service exists after `pnputil` and stops with the
    `pnputil /enum-drivers` hint if it does not, rather than letting `-selftest`
    fail later with a bare file-not-found.
  - `-selftest` now prints an ordered checklist when it cannot open the device
    (reboot? test-signing on? HVCI off? service registered? `UpperFilters` correct?
    Event Viewer for a start failure?), so the error explains itself.
  - README gains a troubleshooting section for that exact error; AGENTS.md records
    that trusting the certificate is not the same as the driver being loadable.

## 1.2.0

### Minor Changes

- 2ddd31d: Install the driver as a real keyboard class upper filter, and make the install
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

- eb59a35: Make injected keystroke timing statistically match real hardware, and document
  what the software approach can and cannot hide:

  - New `cmd/keyboardctl/timing.go`: key-hold and inter-key intervals are drawn
    from **log-normal** distributions instead of a flat uniform range. Real typing
    is right-skewed, so a uniform histogram was itself a detectable signature.
  - Hold time now depends on the **key class** (character / modifier / function /
    toggle), because a held Shift behaves differently from a tapped letter.
  - Inter-key intervals model the **digraph effect**: same-finger pairs (`ed`) are
    slower than same-hand rolls (`lo`), so intervals are pair-correlated rather
    than independent.
  - `timing.go` is platform-independent, so its statistical tests run on any OS,
    including the Linux CI runner.
  - README: new "timing model and fidelity" section, including the architectural
    fingerprints that a filter-driver design cannot remove (test signing,
    `UpperFilters`, the loaded module, the device stack).

### Patch Changes

- 20c8dc0: Fix `-combo` silently dropping arguments, and support multi-modifier combos.

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

- e2a6d6f: Make the CLI easier to use and installs fail loudly:

  - `-list` prints every accepted key name (sorted, including aliases).
  - `-selftest` checks the driver is reachable without sending a keystroke;
    `install.bat` runs it so a broken install is reported immediately.
  - `-text` warns up front about characters that have no US-layout scan code
    instead of dropping them silently; unknown-key errors now point at `-list`.
  - `uninstall.bat` removes the directory from the user PATH that
    `install.bat` added.

## 1.1.0

### Minor Changes

- 36c36f0: Add automated releases: changesets drives versioning and the changelog, and CI
  builds, signs and publishes a ready-to-run Windows zip (CLI + driver + test
  certificate) to GitHub Releases. Installation no longer requires a compiler or
  the WDK - download the zip, run `install.bat`, done.
