/*
 * driver.c - WDF keyboard filter driver entry point
 *
 * Responsibilities:
 *   - DriverEntry: register the WDF driver and create the control device that
 *     user mode talks to (\\.\KbdFilter).  A filter driver has no device of its
 *     own, so the control device is created here rather than in EvtDeviceAdd.
 *   - EvtDeviceAdd: called once per keyboard; create the filter device that
 *     attaches below kbdclass and hook IOCTL_INTERNAL_KEYBOARD_CONNECT.
 *   - EvtDeviceCleanup: drop the injection target if this device owned it.
 *
 * Target platform: Windows 10/11 x64 (KMDF 1.31+)
 */

#include "driver.h"

GLOBAL_CONTEXT g_Global;

/*
 * DriverEntry - WDF driver entry point
 */
NTSTATUS
DriverEntry(
    _In_ PDRIVER_OBJECT  DriverObject,
    _In_ PUNICODE_STRING RegistryPath
)
{
    WDF_DRIVER_CONFIG config;
    WDFDRIVER         driver;
    NTSTATUS          status;

    KdPrint((DRIVER_NAME ": DriverEntry v" DRIVER_VERSION "\n"));

    KeInitializeSpinLock(&g_Global.Lock);
    g_Global.Target = NULL;
    g_Global.KeyboardCount = 0;

    WDF_DRIVER_CONFIG_INIT(&config, EvtDeviceAdd);

    status = WdfDriverCreate(
        DriverObject,
        RegistryPath,
        WDF_NO_OBJECT_ATTRIBUTES,
        &config,
        &driver
    );
    if (!NT_SUCCESS(status)) {
        KdPrint((DRIVER_NAME ": WdfDriverCreate failed 0x%08X\n", status));
        return status;
    }

    /*
     * Create the control device up front so the CLI can open \\.\KbdFilter
     * whether or not a keyboard is present.
     */
    status = CreateControlDevice(driver);
    if (!NT_SUCCESS(status)) {
        KdPrint((DRIVER_NAME ": CreateControlDevice failed 0x%08X\n", status));
        return status;
    }

    return STATUS_SUCCESS;
}

/*
 * CreateControlDevice - create the \\.\KbdFilter control device
 *
 * The control device is not part of any device stack: the I/O manager delivers
 * requests for it straight to us.  It is the sideband channel the user-mode
 * controller uses to submit keystrokes.
 */
NTSTATUS
CreateControlDevice(
    _In_ WDFDRIVER Driver
)
{
    DECLARE_CONST_UNICODE_STRING(deviceName, CONTROL_DEVICE_NAME);
    DECLARE_CONST_UNICODE_STRING(symbolicLink, CONTROL_SYMBOLIC_LINK);

    PWDFDEVICE_INIT       deviceInit = NULL;
    WDFDEVICE             device     = NULL;
    WDF_OBJECT_ATTRIBUTES attributes;
    NTSTATUS              status;

    PAGED_CODE();

    /*
     * SDDL_DEVOBJ_SYS_ALL_ADM_ALL grants the device to Administrators and
     * SYSTEM only, so an unprivileged process cannot inject keystrokes.
     */
    deviceInit = WdfControlDeviceInitAllocate(Driver, &SDDL_DEVOBJ_SYS_ALL_ADM_ALL);
    if (deviceInit == NULL) {
        return STATUS_INSUFFICIENT_RESOURCES;
    }

    status = WdfDeviceInitAssignName(deviceInit, &deviceName);
    if (!NT_SUCCESS(status)) {
        KdPrint((DRIVER_NAME ": WdfDeviceInitAssignName failed 0x%08X\n", status));
        goto cleanup;
    }

    WdfDeviceInitSetDeviceType(deviceInit, FILE_DEVICE_KEYBOARD);

    WDF_OBJECT_ATTRIBUTES_INIT(&attributes);

    status = WdfDeviceCreate(&deviceInit, &attributes, &device);
    if (!NT_SUCCESS(status)) {
        KdPrint((DRIVER_NAME ": WdfDeviceCreate failed 0x%08X\n", status));
        goto cleanup;
    }
    /*
     * WdfDeviceCreate consumes deviceInit on success.  WdfDeviceInitFree must
     * only ever be called on failure, so drop our reference to it here.
     */
    deviceInit = NULL;

    status = ControlQueueInitialize(device);
    if (!NT_SUCCESS(status)) {
        goto cleanup;
    }

    status = WdfDeviceCreateSymbolicLink(device, &symbolicLink);
    if (!NT_SUCCESS(status)) {
        KdPrint((DRIVER_NAME ": WdfDeviceCreateSymbolicLink failed 0x%08X\n", status));
        goto cleanup;
    }

    WdfControlFinishInitializing(device);
    device = NULL;

    KdPrint((DRIVER_NAME ": control device ready\n"));
    status = STATUS_SUCCESS;

cleanup:
    if (device != NULL) {
        WdfObjectDelete(device);
    }
    if (deviceInit != NULL) {
        WdfDeviceInitFree(deviceInit);
    }
    return status;
}

