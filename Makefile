VERSION := $(strip $(file < VERSION.txt))
VERSION_PACKAGE := github.com/androsh7/cloud_command/internal/version
RELEASE_LDFLAGS := -s -w -X $(VERSION_PACKAGE).Version=v$(VERSION)

.PHONY: build build-dev run vet test test-live-ssh test-live-aws fmt sshd-up sshd-down

# production build: title shows "Cloud Command v<VERSION.txt>"
build:
	go build -ldflags="$(RELEASE_LDFLAGS)" ./cmd/cloud_command

# development build: title shows "Cloud Command DEVELOPMENT"
build-dev:
	go build ./cmd/cloud_command

run:
	go run ./cmd/cloud_command

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
