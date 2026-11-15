/*
 * inject.c - keyboard connect interception and event injection
 *
 * This module implements the two halves of the filter:
 *
 *  1. EvtIoInternalDeviceControl()
 *     Handles IOCTL_INTERNAL_KEYBOARD_CONNECT, which kbdclass sends *down* the
 *     stack to bind a port driver as its input source.  The request carries
 *     kbdclass's own KeyboardClassServiceCallback in CONNECT_DATA.ClassService.
 *     We save it, then overwrite the request with our own device object and
 *     callback, and forward it to the port driver.  From then on every
 *     keystroke the port driver reports reaches ServiceCallback() first.
 *
 *  2. InjectKeyboardEvents()
 *     Calls the saved kbdclass callback with caller-supplied KEYBOARD_INPUT_DATA,
 *     so synthetic keystrokes enter kbdclass exactly as real hardware events do.
 *
 * Why a filter, and not a control device that "connects" to kbdclass:
 * IOCTL_INTERNAL_KEYBOARD_CONNECT flows downward.  kbdclass is its sender and
 * never returns a ClassService to anyone who sends it upward, so the only place
 * that can capture the callback is a driver sitting between kbdclass and the
 * port driver.
 */

#include "driver.h"

/*
 * EvtIoInternalDeviceControl - internal device control dispatch
 *
 * Only IOCTL_INTERNAL_KEYBOARD_CONNECT is handled specially; everything else is
 * forwarded down untouched, including IOCTL_INTERNAL_KEYBOARD_DISCONNECT.
 */
VOID
EvtIoInternalDeviceControl(
    _In_ WDFQUEUE Queue,
    _In_ WDFREQUEST Request,
    _In_ size_t OutputBufferLength,
    _In_ size_t InputBufferLength,
    _In_ ULONG IoControlCode
)
{
    PDEVICE_EXTENSION         devCtx;
    WDFDEVICE                 device;
    PCONNECT_DATA             connectData = NULL;
    NTSTATUS                  status = STATUS_SUCCESS;
    size_t                    length;
    WDF_REQUEST_SEND_OPTIONS  options;
    KIRQL                     oldIrql;

    UNREFERENCED_PARAMETER(OutputBufferLength);
    UNREFERENCED_PARAMETER(InputBufferLength);

    device = WdfIoQueueGetDevice(Queue);
    devCtx = DeviceGetContext(device);

    switch (IoControlCode) {

    case IOCTL_INTERNAL_KEYBOARD_CONNECT:
        /*
         * Only one class driver may be connected at a time.  A second CONNECT
         * (without an intervening DISCONNECT) is a sharing violation, exactly
         * as the port drivers report it.
         */
        if (devCtx->Connected) {
            status = STATUS_SHARING_VIOLATION;
            break;
        }

        status = WdfRequestRetrieveInputBuffer(
            Request,
            sizeof(CONNECT_DATA),
            (PVOID *)&connectData,
            &length
        );
        if (!NT_SUCCESS(status)) {
            KdPrint((DRIVER_NAME ": retrieve CONNECT_DATA failed 0x%08X\n", status));
            break;
        }

        /* Remember kbdclass's device object and callback so we can call back up. */
        devCtx->UpperConnectData = *connectData;

        /*
         * Hand the port driver our own device object and callback.  It will
         * report keystrokes to ServiceCallback() instead of directly to
         * kbdclass.
         */
        connectData->ClassDeviceObject = WdfDeviceWdmGetDeviceObject(device);
#pragma warning(disable:4152)  /* function/data pointer conversion */
        connectData->ClassService = (PVOID)ServiceCallback;
#pragma warning(default:4152)

        devCtx->Connected = TRUE;

        /* Publish this device as the injection target. */
        KeAcquireSpinLock(&g_Global.Lock, &oldIrql);
        if (g_Global.Target == NULL) {
            g_Global.Target = devCtx;
        }
        g_Global.KeyboardCount++;
        KeReleaseSpinLock(&g_Global.Lock, oldIrql);

        KdPrint((DRIVER_NAME ": keyboard connected\n"));
        break;

    case IOCTL_INTERNAL_KEYBOARD_DISCONNECT:
        /*
         * Clear the connection and withdraw the injection target.  Racing with
         * in-flight input is safe: ServiceCallback() checks for a NULL callback
         * and swallows the packet instead of dereferencing a stale pointer.
         */
        KeAcquireSpinLock(&g_Global.Lock, &oldIrql);
        if (g_Global.Target == devCtx) {
            g_Global.Target = NULL;
        }
        KeReleaseSpinLock(&g_Global.Lock, oldIrql);

        RtlZeroMemory(&devCtx->UpperConnectData, sizeof(devCtx->UpperConnectData));
        devCtx->Connected = FALSE;
        KdPrint((DRIVER_NAME ": keyboard disconnected\n"));
        break;

    default:
        break;
    }

    if (!NT_SUCCESS(status)) {
        WdfRequestComplete(Request, status);
        return;
    }

    WDF_REQUEST_SEND_OPTIONS_INIT(&options, WDF_REQUEST_SEND_OPTION_SEND_AND_FORGET);
    if (!WdfRequestSend(Request, WdfDeviceGetIoTarget(device), &options)) {
        status = WdfRequestGetStatus(Request);
        KdPrint((DRIVER_NAME ": forwarding IOCTL 0x%08X failed 0x%08X\n",
                 IoControlCode, status));
        WdfRequestComplete(Request, status);
    }
}

