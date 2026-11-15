/*
 * queue.c - control device IOCTL handling
 *
 * Creates the I/O queue for the \\.\KbdFilter control device and processes
 * IOCTL_KEYBOARD_INJECT requests sent by the Go userspace controller.
 *
 * Protocol (METHOD_BUFFERED):
 *   Input buffer:  one or more KEYBOARD_INPUT_DATA structures
 *   Output buffer: none
 *
 * Each KEYBOARD_INPUT_DATA entry:
 *   USHORT UnitId           - always 0
 *   USHORT MakeCode         - PC/AT scan code (Set 1)
 *   USHORT Flags            - KEY_MAKE=0, KEY_BREAK=1, KEY_E0=2, KEY_E1=4
 *   USHORT Reserved         - must be 0
 *   ULONG  ExtraInformation - must be 0
 */

#include "driver.h"

/*
 * ControlQueueInitialize - create the control device's default I/O queue
 */
NTSTATUS
ControlQueueInitialize(
    _In_ WDFDEVICE Device
)
{
    WDF_IO_QUEUE_CONFIG queueConfig;
    NTSTATUS            status;

    WDF_IO_QUEUE_CONFIG_INIT_DEFAULT_QUEUE(&queueConfig, WdfIoQueueDispatchSequential);
    queueConfig.EvtIoDeviceControl = EvtIoDeviceControl;

    status = WdfIoQueueCreate(
        Device,
        &queueConfig,
        WDF_NO_OBJECT_ATTRIBUTES,
        WDF_NO_HANDLE
    );
    if (!NT_SUCCESS(status)) {
        KdPrint((DRIVER_NAME ": WdfIoQueueCreate failed 0x%08X\n", status));
    }
    return status;
}

/*
 * EvtIoDeviceControl - handle IOCTL requests from user mode
 */
VOID
EvtIoDeviceControl(
    _In_ WDFQUEUE Queue,
    _In_ WDFREQUEST Request,
    _In_ size_t OutputBufferLength,
    _In_ size_t InputBufferLength,
    _In_ ULONG IoControlCode
)
{
    NTSTATUS             status;
    PKEYBOARD_INPUT_DATA events;
    PKEYBOARDCTL_STATE   state;
    size_t               bufferSize;
    ULONG                eventCount;
    KIRQL                oldIrql;

    UNREFERENCED_PARAMETER(Queue);

    switch (IoControlCode) {

    case IOCTL_KEYBOARD_INJECT:

        if (OutputBufferLength != 0) {
            KdPrint((DRIVER_NAME ": unexpected output buffer for inject\n"));
            status = STATUS_INVALID_PARAMETER;
            break;
        }

        /* Must receive at least one complete KEYBOARD_INPUT_DATA struct */
        if (InputBufferLength < sizeof(KEYBOARD_INPUT_DATA)) {
            KdPrint((DRIVER_NAME ": buffer too small: %Iu\n", InputBufferLength));
            status = STATUS_INVALID_BUFFER_SIZE;
            break;
        }

        if (InputBufferLength % sizeof(KEYBOARD_INPUT_DATA) != 0) {
            KdPrint((DRIVER_NAME ": unaligned buffer length %Iu\n", InputBufferLength));
            status = STATUS_INVALID_BUFFER_SIZE;
            break;
        }

        eventCount = (ULONG)(InputBufferLength / sizeof(KEYBOARD_INPUT_DATA));
        if (eventCount > KEYBOARDSIM_MAX_EVENTS) {
            KdPrint((DRIVER_NAME ": too many events %lu (max %d)\n",
                     eventCount, KEYBOARDSIM_MAX_EVENTS));
            status = STATUS_INVALID_PARAMETER;
            break;
        }

        status = WdfRequestRetrieveInputBuffer(
            Request,
            InputBufferLength,
            (PVOID *)&events,
            &bufferSize
        );
        if (!NT_SUCCESS(status)) {
            KdPrint((DRIVER_NAME ": WdfRequestRetrieveInputBuffer failed 0x%08X\n", status));
            break;
        }

        status = InjectKeyboardEvents(events, eventCount);
        break;

    case IOCTL_KEYBOARD_QUERY_STATE:

        if (OutputBufferLength < sizeof(KEYBOARDCTL_STATE)) {
            status = STATUS_INVALID_BUFFER_SIZE;
            break;
        }

        status = WdfRequestRetrieveOutputBuffer(
            Request,
            sizeof(KEYBOARDCTL_STATE),
            (PVOID *)&state,
            &bufferSize
        );
        if (!NT_SUCCESS(status)) {
            KdPrint((DRIVER_NAME ": WdfRequestRetrieveOutputBuffer failed 0x%08X\n", status));
            break;
        }

        KeAcquireSpinLock(&g_Global.Lock, &oldIrql);
        state->Connected     = (g_Global.Target != NULL) ? 1 : 0;
        state->KeyboardCount = g_Global.KeyboardCount;
        KeReleaseSpinLock(&g_Global.Lock, oldIrql);

        WdfRequestCompleteWithInformation(Request, STATUS_SUCCESS, sizeof(KEYBOARDCTL_STATE));
        return;

    default:
        KdPrint((DRIVER_NAME ": unknown IOCTL 0x%08X\n", IoControlCode));
        status = STATUS_INVALID_DEVICE_REQUEST;
        break;
    }

    WdfRequestComplete(Request, status);
}
