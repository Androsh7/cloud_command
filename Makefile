.PHONY: build run vet test test-live-ssh test-live-aws fmt sshd-up sshd-down

build:
	go build -ldflags="-s -w" ./cmd/cloud_command

run:
	go run ./cmd/cloud_command

fmt:
	gofmt -l -w .

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
