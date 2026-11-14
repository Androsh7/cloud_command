VERSION := $(strip $(file < VERSION.txt))
VERSION_PACKAGE := github.com/androsh7/cloud_command/internal/version
RELEASE_LDFLAGS := -s -w -X $(VERSION_PACKAGE).Version=v$(VERSION)

APP    := cloud-command
PKG    := ./cmd/cloud_command
DIST   := dist
SYSO   := cmd/cloud_command/rsrc_windows_amd64.syso

ifeq ($(OS),Windows_NT)
    FIND    := where.exe
    DEVNULL := NUL
    EXE     := .exe
else
    FIND    := command -v
    DEVNULL := /dev/null
    EXE     :=
endif

require-%:
	@$(FIND) $* >$(DEVNULL) 2>&1 || (echo error: $* not found on PATH && exit 1)

.PHONY: build build-dev run vet test test-live-ssh test-live-aws fmt sshd-up sshd-down \
        windows windows-installer linux deb rpm apk archlinux tarball linux-packages

ifeq ($(OS),Windows_NT)
build: build-windows
else
build: build-linux
endif

build-dev:
	go build -o $(APP)$(EXE) $(PKG)

# go install github.com/tc-hib/go-winres@latest
build-windows-syso: require-go-winres
	go-winres make --in packaging/winres.json --out cmd/cloud_command/rsrc \
	--arch amd64 \
	--product-version $(VERSION) \
	--file-version $(VERSION)

build-windows: build-windows-syso
	go build -trimpath -ldflags="$(RELEASE_LDFLAGS)" -o $(DIST)/windows/$(APP).exe $(PKG)

# winget install JRSoftware.InnoSetup
build-windows-installer: require-iscc build-windows
	iscc packaging/installer.iss

build-linux:
	CGO_ENABLED=0 go build -trimpath -ldflags="$(RELEASE_LDFLAGS)" -o $(DIST)/linux/$(APP) $(PKG)

# go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest
build-linux-deb build-linux-rpm build-linux-apk build-linux-archlinux: build-linux-%: require-nfpm build-linux
	VERSION=$(VERSION) nfpm package --config packaging/nfpm.yaml --packager $* --target $(DIST)/linux/

build-linux-packages: build-linux-deb build-linux-rpm build-linux-apk build-linux-archlinux

run:
	go run $(PKG)

fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test ./...

test-live-ssh:
	go test -tags live ./pkg/sshx/ -run Live -v

test-live-aws:
	go test -tags live_aws ./pkg/agent/ -run Live -v -timeout 15m

sshd-up:
	docker compose up -d

sshd-down:
	docker compose down
