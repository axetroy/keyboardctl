/*
 * driver.h - Keyboard simulator driver definitions
 *
 * Shared header for driver.c, queue.c, and inject.c.
 *
 * Target platform: Windows 10/11 x64
 * Build environment: WDK + KMDF (WDF)
 */

#pragma once

#include <ntddk.h>
#include <wdf.h>
#include <kbdmou.h>     /* KEYBOARD_INPUT_DATA, CONNECT_DATA, keyboard IOCTL codes */

/*
 * Device names
 *
 * DEVICE_NAME    - kernel-mode NT device object path
 * DOS_DEVICE_NAME - userspace-visible symbolic link (\\.\KbdFilter)
 *
 * The names are intentionally generic so the device is not trivially
 * identified as a software input simulator by tools that enumerate kernel
 * objects (e.g. WinObj).
 */
#define DEVICE_NAME      L"\\Device\\KbdFilter"
#define DOS_DEVICE_NAME  L"\\DosDevices\\KbdFilter"
#define DRIVER_NAME      "keyboardctl"
#define DRIVER_VERSION   "1.0.0"

/*
 * IOCTL_KEYBOARD_INJECT
 *
 * Input buffer:  array of KEYBOARD_INPUT_DATA (one or more entries)
 * Output buffer: none
 *
 * CTL_CODE(FILE_DEVICE_KEYBOARD=0x0B, Function=0x800,
 *          METHOD_BUFFERED=0, FILE_WRITE_ACCESS=2)
 * = (0x0B << 16) | (0x02 << 14) | (0x800 << 2) | 0
 * = 0x000BA000
 */
#define IOCTL_KEYBOARD_INJECT \
    CTL_CODE(FILE_DEVICE_KEYBOARD, 0x800, METHOD_BUFFERED, FILE_WRITE_ACCESS)

/* Maximum KEYBOARD_INPUT_DATA entries accepted per IOCTL call */
#define KEYBOARDSIM_MAX_EVENTS  64

/*
 * DEVICE_CONTEXT - per-device WDF context
 *
 * Allocated once per device by WDF and retrieved with DeviceGetContext().
 */
typedef struct _DEVICE_CONTEXT {
    WDFDEVICE     WdfDevice;

    /*
     * UpperConnectData holds the ClassService callback obtained when the
     * keyboard class driver connects to this filter.  We call it to inject
     * simulated key events into the class driver's input stream.
     */
    CONNECT_DATA  UpperConnectData;

    /* TRUE once ConnectKeyboardStack() has successfully connected */
    BOOLEAN       Connected;
} DEVICE_CONTEXT, *PDEVICE_CONTEXT;

WDF_DECLARE_CONTEXT_TYPE_WITH_NAME(DEVICE_CONTEXT, DeviceGetContext)

/* -------------------------------------------------------------------------
 * Function prototypes
 * ---------------------------------------------------------------------- */

/* driver.c */
DRIVER_INITIALIZE DriverEntry;

EVT_WDF_DRIVER_DEVICE_ADD   EvtDeviceAdd;
EVT_WDF_DEVICE_D0_ENTRY     EvtDeviceD0Entry;
EVT_WDF_DEVICE_D0_EXIT      EvtDeviceD0Exit;

/* queue.c */
NTSTATUS
QueueInitialize(
    _In_ WDFDEVICE Device
);

EVT_WDF_IO_QUEUE_IO_DEVICE_CONTROL EvtIoDeviceControl;

/* inject.c */
NTSTATUS
ConnectKeyboardStack(
    _In_ PDEVICE_CONTEXT DevCtx
);

VOID
DisconnectKeyboardStack(
    _In_ PDEVICE_CONTEXT DevCtx
);

NTSTATUS
InjectKeyboardEvents(
    _In_ PDEVICE_CONTEXT     DevCtx,
    _In_ PKEYBOARD_INPUT_DATA Events,
    _In_ ULONG               EventCount
);
