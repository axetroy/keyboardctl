# keyboardctl — Go userspace + Windows WDF kernel driver keyboard simulator
#
# Platform: Windows 10/11 x64
#
# Targets:
#   all     Cross-compile the Go CLI for Windows (amd64)
#   go      Same as 'all'
#   test    Run platform-independent Go unit tests (works on any OS)
#   clean   Remove build artefacts
#
# The C kernel driver (driver/) must be built separately in a Windows
# environment with the WDK installed:
#   cd driver && build -cZ      (legacy WDK)
#   msbuild driver.vcxproj      (Visual Studio + WDK)

BINARY_WIN := keyboardctl.exe

GO       ?= go
GOFLAGS  ?=

.PHONY: all go test clean

all: go

## ── Go (cross-compile for Windows amd64) ───────────────────────────────────

go:
	GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) \
	    -o $(BINARY_WIN) ./cmd/keyboardctl

## ── Tests (platform-independent scan code tests, runs on any OS) ───────────

test:
	$(GO) test $(GOFLAGS) ./cmd/keyboardctl/

## ── Clean ───────────────────────────────────────────────────────────────────

clean:
	rm -f $(BINARY_WIN)