/*
 * ServiceCallback - the interception point for real keyboard input
 *
 * The port driver calls this for every batch of hardware keystrokes.  We simply
 * pass them up to kbdclass: this driver injects input, it does not alter what
 * the user types.
 *
 * Runs at DISPATCH_LEVEL (called from the port driver's ISR DPC), which is why
 * the connection is only read here, never locked.
 */
VOID
ServiceCallback(
    _In_ PDEVICE_OBJECT       DeviceObject,
    _In_ PKEYBOARD_INPUT_DATA InputDataStart,
    _In_ PKEYBOARD_INPUT_DATA InputDataEnd,
    _Inout_ PULONG            InputDataConsumed
)
{
    PDEVICE_EXTENSION         devCtx;
    PSERVICE_CALLBACK_ROUTINE classService;

    devCtx = DeviceGetContext(WdfWdmDeviceGetWdfDeviceHandle(DeviceObject));

    classService = (PSERVICE_CALLBACK_ROUTINE)devCtx->UpperConnectData.ClassService;
    if (classService == NULL) {
        /* Not connected yet: swallow the input rather than crash. */
        *InputDataConsumed = (ULONG)(InputDataEnd - InputDataStart);
        return;
    }

    classService(
        devCtx->UpperConnectData.ClassDeviceObject,
        InputDataStart,
        InputDataEnd,
        InputDataConsumed
    );
}

/*
 * InjectKeyboardEvents - feed synthetic events to kbdclass
 *
 * Calls the class driver's service callback directly with the supplied events.
 * The callback is invoked at DISPATCH_LEVEL, matching the IRQL at which a real
 * port driver's DPC calls it, because the class driver acquires a spinlock
 * internally.
 */
NTSTATUS
InjectKeyboardEvents(
    _In_reads_(EventCount) PKEYBOARD_INPUT_DATA Events,
    _In_ ULONG EventCount
)
{
    PDEVICE_EXTENSION         devCtx;
    PSERVICE_CALLBACK_ROUTINE classService;
    PDEVICE_OBJECT            classDevice;
    KIRQL                     oldIrql;
    ULONG                     consumed = 0;

    if (EventCount == 0) {
        return STATUS_SUCCESS;
    }

    KeAcquireSpinLock(&g_Global.Lock, &oldIrql);
    devCtx = g_Global.Target;
    if (devCtx == NULL || !devCtx->Connected ||
        devCtx->UpperConnectData.ClassService == NULL) {
        KeReleaseSpinLock(&g_Global.Lock, oldIrql);
        KdPrint((DRIVER_NAME ": no keyboard connected - cannot inject\n"));
        return STATUS_DEVICE_NOT_CONNECTED;
    }
    classService = (PSERVICE_CALLBACK_ROUTINE)devCtx->UpperConnectData.ClassService;
    classDevice  = devCtx->UpperConnectData.ClassDeviceObject;
    KeReleaseSpinLock(&g_Global.Lock, oldIrql);

    /*
     * Raise to DISPATCH_LEVEL: the class driver's callback takes a spinlock
     * with KeAcquireSpinLockAtDpcLevel and asserts it is already at that IRQL.
     */
    oldIrql = KeRaiseIrqlToDpcLevel();
    classService(classDevice, Events, Events + EventCount, &consumed);
    KeLowerIrql(oldIrql);

    KdPrint((DRIVER_NAME ": injected %lu event(s), consumed %lu\n",
             EventCount, consumed));
    return STATUS_SUCCESS;
}
