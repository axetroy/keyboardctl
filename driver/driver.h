/*
 * driver.h - keyboardctl keyboard filter driver definitions
 *
 * Shared header for driver.c, inject.c and queue.c.
 *
 * Architecture
 * ------------
 * keyboardctl is a KMDF *upper filter* driver for the Keyboard device class.
 * The installer puts it in the class "UpperFilters" list ahead of kbdclass, so
 * each keyboard's stack becomes:
 *
 *     kbdclass -> keyboardctl -> <port driver: i8042prt / kbdhid> -> <bus>
 *
 * Sitting below kbdclass is what makes injection possible: kbdclass sends
 * IOCTL_INTERNAL_KEYBOARD_CONNECT *down* to the port driver, and that request
 * carries kbdclass's own KeyboardClassServiceCallback.  We keep that callback
 * and hand the port driver ours instead, so real keystrokes reach us first.
 * Injection reuses the very same callback: we give synthetic
 * KEYBOARD_INPUT_DATA to kbdclass, which queues it for Win32k exactly as if the
 * port driver had reported it.
 *
 * The user-mode controller talks to a control device (created in DriverEntry,
 * because a filter driver has no device of its own) reached through the
 * symbolic link \\.\KbdFilter.
 *
 * Target platform: Windows 10/11 x64
 * Build environment: WDK + KMDF (WDF)
 */

#pragma once

#include <ntddk.h>
#include <wdf.h>
#include <kbdmou.h>     /* CONNECT_DATA, KEYBOARD_INPUT_DATA, keyboard IOCTL codes */
#include <wdmsec.h>     /* SDDL_DEVOBJ_* security descriptors */

/*
 * Device names
 *
 * CONTROL_DEVICE_NAME   - kernel-mode NT path of the control device
 * CONTROL_SYMBOLIC_LINK - user-mode path (\\.\KbdFilter) the CLI opens
 */
#define CONTROL_DEVICE_NAME    L"\\Device\\KbdFilter"
#define CONTROL_SYMBOLIC_LINK  L"\\DosDevices\\KbdFilter"

#define DRIVER_NAME            "keyboardctl"
#ifndef DRIVER_VERSION
#define DRIVER_VERSION         "1.0.0"   /* overridden at build time (driver.vcxproj) */
#endif

/*
 * IOCTL_KEYBOARD_INJECT
 *
 * Input buffer:  array of KEYBOARD_INPUT_DATA (one or more entries)
 * Output buffer: none
 *
 * CTL_CODE(FILE_DEVICE_KEYBOARD=0x0B, Function=0x800,
 *          METHOD_BUFFERED=0, FILE_WRITE_ACCESS=2) = 0x000BA000
 */
#define IOCTL_KEYBOARD_INJECT \
    CTL_CODE(FILE_DEVICE_KEYBOARD, 0x800, METHOD_BUFFERED, FILE_WRITE_ACCESS)

/*
 * IOCTL_KEYBOARD_QUERY_STATE
 *
 * Input buffer:  none
 * Output buffer: KEYBOARDCTL_STATE
 *
 * CTL_CODE(FILE_DEVICE_KEYBOARD=0x0B, Function=0x801,
 *          METHOD_BUFFERED=0, FILE_READ_ACCESS=1) = 0x000B6004
 */
#define IOCTL_KEYBOARD_QUERY_STATE \
    CTL_CODE(FILE_DEVICE_KEYBOARD, 0x801, METHOD_BUFFERED, FILE_READ_ACCESS)

/* Maximum KEYBOARD_INPUT_DATA entries accepted per IOCTL call */
#define KEYBOARDSIM_MAX_EVENTS  64

/*
 * KEYBOARDCTL_STATE - reply to IOCTL_KEYBOARD_QUERY_STATE
 *
 * Lets the CLI tell "the driver is loaded" apart from "a keyboard is connected
 * and injection will actually reach the system".  The device can be opened as
 * soon as the service loads, but injection only works once kbdclass has sent
 * IOCTL_INTERNAL_KEYBOARD_CONNECT down through us, which needs the filter to be
 * correctly positioned in the keyboard stack.
 */
typedef struct _KEYBOARDCTL_STATE {
    ULONG Connected;      /* 1 if a keyboard is connected; 0 otherwise */
    ULONG KeyboardCount;  /* how many keyboards have connected to this driver */
} KEYBOARDCTL_STATE, *PKEYBOARDCTL_STATE;

/*
 * DEVICE_EXTENSION - per-filter-device WDF context
 */
typedef struct _DEVICE_EXTENSION {
    WDFDEVICE     WdfDevice;

    /*
     * The upward connection captured from kbdclass: its device object and
     * service callback.  Injection calls ClassService directly to hand events
     * to the class driver.
     */
    CONNECT_DATA  UpperConnectData;

    BOOLEAN       Connected;
} DEVICE_EXTENSION, *PDEVICE_EXTENSION;

WDF_DECLARE_CONTEXT_TYPE_WITH_NAME(DEVICE_EXTENSION, DeviceGetContext)

/*
 * GLOBAL_CONTEXT - the injection target
 *
 * Every keyboard gets its own filter device, but a scan code injected through
 * any one of them reaches the same Win32k input queue, so we inject through a
 * single instance: the first one that connects.  The pointer is published by
 * the CONNECT handler and cleared by DISCONNECT and by device cleanup; every
 * access is serialised by the spin lock.
 */
typedef struct _GLOBAL_CONTEXT {
    KSPIN_LOCK        Lock;
    PDEVICE_EXTENSION Target;   /* connected filter device, or NULL */
    ULONG             KeyboardCount; /* keyboards that have connected so far */
} GLOBAL_CONTEXT, *PGLOBAL_CONTEXT;

extern GLOBAL_CONTEXT g_Global;

/* -------------------------------------------------------------------------
 * Function prototypes
 * ---------------------------------------------------------------------- */

/* driver.c */
DRIVER_INITIALIZE DriverEntry;

EVT_WDF_DRIVER_DEVICE_ADD        EvtDeviceAdd;
EVT_WDF_OBJECT_CONTEXT_CLEANUP   EvtDeviceCleanup;

NTSTATUS
CreateControlDevice(
    _In_ WDFDRIVER Driver
);

/* inject.c */
EVT_WDF_IO_QUEUE_IO_INTERNAL_DEVICE_CONTROL EvtIoInternalDeviceControl;

VOID
ServiceCallback(
    _In_ PDEVICE_OBJECT       DeviceObject,
    _In_ PKEYBOARD_INPUT_DATA InputDataStart,
    _In_ PKEYBOARD_INPUT_DATA InputDataEnd,
    _Inout_ PULONG            InputDataConsumed
);

NTSTATUS
InjectKeyboardEvents(
    _In_reads_(EventCount) PKEYBOARD_INPUT_DATA Events,
    _In_ ULONG EventCount
);

/* queue.c */
NTSTATUS
ControlQueueInitialize(
    _In_ WDFDEVICE Device
);

EVT_WDF_IO_QUEUE_IO_DEVICE_CONTROL EvtIoDeviceControl;
