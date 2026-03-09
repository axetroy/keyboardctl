/*
 * queue.c - IOCTL request handling
 *
 * Creates a WDF I/O queue and processes IOCTL_KEYBOARD_INJECT requests sent
 * from the Go userspace controller.
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
 * QueueInitialize - create the default I/O queue for a device
 *
 * Uses the sequential dispatch type so that IOCTL requests are handled one
 * at a time, matching the mutex-like serialisation done in userspace.
 */
NTSTATUS
QueueInitialize(
    _In_ WDFDEVICE Device
)
{
    NTSTATUS              status;
    WDF_IO_QUEUE_CONFIG   queueConfig;
    WDFQUEUE              queue;

    WDF_IO_QUEUE_CONFIG_INIT_DEFAULT_QUEUE(
        &queueConfig,
        WdfIoQueueDispatchSequential
    );

    queueConfig.EvtIoDeviceControl = EvtIoDeviceControl;

    status = WdfIoQueueCreate(
        Device,
        &queueConfig,
        WDF_NO_OBJECT_ATTRIBUTES,
        &queue
    );

    if (!NT_SUCCESS(status)) {
        KdPrint((DRIVER_NAME ": WdfIoQueueCreate failed 0x%08X\n", status));
    }

    return status;
}

/*
 * EvtIoDeviceControl - handle IOCTL requests
 *
 * Currently only IOCTL_KEYBOARD_INJECT is handled; all other codes are
 * completed with STATUS_INVALID_DEVICE_REQUEST.
 */
VOID
EvtIoDeviceControl(
    _In_ WDFQUEUE   Queue,
    _In_ WDFREQUEST Request,
    _In_ size_t     OutputBufferLength,
    _In_ size_t     InputBufferLength,
    _In_ ULONG      IoControlCode
)
{
    NTSTATUS          status;
    PDEVICE_CONTEXT   devCtx;
    PKEYBOARD_INPUT_DATA events;
    size_t            bufferSize;
    ULONG             eventCount;

    UNREFERENCED_PARAMETER(OutputBufferLength);

    devCtx = DeviceGetContext(WdfIoQueueGetDevice(Queue));

    switch (IoControlCode) {

    case IOCTL_KEYBOARD_INJECT:

        /* Must receive at least one complete KEYBOARD_INPUT_DATA struct */
        if (InputBufferLength < sizeof(KEYBOARD_INPUT_DATA)) {
            KdPrint((DRIVER_NAME ": buffer too small: %Iu\n",
                     InputBufferLength));
            status = STATUS_INVALID_BUFFER_SIZE;
            break;
        }

        if (InputBufferLength % sizeof(KEYBOARD_INPUT_DATA) != 0) {
            KdPrint((DRIVER_NAME ": unaligned buffer length %Iu\n",
                     InputBufferLength));
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
            KdPrint((DRIVER_NAME ": WdfRequestRetrieveInputBuffer failed "
                     "0x%08X\n", status));
            break;
        }

        status = InjectKeyboardEvents(devCtx, events, eventCount);
        break;

    default:
        KdPrint((DRIVER_NAME ": unknown IOCTL 0x%08X\n", IoControlCode));
        status = STATUS_INVALID_DEVICE_REQUEST;
        break;
    }

    WdfRequestComplete(Request, status);
}
