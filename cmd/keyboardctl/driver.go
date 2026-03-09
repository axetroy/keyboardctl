//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

// devicePath is the Win32 path of the symbolic link created by the driver.
const devicePath = `\\.\KeyboardSimulator`

/*
 * IOCTL_KEYBOARD_INJECT
 *
 * CTL_CODE(FILE_DEVICE_KEYBOARD=0x0B, Function=0x800,
 *          METHOD_BUFFERED=0, FILE_WRITE_ACCESS=2)
 * = (0x0B << 16) | (0x02 << 14) | (0x800 << 2) | 0
 * = 0x000BA000
 */
const ioctlKeyboardInject uint32 = 0x000BA000

/*
 * KEY_* flag constants for KEYBOARD_INPUT_DATA.Flags.
 * Must stay in sync with ntddkbd.h / kbdmou.h.
 */
const (
	keyFlagMake  uint16 = 0x0000 // key down
	keyFlagBreak uint16 = 0x0001 // key up
	keyFlagE0    uint16 = 0x0002 // extended key (E0 prefix)
	keyFlagE1    uint16 = 0x0004 // extended key (E1 prefix, Pause only)
)

// KeyboardInputData mirrors the Windows KEYBOARD_INPUT_DATA structure from
// ntddkbd.h.  The layout must match exactly — 12 bytes, no implicit padding.
//
//	typedef struct _KEYBOARD_INPUT_DATA {
//	    USHORT UnitId;
//	    USHORT MakeCode;
//	    USHORT Flags;
//	    USHORT Reserved;
//	    ULONG  ExtraInformation;
//	} KEYBOARD_INPUT_DATA;
type KeyboardInputData struct {
	UnitId           uint16
	MakeCode         uint16
	Flags            uint16
	Reserved         uint16
	ExtraInformation uint32
}

// KeyboardDriver holds an open handle to the KeyboardSimulator device.
type KeyboardDriver struct {
	handle syscall.Handle
}

// Open opens a handle to the KeyboardSimulator device.
// The caller must call Close when done.
func Open() (*KeyboardDriver, error) {
	path, err := syscall.UTF16PtrFromString(devicePath)
	if err != nil {
		return nil, fmt.Errorf("driver: encode device path: %w", err)
	}

	handle, err := syscall.CreateFile(
		path,
		syscall.GENERIC_WRITE,
		0,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("driver: open %s: %w", devicePath, err)
	}

	return &KeyboardDriver{handle: handle}, nil
}

// Close closes the device handle.
func (d *KeyboardDriver) Close() error {
	return syscall.CloseHandle(d.handle)
}

// SendKey sends a single raw KEYBOARD_INPUT_DATA entry to the driver.
//
//   - makeCode: PC/AT scan code (without the E0 prefix byte)
//   - flags:    combination of keyFlagMake/keyFlagBreak/keyFlagE0/keyFlagE1
func (d *KeyboardDriver) SendKey(makeCode uint16, flags uint16) error {
	data := KeyboardInputData{
		UnitId:   0,
		MakeCode: makeCode,
		Flags:    flags,
	}

	var bytesReturned uint32
	err := syscall.DeviceIoControl(
		d.handle,
		ioctlKeyboardInject,
		(*byte)(unsafe.Pointer(&data)),
		uint32(unsafe.Sizeof(data)),
		nil,
		0,
		&bytesReturned,
		nil,
	)
	if err != nil {
		return fmt.Errorf("driver: DeviceIoControl: %w", err)
	}
	return nil
}

// sendBatch sends multiple KEYBOARD_INPUT_DATA entries in a single IOCTL call.
func (d *KeyboardDriver) sendBatch(events []KeyboardInputData) error {
	if len(events) == 0 {
		return nil
	}

	size := uint32(len(events)) * uint32(unsafe.Sizeof(events[0]))
	var bytesReturned uint32
	err := syscall.DeviceIoControl(
		d.handle,
		ioctlKeyboardInject,
		(*byte)(unsafe.Pointer(&events[0])),
		size,
		nil,
		0,
		&bytesReturned,
		nil,
	)
	if err != nil {
		return fmt.Errorf("driver: DeviceIoControl (batch %d): %w", len(events), err)
	}
	return nil
}

// scToMakeFlags converts a ScanCode into the (makeCode, baseFlags) pair needed
// for a KEYBOARD_INPUT_DATA.  The key-down/up flag must be OR-ed by the caller.
func scToMakeFlags(sc ScanCode) (makeCode uint16, baseFlags uint16) {
	if sc&ScanCodeExtended != 0 {
		return uint16(sc &^ ScanCodeExtended), keyFlagE0
	}
	return uint16(sc), 0
}
