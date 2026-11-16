---
"keyboardctl": patch
---

Fix the installer never enabling test-signing, which left the driver unloaded.

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
