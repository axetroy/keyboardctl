# keyboardctl — Windows keyboard input simulator (Go CLI + KMDF kernel driver)
#
# Platform: Windows 10/11 x64 only.
#
# Targets:
#   all       Build keyboardctl.exe (default)
#   build     Build keyboardctl.exe -> build/keyboardctl.exe
#   driver    Build the kernel driver -> driver/x64/Release/keyboardsimulator.sys
#   test      Run unit tests
#   clean     Remove build artefacts
#   install   Install the kernel driver (requires Administrator)
#   uninstall Remove the kernel driver (requires Administrator)
#   help      Show this help
#
# Most users do NOT need to build from source: download the prebuilt zip from
# the Releases page instead.  See README.md.
#
# The kernel driver needs Visual Studio 2022 + the WDK for Windows 11.
# The legacy `build` tool no longer ships with current WDKs, so MSBuild is the
# only supported driver build path.
#
# Requires GNU make on Windows.  Commands that are cmd.exe built-ins are
# wrapped in "cmd /c" so the file works whether make uses cmd.exe or sh.

GO      ?= go
GOFLAGS ?= -trimpath
VERSION ?= 1.0.0
LDFLAGS ?= -s -w -X main.version=$(VERSION)

BINARY  := build/keyboardctl.exe
DRIVER  := driver/x64/Release/keyboardsimulator.sys

.PHONY: all build driver test clean install uninstall help

all: build

## ── Build ───────────────────────────────────────────────────────────────────

build:
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/keyboardctl

driver:
	@cmd /c "if not exist driver\x64\Release mkdir driver\x64\Release"
	msbuild driver\driver.vcxproj /p:Configuration=Release /p:Platform=x64 /p:DriverVersion=$(VERSION) /m /nologo

## ── Tests ───────────────────────────────────────────────────────────────────

test:
	$(GO) test $(GOFLAGS) ./cmd/keyboardctl/

## ── Clean ───────────────────────────────────────────────────────────────────

clean:
	@cmd /c "if exist build rmdir /s /q build"
	@cmd /c "if exist keyboardctl.exe del /f /q keyboardctl.exe"
	@cmd /c "if exist driver\x64 rmdir /s /q driver\x64"
	@cmd /c "if exist driver\Debug rmdir /s /q driver\Debug"
	@cmd /c "if exist driver\Release rmdir /s /q driver\Release"

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
	@cmd /c "echo   driver     - Build keyboardsimulator.sys [needs VS2022 + WDK]"
	@cmd /c "echo   test       - Run unit tests"
	@cmd /c "echo   clean      - Remove build artefacts"
	@cmd /c "echo   install    - Install kernel driver [requires Admin]"
	@cmd /c "echo   uninstall  - Uninstall kernel driver [requires Admin]"
	@cmd /c "echo   help       - Show this help"