/*
 * EvtDeviceAdd - a keyboard has been detected; create our filter device
 *
 * WdfFdoInitSetFilter makes the framework build the device object as a filter
 * and attach it to the stack below kbdclass (the installer places us ahead of
 * kbdclass in the class UpperFilters list).
 */
NTSTATUS
EvtDeviceAdd(
    _In_    WDFDRIVER       Driver,
    _Inout_ PWDFDEVICE_INIT DeviceInit
)
{
    NTSTATUS              status;
    WDFDEVICE             device;
    WDF_OBJECT_ATTRIBUTES attributes;
    WDF_IO_QUEUE_CONFIG   queueConfig;
    PDEVICE_EXTENSION     devCtx;

    UNREFERENCED_PARAMETER(Driver);

    PAGED_CODE();

    WdfFdoInitSetFilter(DeviceInit);

    WDF_OBJECT_ATTRIBUTES_INIT_CONTEXT_TYPE(&attributes, DEVICE_EXTENSION);
    attributes.EvtCleanupCallback = EvtDeviceCleanup;

    status = WdfDeviceCreate(&DeviceInit, &attributes, &device);
    if (!NT_SUCCESS(status)) {
        KdPrint((DRIVER_NAME ": WdfDeviceCreate failed 0x%08X\n", status));
        return status;
    }

    devCtx = DeviceGetContext(device);
    devCtx->WdfDevice = device;
    devCtx->Connected = FALSE;
    RtlZeroMemory(&devCtx->UpperConnectData, sizeof(devCtx->UpperConnectData));

    /*
     * Parallel dispatch, not sequential: the port driver can send a request to
     * the top of the stack and wait for it while an earlier IOCTL is still
     * outstanding.  A sequential queue would deadlock in that case.
     */
    WDF_IO_QUEUE_CONFIG_INIT_DEFAULT_QUEUE(&queueConfig, WdfIoQueueDispatchParallel);
    queueConfig.EvtIoInternalDeviceControl = EvtIoInternalDeviceControl;

    status = WdfIoQueueCreate(
        device,
        &queueConfig,
        WDF_NO_OBJECT_ATTRIBUTES,
        WDF_NO_HANDLE
    );
    if (!NT_SUCCESS(status)) {
        KdPrint((DRIVER_NAME ": WdfIoQueueCreate failed 0x%08X\n", status));
        return status;
    }

    return STATUS_SUCCESS;
}

/*
 * EvtDeviceCleanup - the filter device is being torn down
 *
 * If this device is the current injection target, clear it so a later
 * InjectKeyboardEvents() does not touch freed memory.
 */
VOID
EvtDeviceCleanup(
    _In_ WDFOBJECT Device
)
{
    PDEVICE_EXTENSION devCtx = DeviceGetContext((WDFDEVICE)Device);
    KIRQL             oldIrql;

    KeAcquireSpinLock(&g_Global.Lock, &oldIrql);
    if (g_Global.Target == devCtx) {
        g_Global.Target = NULL;
    }
    KeReleaseSpinLock(&g_Global.Lock, oldIrql);
}
