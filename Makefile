# keyboardctl — Windows keyboard input simulator (Go CLI + KMDF kernel driver)
#
# Platform: Windows 10/11 x64 only.
#
# Targets:
#   all       Build keyboardctl.exe (default)
#   build     Build keyboardctl.exe -> build/keyboardctl.exe
#   test      Run unit tests
#   clean     Remove build artefacts
#   install   Install the kernel driver (requires Administrator)
#   uninstall Remove the kernel driver (requires Administrator)
#   help      Show this help
#
# The C kernel driver (driver/) is built separately with MSBuild + WDK:
#   msbuild driver\driver.vcxproj /p:Configuration=Release /p:Platform=x64
# (Visual Studio 2022 + WDK for Windows 11; the legacy `build` tool no
#  longer ships with current WDKs.)
#
# Requires GNU make on Windows.  Commands that are cmd.exe built-ins are
# wrapped in "cmd /c" so the file works whether make uses cmd.exe or sh.

GO      ?= go
GOFLAGS ?= -trimpath
LDFLAGS ?= -s -w

BINARY := build/keyboardctl.exe

.PHONY: all build test clean install uninstall help

all: build

## ── Build ───────────────────────────────────────────────────────────────────

build:
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/keyboardctl

## ── Tests ───────────────────────────────────────────────────────────────────

test:
	$(GO) test $(GOFLAGS) ./cmd/keyboardctl/

## ── Clean ───────────────────────────────────────────────────────────────────

clean:
	@cmd /c "if exist build rmdir /s /q build"
	@cmd /c "if exist keyboardctl.exe del /f /q keyboardctl.exe"

## ── Driver management (requires Administrator) ──────────────────────────────

install:
	@cmd /c install.bat

uninstall:
	@cmd /c uninstall.bat

## ── Help ────────────────────────────────────────────────────────────────────

help:
	@cmd /c "echo keyboardctl Makefile targets:"
	@cmd /c "echo   all        - Build keyboardctl.exe [default]"
	@cmd /c "echo   build      - Build keyboardctl.exe"
	@cmd /c "echo   test       - Run unit tests"
	@cmd /c "echo   clean      - Remove build artefacts"
	@cmd /c "echo   install    - Install kernel driver [requires Admin]"
	@cmd /c "echo   uninstall  - Uninstall kernel driver [requires Admin]"
	@cmd /c "echo   help       - Show this help"
