# keyboardctl — Go userspace + Windows WDF kernel driver keyboard simulator
#
# Platform: Windows 10/11 x64 (primary), cross-compile from Linux/macOS
#
# Targets:
#   all           Build for current platform (default: windows/amd64)
#   build         Cross-compile for Windows (amd64) - only platform supported
#   build-windows Cross-compile for Windows (amd64)
#   go            Same as 'build-windows' (legacy alias)
#   test          Run platform-independent Go unit tests (works on any OS)
#   clean         Remove build artefacts
#   install       Install driver (Windows only, requires Admin)
#   uninstall     Uninstall driver (Windows only, requires Admin)
#
# The C kernel driver (driver/) must be built separately in a Windows
# environment with the WDK installed:
#   cd driver && build -cZ      (legacy WDK)
#   msbuild driver.vcxproj      (Visual Studio + WDK)
#
# Note: This tool is Windows-only (uses Windows kernel driver).
# Cross-compilation to Linux/macOS is not supported as the main package
# has a //go:build windows constraint.

# Detect OS
UNAME_S := $(shell uname -s 2>/dev/null || echo Windows_NT)
ifeq ($(UNAME_S),Windows_NT)
    IS_WINDOWS := 1
    RM := del /f /q
    RMDIR := rmdir /s /q
    PATH_SEP := \\
    EXE := .exe
    # On Windows with cmd.exe, use set for env vars
    ENV_SET := set
    ENV_SEP := &
    MKDIR := mkdir
else
    IS_WINDOWS :=
    RM := rm -f
    RMDIR := rm -rf
    PATH_SEP := /
    EXE :=
    # On Unix, use export for env vars
    ENV_SET := export
    ENV_SEP := ;
    MKDIR := mkdir -p
endif

# Build configuration
BINARY_NAME := keyboardctl
BINARY_WIN := $(BINARY_NAME).exe

GO       ?= go
GOFLAGS  ?= -trimpath
LDFLAGS  ?= -s -w

# Output directory
BUILD_DIR := build

.PHONY: all build build-windows go test clean install uninstall help

# Default target
all: build-windows

## ── Help ────────────────────────────────────────────────────────────────────

help:
	@echo "keyboardctl Makefile targets:"
	@echo "  all            - Build for current platform (default: windows/amd64)"
	@echo "  build          - Cross-compile for Windows (amd64)"
	@echo "  build-windows  - Cross-compile for Windows (amd64)"
	@echo "  go             - Legacy alias for build-windows"
	@echo "  test           - Run platform-independent Go unit tests"
	@echo "  clean          - Remove build artefacts"
	@echo "  install        - Install kernel driver (Windows only, requires Admin)"
	@echo "  uninstall      - Uninstall kernel driver (Windows only, requires Admin)"
	@echo "  help           - Show this help"

## ── Cross-compilation targets ──────────────────────────────────────────────

# Build for Windows (only supported platform)
build: build-windows

# Windows (amd64)
build-windows: $(BUILD_DIR)$(PATH_SEP)windows-amd64$(PATH_SEP)$(BINARY_WIN)

$(BUILD_DIR)$(PATH_SEP)windows-amd64$(PATH_SEP)$(BINARY_WIN):
	@$(MKDIR) $(BUILD_DIR)$(PATH_SEP)windows-amd64
	$(ENV_SET) GOOS=windows$(ENV_SEP) $(ENV_SET) GOARCH=amd64$(ENV_SEP) $(ENV_SET) CGO_ENABLED=0$(ENV_SEP) $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" \
	    -o $@ ./cmd/keyboardctl

## ── Legacy alias ────────────────────────────────────────────────────────────

go: build-windows

## ── Tests (platform-independent scan code tests, runs on any OS) ───────────

test:
	$(GO) test $(GOFLAGS) ./cmd/keyboardctl/

## ── Clean ───────────────────────────────────────────────────────────────────

clean:
	$(RMDIR) $(BUILD_DIR)
	$(RM) $(BINARY_WIN) 2>nul || true

## ── Driver management (Windows only) ────────────────────────────────────────

install:
ifeq ($(IS_WINDOWS),1)
	@scripts\install.bat
else
	@echo "Error: Driver installation only supported on Windows"
	@exit 1
endif

uninstall:
ifeq ($(IS_WINDOWS),1)
	@scripts\uninstall.bat
else
	@echo "Error: Driver uninstallation only supported on Windows"
	@exit 1
endif
