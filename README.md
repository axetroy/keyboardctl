# keyboardctl

Software-only keyboard input simulator for Windows — no physical hardware required.
Applications receive the injected keystrokes exactly as if a real keyboard had been pressed.

---

## Architecture

```
用户态 (Ring 3)
└── Go控制程序 (keyboardctl.exe)
    └── 业务逻辑、IOCTL通信 → \\.\KeyboardSimulator

内核态 (Ring 0)
└── C键盘驱动 (keyboardsimulator.sys)
    ├── WDF虚拟设备 + 符号链接
    ├── IOCTL请求处理队列
    └── 附加到 \\Device\\KeyboardClass0 → 事件注入
```

```
┌─────────────────────────────────────────────────────┐
│          User Applications (Ring 3)                 │
│     (Notepad, browser, games, …)                    │
└─────────────────────┬───────────────────────────────┘
                      │  Win32 / Raw Input
                      ▼
┌─────────────────────────────────────────────────────┐
│          win32k.sys / RawInput subsystem            │
└─────────────────────┬───────────────────────────────┘
                      │  ClassService callback
                      ▼
┌─────────────────────────────────────────────────────┐
│     kbdclass.sys — Keyboard Class Driver            │
│     \\Device\\KeyboardClass0                        │
└─────────────────────┬───────────────────────────────┘
                      │  IOCTL_INTERNAL_KEYBOARD_CONNECT
                      ▼
┌─────────────────────────────────────────────────────┐
│     keyboardsimulator.sys (Ring 0, KMDF)            │
│     \\Device\\KeyboardSimulator                     │
│     \\.\KeyboardSimulator  (symbolic link)          │
└─────────────────────┬───────────────────────────────┘
                      │  IOCTL_KEYBOARD_INJECT (DeviceIoControl)
                      ▼
┌─────────────────────────────────────────────────────┐
│     keyboardctl.exe (Ring 3, Go)                    │
│     -key / -text / -combo / -hold / -release        │
└─────────────────────────────────────────────────────┘
```

### Kernel driver (`driver/`)

| File | Purpose |
|------|---------|
| `driver.h` | Shared definitions: `DEVICE_CONTEXT`, `IOCTL_KEYBOARD_INJECT`, prototypes |
| `driver.c` | `DriverEntry`, `EvtDeviceAdd` — WDF device + symbolic link creation |
| `queue.c`  | `QueueInitialize`, `EvtIoDeviceControl` — IOCTL dispatch |
| `inject.c` | `ConnectKeyboardStack`, `InjectKeyboardEvents` — attach to `KeyboardClass0` and call its `ClassService` callback |
| `sources`  | WDK (legacy) build configuration |
| `makefile` | WDK build stub |

### Go userspace (`cmd/keyboardctl/`)

| File | Purpose |
|------|---------|
| `scancodes.go` | `ScanCode` type, all PC/AT scan codes, `ParseScanCode`, `RuneToScanCode` |
| `driver.go` | `KeyboardDriver` struct, `Open`/`Close`, `SendKey`, `sendBatch`, raw IOCTL |
| `keyboard.go` | `PressKey`, `ReleaseKey`, `TapKey`, `TypeText`, `PressModifierAndKey` |
| `main.go` | CLI: `-key`, `-text`, `-combo`, `-hold`, `-release`, `-delay` |
| `scancodes_test.go` | Scan code unit tests (platform-independent, run on any OS) |

---

## Core data structures

```c
// KEYBOARD_INPUT_DATA (ntddkbd.h) — sent via IOCTL_KEYBOARD_INJECT
typedef struct _KEYBOARD_INPUT_DATA {
    USHORT UnitId;           // always 0
    USHORT MakeCode;         // PC/AT scan code (Set 1)
    USHORT Flags;            // KEY_MAKE=0, KEY_BREAK=1, KEY_E0=2, KEY_E1=4
    USHORT Reserved;         // must be 0
    ULONG  ExtraInformation; // must be 0
} KEYBOARD_INPUT_DATA;
```

```go
// Go mirror of KEYBOARD_INPUT_DATA
type KeyboardInputData struct {
    UnitId           uint16
    MakeCode         uint16
    Flags            uint16
    Reserved         uint16
    ExtraInformation uint32
}
```

---

## IOCTL protocol

```
IOCTL_KEYBOARD_INJECT = CTL_CODE(FILE_DEVICE_KEYBOARD, 0x800,
                                  METHOD_BUFFERED, FILE_WRITE_ACCESS)
                      = 0x000BA000
```

