# AGENTS.md

Repository notes for future agents working on `keyboardctl`.

## What this is

Windows-only keyboard input simulator: a Go CLI (`keyboardctl.exe`) plus a KMDF
kernel driver (`keyboardsimulator.sys`). There is no Unix support and no plan to
add any.

## Layout

- `cmd/keyboardctl/` - the Go CLI (`//go:build windows`), except `timing.go` and
  `scancodes.go` which are platform-independent so they can be unit-tested
  anywhere.
- `driver/` - the KMDF driver, built with MSBuild (`driver.vcxproj`).
  - `keyboardctl.inf` - installs the filter (primitive/class-upper-filter style).
- `.github/workflows/ci.yml` - format/vet/test/CLI build + driver compile.
- `.github/workflows/release.yml` - changesets version PR + signed release.
- `install.bat` / `uninstall.bat` - end-user install from a release zip.
- `package.json` - version source for changesets (private, never published).

## Architecture: keyboard class upper filter

The driver is a KMDF **class upper filter** for the Keyboard device class, not a
device of its own. The installer writes it into the class `UpperFilters` value
*ahead of* `kbdclass`, so each keyboard's stack is:

    kbdclass -> keyboardsimulator -> <port driver: i8042prt / kbdhid> -> <bus>

`UpperFilters` is ordered bottom-up: the first entry sits closest to the port
driver. We must be *below* kbdclass because `IOCTL_INTERNAL_KEYBOARD_CONNECT`
flows **downward** — kbdclass is the sender (`KbdSendConnectRequest`) and puts its
own `KeyboardClassServiceCallback` in `CONNECT_DATA.ClassService`. The filter
saves that callback, replaces it with its own, and forwards the request to the
port driver. Injection then calls the saved callback with synthetic
`KEYBOARD_INPUT_DATA`; kbdclass queues it exactly as for real hardware.

Consequences that matter:

- `driver.c` `DriverEntry` creates a **control device** (`\\.\KbdFilter`) for the
  user-mode sideband. A filter driver has no device of its own, so the control
  device cannot live in `EvtDeviceAdd`; it is created up front in `DriverEntry`
  (the WDK PortSniffer sample does the same).
- `EvtDeviceAdd` calls `WdfFdoInitSetFilter` and creates the filter device; the
  default queue is **parallel**, because a sequential queue deadlocks when the
  port driver sends a request up while another IOCTL is outstanding.
- The connect/disconnect handlers forward the IRP down with
  `WdfRequestSend(...SEND_AND_FORGET)`; they never complete it themselves.
- The control device is created with `SDDL_DEVOBJ_SYS_ALL_ADM_ALL`, so only
  Administrators/SYSTEM can open it — injection is a privileged operation.

### Installing a filter needs an INF, not `sc create`

The old design registered the .sys as a legacy kernel service with `sc create`
and then used the PnP `EvtDeviceAdd` callback — contradictory, and it could never
attach to a keyboard stack. A filter is installed through its INF with
`pnputil /add-driver keyboardctl.inf /install`, which copies the package into the
Driver Store, creates the service, and applies the INF's `AddReg` that adds us to
`UpperFilters`. The INF is a **primitive driver**: no `[Manufacturer]`/`[Models]`,
architecture-decorated sections (`.NTamd64`), and it carries the KMDF binding in
`[DefaultInstall.NTamd64.Wdf]` / `KmdfService` (without that the framework library
does not bind and the driver fails to start).

Two things that will bite:

- **`CatalogFile` is declared, so a signed catalog must ship.** `pnputil` refuses
  a package whose catalog is missing or unsigned. `EnableInf2cat=true` +
  `SignMode=TestSign` in `driver.vcxproj` generate and sign `keyboardctl.cat`
  during the build. Sign the `.sys` *before* building the catalog (the catalog
  hashes the already-signed file); doing it the other way invalidates it.
- **`UpperFilters` is set as a whole** (`0x00010000`, `REG_MULTI_SZ`), so the INF
  overwrites any existing value. A third-party keyboard filter would be dropped
  and break. `install.bat` refuses to run if it finds a foreign filter, and
  `uninstall.bat` repairs the value instead of leaving a stale entry.

A filter is picked up at boot, so **a reboot is required** after install (and
after enabling test-signing).

## Keystroke timing model (`cmd/keyboardctl/timing.go`)

Timing lives in `timing.go`, which carries **no build tag** on purpose: it is
pure math, so its tests run on any OS, including the Linux CI runner, even though
the rest of the CLI is `//go:build windows`.

- Intervals are **log-normal**, not uniform. A uniform draw over a plausible
  range has a flat histogram, which no real device emits; a log-normal matches
  the right-skewed shape of human key holds. `skewness()` in `timing_test.go`
  asserts this, so a regression back to uniform fails CI.
- Hold time is parameterised by **key class** (`classChar`, `classModifier`,
  `classFunction`, `classToggle`). `classOf` is an **explicit switch, not byte
  ranges**: extended keys collide with unrelated plain keys on their low byte
  (`Home` 0xE047 vs keypad `7` 0x47; keypad `*` 0x37 vs `Print Screen` 0xE037),
  and a range check misclassifies them. A test caught exactly that bug.
