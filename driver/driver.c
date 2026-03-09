/*
 * driver.c - WDF keyboard simulator driver entry point
 *
 * Responsibilities:
 *   - Register the WDF driver object (DriverEntry)
 *   - Create the virtual control device and its symbolic link (EvtDeviceAdd)
 *   - Connect/disconnect from the keyboard class driver stack on D0 transitions
 *
 * Target platform: Windows 10/11 x64 (KMDF 1.31+)
 */

#include "driver.h"

/*
 * DriverEntry - WDF driver entry point
 *
 * Called once by the I/O manager when the driver is loaded.  Registers the
 * WDF driver and its EvtDeviceAdd callback; WDF takes care of the rest.
 */
NTSTATUS
DriverEntry(
    _In_ PDRIVER_OBJECT  DriverObject,
    _In_ PUNICODE_STRING RegistryPath
)
{
    WDF_DRIVER_CONFIG config;
    NTSTATUS          status;

    KdPrint((DRIVER_NAME ": DriverEntry v" DRIVER_VERSION "\n"));

    WDF_DRIVER_CONFIG_INIT(&config, EvtDeviceAdd);

    status = WdfDriverCreate(
        DriverObject,
        RegistryPath,
        WDF_NO_OBJECT_ATTRIBUTES,
        &config,
        WDF_NO_HANDLE
    );

    if (!NT_SUCCESS(status)) {
        KdPrint((DRIVER_NAME ": WdfDriverCreate failed 0x%08X\n", status));
    }

    return status;
}

/*
 * EvtDeviceAdd - called by WDF when the device is added
 *
 * Creates the named control device object, a symbolic link visible in
 * userspace as \\.\KeyboardSimulator, initialises the IOCTL queue, and
 * registers D0 entry/exit callbacks to manage the keyboard stack connection.
 */
NTSTATUS
EvtDeviceAdd(
    _In_    WDFDRIVER       Driver,
    _Inout_ PWDFDEVICE_INIT DeviceInit
)
{
    NTSTATUS              status;
    WDFDEVICE             device;
    WDF_OBJECT_ATTRIBUTES deviceAttributes;
    PDEVICE_CONTEXT       devCtx;
    UNICODE_STRING        deviceName;
    UNICODE_STRING        dosDeviceName;
    WDF_PNPPOWER_EVENT_CALLBACKS pnpPowerCallbacks;

    UNREFERENCED_PARAMETER(Driver);

    /* Name the device so we can create a symbolic link */
    RtlInitUnicodeString(&deviceName, DEVICE_NAME);
    status = WdfDeviceInitAssignName(DeviceInit, &deviceName);
    if (!NT_SUCCESS(status)) {
        KdPrint((DRIVER_NAME ": WdfDeviceInitAssignName failed 0x%08X\n",
                 status));
        return status;
    }

    /*
     * Declare this as a keyboard device so the WDM device object's DeviceType
     * field is FILE_DEVICE_KEYBOARD.  This makes the control device look like
     * a real keyboard port device to code that inspects device type, and
     * matches what a physical i8042prt or HID keyboard port device presents.
     */
    WdfDeviceInitSetDeviceType(DeviceInit, FILE_DEVICE_KEYBOARD);

    /* Exclusive access: only one handle at a time */
    WdfDeviceInitSetExclusive(DeviceInit, TRUE);

    /* Register D0 entry/exit so we can connect/disconnect the keyboard stack */
    WDF_PNPPOWER_EVENT_CALLBACKS_INIT(&pnpPowerCallbacks);
    pnpPowerCallbacks.EvtDeviceD0Entry = EvtDeviceD0Entry;
    pnpPowerCallbacks.EvtDeviceD0Exit  = EvtDeviceD0Exit;
    WdfDeviceInitSetPnpPowerEventCallbacks(DeviceInit, &pnpPowerCallbacks);

    /* Allocate per-device context */
    WDF_OBJECT_ATTRIBUTES_INIT_CONTEXT_TYPE(&deviceAttributes, DEVICE_CONTEXT);

    status = WdfDeviceCreate(&DeviceInit, &deviceAttributes, &device);
    if (!NT_SUCCESS(status)) {
        KdPrint((DRIVER_NAME ": WdfDeviceCreate failed 0x%08X\n", status));
        return status;
    }

    /* Stash the WDFDEVICE handle in our context for later use */
    devCtx = DeviceGetContext(device);
    devCtx->WdfDevice  = device;
    devCtx->Connected  = FALSE;

    /* Create the userspace symbolic link \\.\KeyboardSimulator */
    RtlInitUnicodeString(&dosDeviceName, DOS_DEVICE_NAME);
    status = WdfDeviceCreateSymbolicLink(device, &dosDeviceName);
    if (!NT_SUCCESS(status)) {
        KdPrint((DRIVER_NAME ": WdfDeviceCreateSymbolicLink failed 0x%08X\n",
                 status));
        return status;
    }

    /* Create the IOCTL dispatch queue */
    status = QueueInitialize(device);
    if (!NT_SUCCESS(status)) {
        KdPrint((DRIVER_NAME ": QueueInitialize failed 0x%08X\n", status));
        return status;
    }

    KdPrint((DRIVER_NAME ": device created (%S -> %S)\n",
             DEVICE_NAME, DOS_DEVICE_NAME));
    return STATUS_SUCCESS;
}

/*
 * EvtDeviceD0Entry - device is transitioning into D0 (active) power state
 *
 * Connect to the keyboard class driver so we can inject events.
 */
NTSTATUS
EvtDeviceD0Entry(
    _In_ WDFDEVICE              Device,
    _In_ WDF_POWER_DEVICE_STATE PreviousState
)
{
    PDEVICE_CONTEXT devCtx = DeviceGetContext(Device);
    NTSTATUS        status;

    UNREFERENCED_PARAMETER(PreviousState);

    status = ConnectKeyboardStack(devCtx);
    if (!NT_SUCCESS(status)) {
        KdPrint((DRIVER_NAME ": ConnectKeyboardStack failed 0x%08X\n",
                 status));
        /* Non-fatal: injection will be unavailable but the device still loads */
    }

    return STATUS_SUCCESS;
}

/*
 * EvtDeviceD0Exit - device is leaving D0
 *
 * Disconnect from the keyboard class driver to avoid dangling references.
 */
NTSTATUS
EvtDeviceD0Exit(
    _In_ WDFDEVICE              Device,
    _In_ WDF_POWER_DEVICE_STATE TargetState
)
{
    PDEVICE_CONTEXT devCtx = DeviceGetContext(Device);

    UNREFERENCED_PARAMETER(TargetState);

    DisconnectKeyboardStack(devCtx);
    return STATUS_SUCCESS;
}