Write one or more `KEYBOARD_INPUT_DATA` structures as the input buffer.
Extended keys (arrows, Insert, Delete, Home, End, …) must have `Flags |= KEY_E0`.

---

## Common scan codes

| Key | Scan code | Key | Scan code |
|-----|-----------|-----|-----------|
| A–Z | 0x1E, 0x30, 0x2E, … | 0–9 | 0x0B, 0x02–0x0A |
| Enter | 0x1C | Space | 0x39 |
| LShift | 0x2A | RShift | 0x36 |
| LCtrl | 0x1D | RCtrl | E0+0x1D |
| LAlt | 0x38 | RAlt | E0+0x38 |
| Up ↑ | E0+0x48 | Down ↓ | E0+0x50 |
| Left ← | E0+0x4B | Right → | E0+0x4D |
| F1–F10 | 0x3B–0x44 | F11 | 0x57 | F12 | 0x58 |

---

## Requirements

| Component | Requirement |
|-----------|-------------|
| OS        | Windows 10 / 11 x64 |
| Privileges | Administrator (driver install & run) |
| WDK       | Windows Driver Kit 10 (for building the driver) |
| Go        | ≥ 1.21 |

> **Driver signing**: For development use `bcdedit /set testsigning on`.
> Production deployment requires an EV code-signing certificate.

---

## Build

### Driver (Windows, requires WDK)

```bat
cd driver
build -cZ                        :: legacy WDK
:: or: msbuild driver.vcxproj   :: Visual Studio + WDK
```

Output: `driver\objfre_wlh_amd64\amd64\keyboardsimulator.sys`

### Go CLI

```bash
# Cross-compile for Windows from any platform
make go
# or: GOOS=windows GOARCH=amd64 go build -o keyboardctl.exe ./cmd/keyboardctl/

# Run tests (platform-independent, works on Linux/macOS too)
make test
```

---

## Quick start (Windows, Administrator)

```bat
:: 1. Install the driver
scripts\install.bat

:: 2. Press A
keyboardctl.exe -key a

:: 3. Type text with 100 ms inter-key delay
keyboardctl.exe -text "Hello, World!" -delay 100

:: 4. Ctrl+C
keyboardctl.exe -combo ctrl c

:: 5. Alt+F4
keyboardctl.exe -combo lalt f4

:: 6. Uninstall when done
scripts\uninstall.bat
```

---

## CLI reference

```
keyboardctl.exe -key    <key>                   press and release a key
keyboardctl.exe -hold   <key>                   key-down only
keyboardctl.exe -release <key>                  key-up only
keyboardctl.exe -combo  <modifier> <key>        simultaneous combo (e.g. Ctrl+C)
keyboardctl.exe -text   <string> [-delay <ms>]  type a string character by character
```

Key argument: hex scan code (`0x1E`) or case-insensitive name (`a`, `enter`, `ctrl`, `f12`, `up`, …).

---

## Go library

```go
drv, err := Open()   // opens \\.\KeyboardSimulator
defer drv.Close()

drv.TapKey(ScancodeA)                             // press & release A
drv.TypeText("Hello!", 50*time.Millisecond)       // type with delay
drv.PressModifierAndKey(ScancodeLCtrl, ScancodeC) // Ctrl+C
drv.PressKey(ScancodeLShift)                      // hold Shift
drv.ReleaseKey(ScancodeLShift)                    // release Shift
```

---

## Testing

```bash
# Scan code unit tests run on any platform (no driver required)
go test ./cmd/keyboardctl/
```

---

## File structure

```
keyboardctl/
├── driver/                  # C kernel driver (Windows KMDF)
│   ├── driver.h             # data structures, IOCTL code, prototypes
│   ├── driver.c             # DriverEntry, EvtDeviceAdd
│   ├── queue.c              # IOCTL queue and handler
│   ├── inject.c             # keyboard stack connection, event injection
│   ├── sources              # WDK legacy build config
│   └── makefile             # WDK build stub
├── cmd/
│   └── keyboardctl/         # Go CLI + library
│       ├── main.go          # CLI entry point
│       ├── driver.go        # KeyboardDriver: Open/Close/SendKey/sendBatch
│       ├── keyboard.go      # PressKey/ReleaseKey/TapKey/TypeText/PressModifierAndKey
│       ├── scancodes.go     # scan code constants + char map
│       └── scancodes_test.go # platform-independent tests
├── scripts/
│   ├── install.bat          # driver installation
│   └── uninstall.bat        # driver removal
├── Makefile
├── go.mod
└── README.md
```

---

## License

GPL-2.0