- Inter-key gaps model the **digraph effect** via `fingerOf`/`digraphScale`:
  same-finger pairs are slower than same-hand rolls, so intervals are
  pair-correlated instead of independent.
- `rng` is an explicit `*rand.Rand` so the helpers take a seeded source in tests
  (`sampleDurations`); production uses a time-seeded source.

Two fields that look like bugs but are not, and should not be "fixed":

- **`UnitId` / `ExtraInformation` stay 0.** Real `i8042prt` writes `UnitId = 0`,
  and no real port driver sets `ExtraInformation`. The injected values already
  match hardware.
- **`-hold` sends a single down event, with no typematic repeat.** Auto-repeat is
  generated by the **hardware device**, not by kbdclass or win32k: kbdclass only
  passes `IOCTL_KEYBOARD_*_TYPEMATIC` through, and win32k sets `KF_REPEAT` purely
  from whether the lower layer already sent a repeated down packet. Synthesising
  repeats in the filter would add a "double repeat" fingerprint.

## Detectability (be honest about it)

The software approach is **detectable**, and the fingerprints below cannot be
removed while keeping the architecture. Do not claim otherwise, and do not add
code that impersonates Microsoft components or targets a specific anti-cheat
product.

- Test signing (`bcdedit /set testsigning on`), the "Test Mode" watermark, and a
  self-signed cert in Root/TrustedPublisher are trivially visible. Most kernel
  anti-cheat refuses to run with test signing on, without needing to "detect"
  anything.
- The Keyboard class `UpperFilters` value, `keyboardsimulator.sys` in the loaded
  module list, the extra non-Microsoft device in the keyboard stack, and the
  `\\.\KbdFilter` symlink are all architectural: a filter driver must be present
  in each.
- A legitimate code-signing certificate removes the test-signing fingerprints but
  not the `UpperFilters`/module/stack ones. The only zero-fingerprint option is
  real USB HID hardware. This is documented in the README.

## History: the old design was broken (fixed)

Before this architecture the driver sent `IOCTL_INTERNAL_KEYBOARD_CONNECT` *to*
`\Device\KeyboardClass0` and expected kbdclass to return a `ClassService`. That
direction is inverted — kbdclass is the sender and its
`IRP_MJ_INTERNAL_DEVICE_CONTROL` handler is `KeyboardClassPassThrough`, which only
forwards downward, and port drivers reject a second connect with
`STATUS_SHARING_VIOLATION`. Injection could never work. The fix is the filter
design above.

## Build

- CLI: `make build` or `go build ./cmd/keyboardctl`.
- Driver: `msbuild driver\driver.vcxproj /p:Configuration=Release /p:Platform=x64`
  (needs Visual Studio 2022 + the WDK). The KMDF package lands in
  `driver\x64\Release\driver\` — a `$(OutDir)<project>\` subdirectory, **not**
  `driver\x64\Release\` directly: `keyboardsimulator.sys`, the stampinf-processed
  `keyboardctl.inf` and the signed `keyboardctl.cat`, all three next to each other
  (which is what `pnputil /add-driver keyboardctl.inf /install` needs). The raw
  stamped INF is also at `driver\x64\Release\keyboardctl.inf`. The INF's `DriverVer`
  is stamped from `/p:DriverVersion`; ship the packaged INF, not the source copy.
- `SignMode=TestSign` + `EnableInf2cat=true` make the WDK toolset generate the test
  certificate, sign the .sys and build+sign the catalog. Creating the certificate
  writes to a machine-wide store, so a **non-elevated** local build can fail with
  `CryptographicException: Access is denied`; run the build from an elevated prompt
  (CI runners are already elevated).

## CI gotchas (learned the hard way)

- **Run on `windows-2022`, not `windows-latest`.** `windows-latest` is now
  Windows Server 2025 + VS 2026, whose image ships only the WDK Visual Studio
  extension and not the full Windows Driver Kit, so `ntddk.h` is missing and the
  driver fails with `C1083`.
- **Keep text files LF.** Windows runners default to `core.autocrlf=true`; without
  `.gitattributes` (`* text=auto eol=lf`) the Go files check out as CRLF and
  `gofmt` reports every file. `*.bat` must stay CRLF (`eol=crlf`).
- **`SkipPackageVerification=true` is deliberate.** The in-build InfVerif runs in
  Windows-Driver mode, which a class filter (no `[Manufacturer]`/`[Models]`)
  legitimately fails. Run `infverif /h keyboardctl.inf` by hand for the WHQL bar.
- **Sign the `.sys` before building the catalog.** `Inf2Cat` hashes the already
  signed file; signing afterwards invalidates the catalog and `pnputil` rejects
  the package.

## Versioning

changesets drives versions and `CHANGELOG.md`. `package.json` is private, so
`.changeset/config.json` sets `privatePackages: { version: true, tag: true }` to
get version bumps and `vX.Y.Z` tags without publishing to npm. The version is
injected into builds via `-X main.version` (CLI) and `/p:DriverVersion` (driver).
