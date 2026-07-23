.PHONY: test run build fmt vet

test:
	go test ./...

run:
	go run ./cmd/controlplane

build:
	go build ./cmd/controlplane

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

vet:
	go vet ./...
