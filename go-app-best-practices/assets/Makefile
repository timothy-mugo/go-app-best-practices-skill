.PHONY: help build test lint fmt vet vuln run clean docker-build check

help:
	@echo "Targets: build test lint fmt vet vuln run clean docker-build check"

build:
	go build -v -o bin/api ./cmd/api

# -race catches data races that only appear under real scheduling
test:
	go test -v -race -coverprofile=coverage.out ./...

fmt:
	gofmt -s -w .

vet:
	go vet ./...

lint:
	golangci-lint run --timeout=5m

# Known-vulnerable dependencies and stdlib usage
vuln:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

run:
	go run ./cmd/api

clean:
	rm -rf bin/ coverage.out

docker-build:
	docker build -t api:latest .

# Everything CI should run, in order of cheapest first
check: fmt vet lint test vuln
