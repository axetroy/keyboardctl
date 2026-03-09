# keyboardctl

Software-only keyboard input simulator — no physical hardware required.
Applications receive the injected keystrokes exactly as if a real keyboard had
been pressed.

---

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                  User Applications                  │
│          (terminal, browser, text editor, …)        │
└─────────────────────┬───────────────────────────────┘
                      │  standard input events
                      ▼
┌─────────────────────────────────────────────────────┐
│            Linux Input Subsystem (kernel)           │
│                /dev/input/eventN                    │
└─────────────────────┬───────────────────────────────┘
                      │  input_event()
                      ▼
┌─────────────────────────────────────────────────────┐
│        keyboardctl kernel module (C)                │
│  • registers a virtual keyboard input device        │
│  • exposes /dev/keyboardctl character device        │
└─────────────────────┬───────────────────────────────┘
                      │  write(keyboardctl_event structs)
                      ▼
┌─────────────────────────────────────────────────────┐
│         keyboardctl CLI / Go library                │
│   type · press · down · up · combo                  │
└─────────────────────────────────────────────────────┘
```

### Kernel module (`driver/keyboardctl.c`)

* Creates a virtual keyboard `input_dev` and registers it with the Linux input
  subsystem — applications cannot distinguish it from real hardware.
* Exposes a character device `/dev/keyboardctl`.
* `write()` calls on the device accept `struct keyboardctl_event` values and
  forward them to `input_event()`, making them visible to all listeners.

### Go userspace (`pkg/keyboard`, `cmd/keyboardctl`)

* Opens `/dev/keyboardctl` and writes little-endian `event` structs.
* No CGO, no external dependencies — pure Go with the standard library.
* `keyboard.Keyboard` provides `Press`, `KeyDown`, `KeyUp`, `Combo`, and
  `Type` (US-ASCII layout, automatic Shift for uppercase/symbols).

---

## Requirements

| Component | Requirement |
|-----------|-------------|
| OS        | Linux (kernel ≥ 4.15) |
| Kernel headers | `linux-headers-$(uname -r)` |
| Go        | ≥ 1.21 |
| C toolchain | GCC + Make |

---

## Build

```bash
# Build everything (Go binary + kernel module)
make all

# Build only the Go CLI
make go

# Build only the kernel module
make driver
```

---

## Quick start

```bash
# 1. Build
make all

# 2. Load the kernel module (requires root)
sudo make load
# or: sudo insmod driver/keyboardctl.ko

# 3. Allow your user to write to the device (or run as root)
sudo chmod a+w /dev/keyboardctl

# 4. Type some text
./keyboardctl type "Hello, World!"

# 5. Press a single key
./keyboardctl press enter

# 6. Key combination (Ctrl+C)
./keyboardctl combo ctrl c

# 7. Unload the module when done
sudo make unload
```

---

## CLI reference

```
keyboardctl type  <text>          Type a string (US keyboard layout)
keyboardctl press <key>           Press and release a key
keyboardctl down  <key>           Send key-down only
keyboardctl up    <key>           Send key-up only
keyboardctl combo <key> [key…]    Press keys simultaneously
```

Set `KEYBOARDCTL_DEVICE` to override the default `/dev/keyboardctl` path.

---

## Go library

```go
import "github.com/axetroy/keyboardctl/pkg/keyboard"

kb, err := keyboard.Open()
if err != nil {
    log.Fatal(err)
}
defer kb.Close()

kb.Type("Hello!")
kb.Press(keyboard.KeyEnter)
kb.Combo(keyboard.KeyLeftCtrl, keyboard.KeyC)
```

---

## Testing

```bash
# Run Go unit tests (no kernel module required)
make test
# or: go test ./...
```

---

## Protocol

The Go library writes `struct keyboardctl_event` values (defined in
`driver/keyboardctl.h`) to `/dev/keyboardctl`:

```c
struct keyboardctl_event {
    __u16 type;   // KBCTL_EV_SYN (0) or KBCTL_EV_KEY (1)
    __u16 code;   // key code (linux/input-event-codes.h)
    __s32 value;  // 0=release, 1=press, 2=repeat
};
```

Always follow key events with an `EV_SYN / SYN_REPORT` event to flush the
batch to listening applications.

---

## License

GPL-2.0
