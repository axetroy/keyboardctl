// SPDX-License-Identifier: GPL-2.0
/*
 * keyboardctl.c - Virtual keyboard input device
 *
 * Architecture:
 *   This driver implements a hardware-independent keyboard simulator using a
 *   two-part design:
 *
 *   1. A virtual input device registered with the Linux input subsystem so that
 *      all applications see it as a real keyboard — fully transparent to the
 *      application layer.
 *
 *   2. A character device (/dev/keyboardctl) that accepts keyboardctl_event
 *      structs written by Go (or any other) userspace process and forwards them
 *      into the input subsystem via input_event().
 *
 * Usage (from userspace):
 *   Open /dev/keyboardctl and write one or more `struct keyboardctl_event`
 *   values.  Always follow key-press/release events with an EV_SYN/SYN_REPORT
 *   event so the input core delivers the batch to listeners.
 */

#include <linux/module.h>
#include <linux/kernel.h>
#include <linux/init.h>
#include <linux/input.h>
#include <linux/fs.h>
#include <linux/cdev.h>
#include <linux/uaccess.h>
#include <linux/device.h>
#include <linux/mutex.h>
#include <linux/slab.h>

#include "keyboardctl.h"

#define DRIVER_NAME     "keyboardctl"
#define DRIVER_DESC     "Virtual keyboard input device"
#define DRIVER_VERSION  "1.0.0"

/* Maximum events accepted in a single write() call */
#define KEYBOARDCTL_MAX_EVENTS  64

static struct input_dev  *vkbd_dev;
static dev_t              dev_num;
static struct cdev        keyboardctl_cdev;
static struct class      *keyboardctl_class;
static struct device     *keyboardctl_device;

/* Serialise concurrent write() calls */
static DEFINE_MUTEX(keyboardctl_write_lock);

/* -------------------------------------------------------------------------
 * Character device file operations
 * ---------------------------------------------------------------------- */

static int keyboardctl_open(struct inode *inode, struct file *filp)
{
	return 0;
}

static int keyboardctl_release(struct inode *inode, struct file *filp)
{
	return 0;
}

static ssize_t keyboardctl_write(struct file *filp, const char __user *buf,
				  size_t count, loff_t *ppos)
{
	struct keyboardctl_event ev;
	const char __user *ptr = buf;
	size_t remaining = count;
	int ret = 0;

	if (count == 0)
		return 0;

	/* Require whole event structs */
	if (count % sizeof(struct keyboardctl_event) != 0)
		return -EINVAL;

	/* Limit burst size to prevent userspace from holding the lock too long */
	if (count > KEYBOARDCTL_MAX_EVENTS * sizeof(struct keyboardctl_event))
		return -EINVAL;

	if (mutex_lock_interruptible(&keyboardctl_write_lock))
		return -ERESTARTSYS;

	while (remaining >= sizeof(struct keyboardctl_event)) {
		if (copy_from_user(&ev, ptr, sizeof(ev))) {
			ret = -EFAULT;
			goto out;
		}

		/* Only forward EV_KEY and EV_SYN events */
		if (ev.type != EV_KEY && ev.type != EV_SYN) {
			ret = -EINVAL;
			goto out;
		}

		input_event(vkbd_dev, ev.type, ev.code, ev.value);

		ptr       += sizeof(ev);
		remaining -= sizeof(ev);
	}

	ret = (ssize_t)count;
out:
	mutex_unlock(&keyboardctl_write_lock);
	return ret;
}

static const struct file_operations keyboardctl_fops = {
	.owner   = THIS_MODULE,
	.open    = keyboardctl_open,
	.release = keyboardctl_release,
	.write   = keyboardctl_write,
};

/* -------------------------------------------------------------------------
 * Module init / exit
 * ---------------------------------------------------------------------- */

static int __init keyboardctl_init(void)
{
	int err;
	int i;

	/* ---- Virtual input device ---- */
	vkbd_dev = input_allocate_device();
	if (!vkbd_dev) {
		pr_err("%s: failed to allocate input device\n", DRIVER_NAME);
		return -ENOMEM;
	}

	vkbd_dev->name       = "keyboardctl Virtual Keyboard";
	vkbd_dev->phys       = "keyboardctl/input0";
	vkbd_dev->id.bustype = BUS_VIRTUAL;
	vkbd_dev->id.vendor  = 0x0001;
	vkbd_dev->id.product = 0x0001;
	vkbd_dev->id.version = 0x0100;

	set_bit(EV_KEY, vkbd_dev->evbit);
	set_bit(EV_SYN, vkbd_dev->evbit);
	set_bit(EV_REP, vkbd_dev->evbit);

	/* Advertise all standard key codes */
	for (i = 0; i < KEY_MAX; i++)
		set_bit(i, vkbd_dev->keybit);

	err = input_register_device(vkbd_dev);
	if (err) {
		pr_err("%s: failed to register input device: %d\n",
		       DRIVER_NAME, err);
		input_free_device(vkbd_dev);
		return err;
	}

	/* ---- Character device ---- */
	err = alloc_chrdev_region(&dev_num, 0, 1, DRIVER_NAME);
	if (err) {
		pr_err("%s: failed to allocate chrdev region: %d\n",
		       DRIVER_NAME, err);
		goto err_unregister_input;
	}

	cdev_init(&keyboardctl_cdev, &keyboardctl_fops);
	keyboardctl_cdev.owner = THIS_MODULE;

	err = cdev_add(&keyboardctl_cdev, dev_num, 1);
	if (err) {
		pr_err("%s: failed to add cdev: %d\n", DRIVER_NAME, err);
		goto err_unregister_chr;
	}

	keyboardctl_class = class_create(DRIVER_NAME);
	if (IS_ERR(keyboardctl_class)) {
		err = PTR_ERR(keyboardctl_class);
		pr_err("%s: failed to create device class: %d\n",
		       DRIVER_NAME, err);
		goto err_del_cdev;
	}

	keyboardctl_device = device_create(keyboardctl_class, NULL,
					   dev_num, NULL, DRIVER_NAME);
	if (IS_ERR(keyboardctl_device)) {
		err = PTR_ERR(keyboardctl_device);
		pr_err("%s: failed to create device node: %d\n",
		       DRIVER_NAME, err);
		goto err_destroy_class;
	}

	pr_info("%s v%s loaded: /dev/%s (major=%d)\n",
		DRIVER_NAME, DRIVER_VERSION, DRIVER_NAME, MAJOR(dev_num));
	return 0;

err_destroy_class:
	class_destroy(keyboardctl_class);
err_del_cdev:
	cdev_del(&keyboardctl_cdev);
err_unregister_chr:
	unregister_chrdev_region(dev_num, 1);
err_unregister_input:
	input_unregister_device(vkbd_dev);
	return err;
}

static void __exit keyboardctl_exit(void)
{
	device_destroy(keyboardctl_class, dev_num);
	class_destroy(keyboardctl_class);
	cdev_del(&keyboardctl_cdev);
	unregister_chrdev_region(dev_num, 1);
	input_unregister_device(vkbd_dev);
	pr_info("%s: unloaded\n", DRIVER_NAME);
}

module_init(keyboardctl_init);
module_exit(keyboardctl_exit);

MODULE_LICENSE("GPL");
MODULE_AUTHOR("axetroy");
MODULE_DESCRIPTION(DRIVER_DESC);
MODULE_VERSION(DRIVER_VERSION);
