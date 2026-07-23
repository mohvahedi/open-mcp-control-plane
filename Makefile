.PHONY: test race fmt vet build run cli

test:
	GOTOOLCHAIN=local go test ./...

race:
	GOTOOLCHAIN=local go test -race ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

vet:
	GOTOOLCHAIN=local go vet ./...

build:
	GOTOOLCHAIN=local go build -o bin/controlplane ./cmd/controlplane
	GOTOOLCHAIN=local go build -o bin/mcpctl ./cmd/mcpctl

cli: build

run:
	GOTOOLCHAIN=local OPENMCP_USE_FAKE_RUNTIME=true go run ./cmd/controlplane
