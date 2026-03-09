/* SPDX-License-Identifier: GPL-2.0 */
/*
 * keyboardctl.h - Shared definitions for keyboardctl driver and userspace
 *
 * This header is used by both the kernel module and (via CGO) the Go userspace
 * library to ensure the protocol struct layout stays in sync.
 */
#ifndef _KEYBOARDCTL_H
#define _KEYBOARDCTL_H

#ifdef __KERNEL__
#include <linux/types.h>
#else
#include <stdint.h>
typedef uint16_t __u16;
typedef int32_t  __s32;
#endif

/**
 * struct keyboardctl_event - keyboard event written to /dev/keyboardctl
 * @type:  event type: KBCTL_EV_SYN (0) or KBCTL_EV_KEY (1)
 * @code:  key code (KEY_A, KEY_ENTER, etc. — Linux input event codes)
 * @value: key state: KBCTL_KEY_RELEASE (0), KBCTL_KEY_PRESS (1),
 *         or KBCTL_KEY_REPEAT (2)
 *
 * Write one or more of these structs sequentially to /dev/keyboardctl.
 * Always terminate a key sequence with a SYN_REPORT event so that the
 * input subsystem delivers the events to listening applications.
 *
 * Example — press and release KEY_A:
 *   {KBCTL_EV_KEY, KEY_A,        KBCTL_KEY_PRESS}
 *   {KBCTL_EV_SYN, SYN_REPORT,  0}
 *   {KBCTL_EV_KEY, KEY_A,        KBCTL_KEY_RELEASE}
 *   {KBCTL_EV_SYN, SYN_REPORT,  0}
 */
struct keyboardctl_event {
	__u16 type;
	__u16 code;
	__s32 value;
};

/* Event types (mirror linux/input-event-codes.h) */
#define KBCTL_EV_SYN  0x00
#define KBCTL_EV_KEY  0x01

/* SYN sub-codes */
#define KBCTL_SYN_REPORT  0

/* Key state values */
#define KBCTL_KEY_RELEASE  0
#define KBCTL_KEY_PRESS    1
#define KBCTL_KEY_REPEAT   2

#endif /* _KEYBOARDCTL_H */
