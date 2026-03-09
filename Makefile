# keyboardctl — Go userspace + C kernel driver keyboard simulator
#
# Targets:
#   all        Build both the Go binary and the kernel module
#   go         Build the Go CLI binary
#   driver     Build the kernel module
#   test       Run Go unit tests
#   install    Install the kernel module and the CLI binary
#   load       Load the kernel module (requires root)
#   unload     Unload the kernel module (requires root)
#   clean      Remove build artefacts

BINARY     := keyboardctl
INSTALL_BIN := /usr/local/bin
DRIVER_DIR := driver

GO       ?= go
GOFLAGS  ?=

.PHONY: all go driver test install load unload clean

all: go driver

## ── Go ─────────────────────────────────────────────────────────────────────

go:
	$(GO) build $(GOFLAGS) -o $(BINARY) ./cmd/keyboardctl

test:
	$(GO) test $(GOFLAGS) ./...

## ── Kernel module ──────────────────────────────────────────────────────────

driver:
	$(MAKE) -C $(DRIVER_DIR)

## ── Installation ───────────────────────────────────────────────────────────

install: go driver
	install -m 755 $(BINARY) $(INSTALL_BIN)/$(BINARY)
	$(MAKE) -C $(DRIVER_DIR) install

## ── Module lifecycle (require root / sudo) ─────────────────────────────────

load: driver
	insmod $(DRIVER_DIR)/keyboardctl.ko

unload:
	rmmod keyboardctl

## ── Clean ───────────────────────────────────────────────────────────────────

clean:
	rm -f $(BINARY)
	$(MAKE) -C $(DRIVER_DIR) clean
