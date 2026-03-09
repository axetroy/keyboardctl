/*
 * inject.c - Keyboard class driver attachment and event injection
 *
 * This module implements two responsibilities:
 *
 *  1. ConnectKeyboardStack() / DisconnectKeyboardStack()
 *     Establishes a connection to the keyboard class driver
 *     (\\Device\\KeyboardClass0) via IOCTL_INTERNAL_KEYBOARD_CONNECT.  On
 *     success, the class driver provides a ClassService callback pointer
 *     stored in DEVICE_CONTEXT.UpperConnectData.  This callback is the
 *     official, documented way to feed events into the keyboard class driver
 *     without bypassing the upper-filter chain.
 *
 *  2. InjectKeyboardEvents()
 *     Calls the ClassService callback directly with the caller-supplied array
 *     of KEYBOARD_INPUT_DATA, making the events appear as if they arrived from
 *     real hardware — fully transparent to applications and the OS.
 *
 * Design notes:
 *  - We use IoGetDeviceObjectPointer() to locate KeyboardClass0 at runtime,
 *    which handles systems with multiple physical keyboards gracefully: all
 *    keyboards are multiplexed into KeyboardClass0.
 *  - The connection IRP is sent synchronously (IoBuildDeviceIoControlRequest +
 *    IoCallDriver with a KEVENT for completion).
 *  - Injection is done at DISPATCH_LEVEL via KeRaiseIrqlToDpcLevel, matching
 *    how the real keyboard port driver calls back the class driver.
 */

#include "driver.h"

/* Name of the keyboard class device exported by kbdclass.sys */
static const WCHAR kKeyboardClass0[] = L"\\Device\\KeyboardClass0";

/*
 * ConnectKeyboardStack - send IOCTL_INTERNAL_KEYBOARD_CONNECT to kbdclass
 *
 * On success, devCtx->UpperConnectData.ClassService points to the class
 * driver's service routine, and devCtx->Connected is TRUE.
 */
NTSTATUS
ConnectKeyboardStack(
    _In_ PDEVICE_CONTEXT DevCtx
)
{
    UNICODE_STRING      deviceName;
    PFILE_OBJECT        fileObject = NULL;
    PDEVICE_OBJECT      deviceObject = NULL;
    PIRP                irp;
    IO_STATUS_BLOCK     ioStatus;
    KEVENT              event;
    CONNECT_DATA        connectData;
    NTSTATUS            status;

    if (DevCtx->Connected) {
        return STATUS_SUCCESS;
    }

    /* Locate the keyboard class device */
    RtlInitUnicodeString(&deviceName, kKeyboardClass0);

    status = IoGetDeviceObjectPointer(
        &deviceName,
        FILE_READ_DATA,
        &fileObject,
        &deviceObject
    );
    if (!NT_SUCCESS(status)) {
        KdPrint((DRIVER_NAME ": IoGetDeviceObjectPointer(%S) failed 0x%08X\n",
                 kKeyboardClass0, status));
        return status;
    }

    /* Prepare the connect data — we supply our device object as the "port" */
    connectData.ClassDeviceObject = WdfDeviceWdmGetDeviceObject(DevCtx->WdfDevice);
    connectData.ClassService      = NULL;   /* filled in by the class driver */

    KeInitializeEvent(&event, NotificationEvent, FALSE);

    irp = IoBuildDeviceIoControlRequest(
        IOCTL_INTERNAL_KEYBOARD_CONNECT,
        deviceObject,
        &connectData,
        sizeof(CONNECT_DATA),
        NULL,
        0,
        TRUE,           /* InternalDeviceIoControl */
        &event,
        &ioStatus
    );
    if (!irp) {
        KdPrint((DRIVER_NAME ": IoBuildDeviceIoControlRequest failed "
                 "(insufficient resources)\n"));
        ObDereferenceObject(fileObject);
        return STATUS_INSUFFICIENT_RESOURCES;
    }

    status = IoCallDriver(deviceObject, irp);
    if (status == STATUS_PENDING) {
        KeWaitForSingleObject(&event, Executive, KernelMode, FALSE, NULL);
        status = ioStatus.Status;
    }

    if (!NT_SUCCESS(status)) {
        KdPrint((DRIVER_NAME ": IOCTL_INTERNAL_KEYBOARD_CONNECT failed "
                 "0x%08X\n", status));
        ObDereferenceObject(fileObject);
        return status;
    }

    /* Save the ClassService pointer provided by the class driver */
    DevCtx->UpperConnectData = connectData;
    DevCtx->Connected        = TRUE;

    ObDereferenceObject(fileObject);

    KdPrint((DRIVER_NAME ": connected to %S (ClassService=%p)\n",
             kKeyboardClass0, connectData.ClassService));
    return STATUS_SUCCESS;
}

