# AGENTS.md

Repository notes for future agents working on `keyboardctl`.

## What this is

Windows-only keyboard input simulator: a Go CLI (`keyboardctl.exe`) plus a KMDF
kernel driver (`keyboardsimulator.sys`). There is no Unix support and no plan to
add any.

## Layout

- `cmd/keyboardctl/` - the Go CLI (`//go:build windows`).
- `driver/` - the KMDF driver, built with MSBuild (`driver.vcxproj`).
- `.github/workflows/ci.yml` - format/vet/test/CLI build + driver compile.
- `.github/workflows/release.yml` - changesets version PR + signed release.
- `install.bat` / `uninstall.bat` - end-user install from a release zip.
- `package.json` - version source for changesets (private, never published).

## Known defect: the driver cannot inject at all (open)

`driver/inject.c` sends `IOCTL_INTERNAL_KEYBOARD_CONNECT` *to*
`\Device\KeyboardClass0` and expects kbdclass to hand back a `ClassService`
callback. The direction is inverted, so injection can never work:

- kbdclass is the **sender** of that IOCTL. It sends it *down* to the port
  driver (i8042prt / kbdhid) with its own `KeyboardClassServiceCallback` in
  `CONNECT_DATA.ClassService` (`KbdSendConnectRequest` in kbdclass.c).
- kbdclass registers `IRP_MJ_INTERNAL_DEVICE_CONTROL` as
  `KeyboardClassPassThrough`, which only forwards the IRP to `TopPort` (the
  lower port driver). It never returns a `ClassService`.
- Port drivers reject a second connect with `STATUS_SHARING_VIOLATION`
  (`i8042prt`'s `IOCTL_INTERNAL_KEYBOARD_CONNECT` handler), so the connect
  fails outright.

The correct design is a **filter driver in the keyboard stack** (registered
via `UpperFilters` in an INF, like the WDK `kbfiltr` sample): intercept
`IOCTL_INTERNAL_KEYBOARD_CONNECT`, keep the `ClassService` the port driver
returns, and call it to inject `KEYBOARD_INPUT_DATA`.

The driver is also a KMDF **non-PnP control device** loaded with
`sc create ... type= kernel`, yet it uses the PnP-style `EvtDeviceAdd`
callback. Microsoft's guidance is that a non-PnP KMDF driver sets
`WdfDriverInitNonPnpDriver`, provides `EvtDriverUnload`, and creates the
device in `DriverEntry` via `WdfControlDeviceInitAllocate` — not
`EvtDeviceAdd`.

## Build

- CLI: `make build` or `go build ./cmd/keyboardctl`.
- Driver: `msbuild driver\driver.vcxproj /p:Configuration=Release /p:Platform=x64`
  (needs Visual Studio 2022 + the WDK). The KMDF toolset test-signs the .sys and
  exports `keyboardsimulator.cer` next to it.

## CI gotchas (learned the hard way)

- **Run on `windows-2022`, not `windows-latest`.** `windows-latest` is now
  Windows Server 2025 + VS 2026, whose image ships only the WDK Visual Studio
  extension and not the full Windows Driver Kit, so `ntddk.h` is missing and the
  driver fails with `C1083`.
- **Keep text files LF.** Windows runners default to `core.autocrlf=true`; without
  `.gitattributes` (`* text=auto eol=lf`) the Go files check out as CRLF and
  `gofmt` reports every file. `*.bat` must stay CRLF (`eol=crlf`).
- **Do not hand-roll driver signing.** The KMDF MSBuild toolset already test-signs
  the driver and exports the certificate; an extra `signtool` pass is redundant.

## Versioning

changesets drives versions and `CHANGELOG.md`. `package.json` is private, so
`.changeset/config.json` sets `privatePackages: { version: true, tag: true }` to
get version bumps and `vX.Y.Z` tags without publishing to npm. The version is
injected into builds via `-X main.version` (CLI) and `/p:DriverVersion` (driver).