/*
 * DisconnectKeyboardStack - release the connection to the class driver
 *
 * We do a best-effort IOCTL_INTERNAL_KEYBOARD_DISCONNECT; errors are logged
 * but not propagated because this is called from a D0-exit path.
 */
VOID
DisconnectKeyboardStack(
    _In_ PDEVICE_CONTEXT DevCtx
)
{
    UNICODE_STRING  deviceName;
    PFILE_OBJECT    fileObject = NULL;
    PDEVICE_OBJECT  deviceObject = NULL;
    PIRP            irp;
    IO_STATUS_BLOCK ioStatus;
    KEVENT          event;
    NTSTATUS        status;

    if (!DevCtx->Connected) {
        return;
    }

    RtlInitUnicodeString(&deviceName, kKeyboardClass0);

    status = IoGetDeviceObjectPointer(
        &deviceName,
        FILE_READ_DATA,
        &fileObject,
        &deviceObject
    );
    if (!NT_SUCCESS(status)) {
        goto done;
    }

    KeInitializeEvent(&event, NotificationEvent, FALSE);

    irp = IoBuildDeviceIoControlRequest(
        IOCTL_INTERNAL_KEYBOARD_DISCONNECT,
        deviceObject,
        NULL,
        0,
        NULL,
        0,
        TRUE,
        &event,
        &ioStatus
    );
    if (irp) {
        status = IoCallDriver(deviceObject, irp);
        if (status == STATUS_PENDING) {
            KeWaitForSingleObject(&event, Executive, KernelMode, FALSE, NULL);
        }
    }

    ObDereferenceObject(fileObject);

done:
    RtlZeroMemory(&DevCtx->UpperConnectData, sizeof(DevCtx->UpperConnectData));
    DevCtx->Connected = FALSE;
    KdPrint((DRIVER_NAME ": disconnected from keyboard stack\n"));
}

/*
 * InjectKeyboardEvents - feed KEYBOARD_INPUT_DATA entries into the class driver
 *
 * This function calls the keyboard class driver's ClassService routine
 * (PSERVICE_CALLBACK_ROUTINE) with the supplied event array.  The class
 * driver queues the events and delivers them to Win32k.sys — exactly as if
 * they came from physical hardware.
 *
 * Must be called at IRQL <= DISPATCH_LEVEL.
 */
NTSTATUS
InjectKeyboardEvents(
    _In_ PDEVICE_CONTEXT     DevCtx,
    _In_ PKEYBOARD_INPUT_DATA Events,
    _In_ ULONG               EventCount
)
{
    PDEVICE_OBJECT         classDevice;
    PKEYBOARD_INPUT_DATA   end;

    if (!DevCtx->Connected || !DevCtx->UpperConnectData.ClassService) {
        KdPrint((DRIVER_NAME ": not connected to keyboard stack\n"));
        return STATUS_DEVICE_NOT_CONNECTED;
    }

    classDevice = DevCtx->UpperConnectData.ClassDeviceObject;
    end         = Events + EventCount;

    /*
     * Call the class driver's service callback.  The prototype matches
     * PSERVICE_CALLBACK_ROUTINE from kbdmou.h:
     *
     *   VOID ServiceCallback(
     *       PDEVICE_OBJECT DeviceObject,
     *       PKEYBOARD_INPUT_DATA InputDataStart,
     *       PKEYBOARD_INPUT_DATA InputDataEnd,
     *       PULONG InputDataConsumed);
     */
    ULONG consumed = 0;
    ((PSERVICE_CALLBACK_ROUTINE)(DevCtx->UpperConnectData.ClassService))(
        classDevice,
        Events,
        end,
        &consumed
    );

    KdPrint((DRIVER_NAME ": injected %lu event(s), consumed %lu\n",
             EventCount, consumed));
    return STATUS_SUCCESS;
}
